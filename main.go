// Copyright (C) 2026 Aleksei Ilin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/thediveo/gons/reexec"
	"github.com/urfave/cli/v2"
	"github.com/vishvananda/netlink"
	"go.yaml.in/yaml/v3"
	"golang.org/x/sys/unix"
)

const (
	ActionPrintNsLinks = "PrintNsLinks"
)

var (
	// Application version is set from Makefile via LD_FLAGS.
	AppVersion string

	dryRun bool

	config Config
)

type Config struct {
	// Container link prefixes to be removed, e.g. "eth".
	ContainerLinkPrefixes []string `yaml:"container_link_prefixes"`
	// Remove duplicated symbols in the resulted name.
	RemoveDuplicatedSymbols bool `yaml:"remove_duplicated_symbols"`
	// Symbol replacements. The replacement order is according to the position in the list.
	// Each replacement is processed non-recursively: when a substring of the container name matches a list item,
	// the substitution will not be matched against other replacements.
	Replacements []map[string]string `yaml:"replacements"`
	// Separator to be added in front of the link index.
	LinkIndexSeparator string `yaml:"link_index_separator"`
}

type VEth struct {
	// Name of the link within the container.
	Name string
	// Index of the peer link at the host.
	ParentIndex int
}

func init() {
	// Register function for the execution within the container namespace.
	reexec.Register(ActionPrintNsLinks, printNsLinks)
	// Check whether should switch to the container namespace.
	reexec.CheckAction()
}

// Returns any key/value from map.
func mapKeyVal[K comparable, V any](m map[K]V) (k K, v V) {
	for k, v := range m {
		return k, v
	}
	return k, v
}

// Print a string of JSON-encoded array of veth links to stdout: []VEth
// This function is executed from within of the container network namespace.
// On error no output to stdout is provided.
func printNsLinks() {
	links, err := netlink.LinkList()
	if err != nil {
		log.Errorf("netlink.LinkList failed: %s", err)
		return
	}

	var vethLinks []VEth
	for _, link := range links {
		if link.Type() != "veth" {
			continue
		}

		attrs := link.Attrs()

		vethLinks = append(vethLinks, VEth{
			Name:        attrs.Name,
			ParentIndex: attrs.ParentIndex,
		})
	}

	vethJson, err := json.Marshal(vethLinks)
	if err != nil {
		log.Errorf("json.Marshal to bytes failed: %s", err)
		return
	}

	fmt.Println(string(vethJson))
}

// Replaces substrings in the container name.
func applyReplacements(containerName string) string {
	type Substring struct {
		text string
		// Whether the current substring was already matched.
		processed bool
	}

	substrings := make([]Substring, 0, len(containerName))
	substrings = append(substrings, Substring{text: containerName})

	for _, pair := range config.Replacements {
		if len(pair) == 0 {
			continue
		}

		needle, replacement := mapKeyVal(pair)
		if len(needle) == 0 {
			continue
		}

		var substringsUpdated []Substring
		for _, m := range substrings {
			if m.processed {
				substringsUpdated = append(substringsUpdated, m)
				continue
			}

			for {
				i := strings.Index(m.text, needle)
				if i == -1 {
					substringsUpdated = append(substringsUpdated, m)
					break
				}

				if i > 0 {
					// Add unprocessed prefix.
					substringsUpdated = append(substringsUpdated, Substring{text: m.text[:i]})
				}

				if len(replacement) > 0 {
					substringsUpdated = append(substringsUpdated, Substring{text: replacement, processed: true})
				}

				if i+len(needle) < len(m.text) {
					// Add unprocessed suffix. Process it on next iteration.
					m = Substring{text: m.text[i+len(needle):]}
				} else {
					// No suffix.
					break
				}
			}
		}

		substrings = substringsUpdated
	}

	// Assemble substrings into a string.
	var sb strings.Builder
	for _, m := range substrings {
		sb.WriteString(m.text)
	}
	return sb.String()
}

