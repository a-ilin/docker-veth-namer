# docker-veth-namer

**docker-veth-namer** is a Linux tool for automatic renaming of Docker-created _veth_ network links to human-readable format.

When a Docker container connects to a bridge network on Linux, a pair of network links is created: one on the host, and one inside the container.
Links within the container are typically named like _eth0_, _eth1_, and so on.
Links on the host side are named like _vethXXXXXXX_, where XXXXXXX are hexadecimal digits.
Those hexadecimal digits do not correlate with container name or ID, which makes it hard to identify which container a link on the host side belongs to.

**docker-veth-namer** renames the _veth_ links on the host side created by Docker to allow easier identification of the container owning the peer link.

The program was tested with Docker service running as root user. Rootless Docker mode may work, but was not tested.

## Compilation

To compile the program yourself install the required dependencies:

* dpkg (for the DEB package)
* gcc
* git
* golang
* libc6-dev
* Linux kernel headers
* make
* realpath
* scdoc (for the manual page)
* sed

To build the program: `make docker-veth-namer`.

To build the man page: `make doc`.

To build DEB package: `make deb`.

To build everything: `make` or `make all`.

## Runtime requirements

* libc
* Linux kernel >= 3.0
* Docker >= 1.12

## Installation

For Debian-based Linux distributions (Debian, Ubuntu, etc) the recommended installation method is using the DEB package.
Download a package corresponding to the CPU architecture of your system from **Releases** page, or build the package yourself: `make deb`.
Install the package with `apt` (replace the file name with the actual downloaded file name): `sudo apt install ./downloaded_deb_file.deb`.

For non-Debian Linux distributions the installation can be done with `sudo make install` command.
Ensure that the program and (optionally) documentation were compiled with `make docker-veth-namer` and `make doc` prior to the installation.

Once the program is installed, a new `systemd` service is added (the service is disabled by default): `docker-veth-namer.service`.
Starting this service will trigger renaming of Docker-created network links.

## Usage

Refer to [the manual page](doc/docker-veth-namer.8.scd) for the configuration options and working modes of the program.

## Authors

**docker-veth-namer** is written by Aleksei Ilin.

## Copyright

Copyright © 2026 Aleksei Ilin

**docker-veth-namer** can be used, and/or redistributed, and/or modified under the terms specified within the included license.

A copy of the license is included within the source code: [LICENSE](LICENSE).