// Make the human-readable link name.
// Name format: v[NAME][SEP][NUM]
// Where [NAME] is a morphed container name, [SEP] is a separator, and [NUM] is the link number within the container.
// Linux has limitation to the link name set to 15 symbols, see IFNAMSIZ,
// therefore [NAME] is morphed container name according to the configuration file.
func makeLinkName(containerName string, containerLinkName string) string {
	if len(containerName) == 0 || len(containerLinkName) == 0 {
		return ""
	}

	// Remove everything before the last slash (including).
	slashIndex := strings.LastIndex(containerName, "/")
	if slashIndex != -1 {
		containerName = containerName[slashIndex+1:]
	}

	// Apply replacements.
	morphedName := applyReplacements(containerName)

	// Keep at least one symbol.
	if len(morphedName) == 0 {
		morphedName = string(containerName[0])
	}

	// Remove duplicated symbols.
	if config.RemoveDuplicatedSymbols {
		dedupName := make([]byte, 0, len(morphedName))
		for i := range morphedName {
			if i > 0 {
				if dedupName[len(dedupName)-1] == morphedName[i] {
					continue
				}
			}
			dedupName = append(dedupName, morphedName[i])
		}
		morphedName = string(dedupName)
	}

	// Remove link prefix.
	linkSuffix := containerLinkName
	for _, prefix := range config.ContainerLinkPrefixes {
		if strings.HasPrefix(containerLinkName, prefix) {
			linkSuffix = strings.TrimPrefix(linkSuffix, prefix)
			break
		}
	}

	// Cut the morphed name to fit IFNAMSIZ-1 (15 bytes).
	// -1 for '\0' and 'v'
	contNameMaxLen := unix.IFNAMSIZ - 1 - len(linkSuffix) - len(config.LinkIndexSeparator) - 1
	if contNameMaxLen < 1 {
		log.Errorf("Cannot make host link name: container link suffix is too long: %s %s", containerName, containerLinkName)
		return ""
	}
	if len(morphedName) > contNameMaxLen {
		morphedName = morphedName[:contNameMaxLen]
	}

	return fmt.Sprintf("v%s%s%s", morphedName, config.LinkIndexSeparator, linkSuffix)
}

// Renames the host link to match the container name and the container link index.
func updateLinkName(link netlink.Link, containerName string, containerLinkName string) {
	linkName := makeLinkName(containerName, containerLinkName)
	if len(linkName) == 0 {
		// Link name cannot be made.
		return
	}

	if link.Attrs().Name == linkName {
		log.Debugf("Link was renamed already: %s %s: %s", containerName, containerLinkName, link.Attrs().Name)
		return
	}

	if !dryRun {
		err := netlink.LinkSetName(link, linkName)
		if err != nil {
			log.Errorf("netlink.LinkSetName failed: %s %s: %s => %s : %s", containerName, containerLinkName, link.Attrs().Name, linkName, err)
			return
		}
	}

	log.Infof("Link renamed: %s %s: %s => %s", containerName, containerLinkName, link.Attrs().Name, linkName)
}

// Renames net links for the container of the inspect record.
func renameContainerLinks(inspect container.InspectResponse) {
	if len(inspect.Name) == 0 {
		log.Errorf("Cannot make host link name: container name must not be empty: %s", inspect.ID)
		return
	}

	// Check network mode.
	switch inspect.HostConfig.NetworkMode {
	case "host":
		log.Debugf("Container is running in host network mode, skipping: %s %s", inspect.Name, inspect.ID)
		return
	case "none":
		log.Debugf("Container is running in none network mode, skipping: %s %s", inspect.Name, inspect.ID)
		return
	}

	// Check sandbox.
	sandboxKey := inspect.NetworkSettings.NetworkSettingsBase.SandboxKey
	if len(sandboxKey) == 0 {
		log.Errorf("Sandbox is not defined for container: %s %s", inspect.Name, inspect.ID)
		return
	} else if strings.HasSuffix(sandboxKey, "/default") {
		log.Errorf("Container uses default namespace, this is not supported: %s %s", inspect.Name, inspect.ID)
		return
	}

	var containerLinks []VEth
	err := reexec.RunReexecAction(ActionPrintNsLinks, reexec.Result(&containerLinks), reexec.Namespaces([]reexec.Namespace{
		{
			Type: "net",
			Path: sandboxKey,
		},
	}))
	if err != nil {
		log.Errorf("reexec.RunReexecAction failed for container: %s %s: %s", inspect.Name, inspect.ID, err)
		return
	}

	for _, containerLink := range containerLinks {
		if len(containerLink.Name) == 0 {
			log.Errorf("Cannot make host link name: container link suffix must not be empty: %s %d", inspect.ID, containerLink.ParentIndex)
			continue
		}

		link, err := netlink.LinkByIndex(containerLink.ParentIndex)
		if err != nil {
			log.Errorf("netlink.LinkByIndex failed: %s", err)
			continue
		}

		updateLinkName(link, inspect.Name, containerLink.Name)
	}
}

// Iterates over running containers updating the corresponding host link names.
func processRunningContainers(ctx context.Context, cli *client.Client) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		log.Errorf("cli.ContainerList failed: %s", err)
		return
	}

	// Sort containers by name to have predictable results between multiple runs,
	// in case of rename failures.
	inspects := make([]container.InspectResponse, 0, len(containers))
	for _, container := range containers {
		inspect, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			log.Errorf("cli.ContainerInspect failed for container ID %s: %s", container.ID, err)
			continue
		}

		inspects = append(inspects, inspect)
	}

	slices.SortFunc(inspects, func(a, b container.InspectResponse) int {
		return cmp.Compare(a.Name, b.Name)
	})

	for _, inspect := range inspects {
		renameContainerLinks(inspect)
	}
}

// Iterates over running containers updating the corresponding host link names,
// and starts listening to Docker events in the endless loop.
func listenToDockerEvents(ctx context.Context, cli *client.Client) {
	filterArgs := filters.NewArgs(
		filters.KeyValuePair{
			Key:   "action",
			Value: string(events.ActionConnect),
		},
	)

	eventChan, errs := cli.Events(ctx, events.ListOptions{Filters: filterArgs})

	// Process currently running containers after events channel is created, to avoid race during system startup.
	processRunningContainers(ctx, cli)

	for {
		select {
		case err := <-errs:
			// Exit the application causing restart via systemd (for example, on Docker restart).
			log.Fatal(err)

		case event := <-eventChan:
			if event.Type == events.NetworkEventType && event.Action == events.ActionConnect {
				log.Debugf("Event: ID: %s, Attr: %v", event.Actor.ID, event.Actor.Attributes)

				if containerID, ok := event.Actor.Attributes["container"]; ok {
					inspect, err := cli.ContainerInspect(ctx, containerID)
					if err != nil {
						log.Errorf("cli.ContainerInspect failed for container ID %s: %s", containerID, err)
						continue
					}

					renameContainerLinks(inspect)

				} else {
					log.Errorf("Event has no container ID: %s", event.Actor.ID)
				}
			}
		}
	}
}

func main() {
	app := &cli.App{
		Usage: "Tool for automatic renaming of Docker-created veth links",

		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"vv"},
				Usage:   "Use verbose logging",
			},
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"n"},
				Usage:   "Display the expected link name changes, but do not make actual renaming",
			},
			&cli.PathFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Value:   "/etc/docker-veth-namer.yml",
				Usage:   "Specify path to the configuration file",
			},
		},

		Before: func(ctx *cli.Context) error {
			// Set log level
			logVerbose := ctx.Bool("verbose")

			if logVerbose {
				log.SetLevel(log.TraceLevel)
			} else {
				log.SetLevel(log.InfoLevel)
			}

			// Set dry run flag.
			dryRun = ctx.Bool("dry-run")

			// Set config.
			configFilePath := ctx.Path("config")
			if len(configFilePath) > 0 {
				configFile, err := os.Open(configFilePath)
				if err != nil {
					return err
				}
				defer configFile.Close()
				configDecoder := yaml.NewDecoder(configFile)
				configDecoder.KnownFields(true)
				if err := configDecoder.Decode(&config); err != nil {
					return err
				}
			}

			return nil
		},

		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print program version",
				Action: func(cCtx *cli.Context) error {
					println(AppVersion)

					return nil
				},
			},
			{
				Name:  "oneshot",
				Usage: "Update veth links for currently running containers, and exit immediately",
				Action: func(cCtx *cli.Context) error {
					cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
					if err != nil {
						log.Fatalf("Failed to connect to Docker API: %s", err)
					}
					defer cli.Close()

					log.Debug("Connected to Docker API")

					ctx := context.Background()
					processRunningContainers(ctx, cli)

					return nil
				},
			},
			{
				Name:  "listen",
				Usage: "Starts listening to Docker events",
				Action: func(cCtx *cli.Context) error {
					cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
					if err != nil {
						log.Fatalf("Failed to connect to Docker API: %s", err)
					}
					defer cli.Close()

					log.Debug("Connected to Docker API")

					ctx := context.Background()
					listenToDockerEvents(ctx, cli)

					return nil
				},
			},
		},

		DefaultCommand: "listen",
		Copyright:      "2026 Aleksei Ilin",
		Version:        AppVersion,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
