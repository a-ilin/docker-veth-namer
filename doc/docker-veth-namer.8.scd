docker-veth-namer(8)


# NAME

*docker-veth-namer* - Tool for automatic renaming of Docker-created _veth_ network links to human-readable format.


# SYNOPSIS

*docker-veth-namer* [*options*] [*command*]


# DESCRIPTION

*docker-veth-namer* is an administration tool for renaming of _veth_ network links created by Docker to human-readable format.

When a Docker container connects to a bridge network, a pair of network links is created: one on the host, and one inside the container.
Links within the container are typically named like _eth0_, _eth1_, and so on.
Links on the host side are named like _vethXXXXXXX_, where XXXXXXX are hexadecimal digits.
Those hexadecimal digits do not correlate with container name or ID, which makes it hard to identify which container a link on the host side belongs to.

*docker-veth-namer* renames the _veth_ links on the host side created by Docker to allow easier identification of the container owning the peer link.

The program supports 2 modes of operation:
. One-time execution: *docker-veth-namer* processes all running containers, and exits immediately.
. One-time execution followed by waiting for more Docker events. This is the default mode.

The links name is constructed using the morphed container name and the link name suffix obtained from within the container namespace.

See *NAME MORPHING* below for the details on how the program constructs the link names.

Upon installation it is recommended to execute the program in _dry run_ and _oneshot_ modes to preview the expected changes,
and add configuration changes to achieve the desired link names.

The program was tested with Docker service running as root user. Rootless mode may work, but was not tested.


# OPTIONS

*-h*, *--help*++
Show help message and all options.

*-vv*, *--verbose*++
Use verbose logging.

*-n*, *--dry-run*++
Display the expected link name changes, but do not execute actual renaming.

*-c*, *--config*++
Specify path to the configuration file.


# COMMANDS

A command specifies the action to be executed by the program. Only one command may be specified at a time.

Available commands:

*listen*++
Process all running containers, and wait for Docker events. This is the default behavior.

*oneshot*++
Process all running containers, and exit immediately.

*version*++
Print program version and exit.


# NAME MORPHING

Linux has a limitation on the name length of network interfaces specified by the constant _IFNAMSIZ_, which is typically resolves to 16 bytes.
With one byte reserved for null-terminator symbol, this limits the interface name to the maximum of 15 visible characters.

Docker sets container name limit is higher, typically limited to 128 characters.

With the Linux network interface name limit being significantly lower than the Docker container name,
on a host running multiple Docker containers a situation may appear when an attempt to name a network interface after the Docker container will result
in duplicated network interface names (since the symbols above 15 will be rejected by kernel), and, consequently, the operation will be denied by kernel,
because of duplicated network interface names are not allowed.

To overcome this problem several approaches of morphing the container name into the host-side network link name were introduced.

The following criteria was set for the approaches:
. Expected result: given a certain container name and container-side link name, the resulted host-side link name should be the same.
. Associative naming: it should be possible to visually recognize which container owns the peer link of a certain host link, without usage of tables or maps of any kind.
. Configuration: it should be possible to cancel or change a name transformation in a particular system without program recompilation.

The following approaches were introduced satisfying those criteria.


## Replacements

The replacements are configured as an array of one-element dictionaries within the configuration file under the key++
*replacements*.

Each replacement specifies the needle, being the key of the one-element dictionary, (what to look for) and the substitution, being the value.

For the container name each replacement is being applied iteratively in the order of appearance in the configuration. The replacements are non-recursive:
once a substitution for a substring of the container name is done, the substituted part is fixed. The rest of the container name may still be substituted by
the current or other replacements.

In case of the name after being processed thru all replacements evolves into an empty string, the first symbol from the original name is being used as the result.

For example, assuming the container name is _mariadb-exporter_, with the following replacements the resulted name will be++
_madbex_:
```
replacements:
  - mariadb: madb
  - exporter: ex
  - "-": ""
```

## Duplicated symbols removal

This transformation removes the duplicated neighbor symbols in the container name, when enabled in the configuration file under the key++
*remove_duplicated_symbols*.

For example, the container name _bdddaemon-de_ will become _bdaemon-de_ after this transformation.


## Link prefix removal

This transformation applies to the name of the container-side network link.
It iterates over the specified prefixes in the configuration file under the key++
*container_link_prefixes*, and removes the first prefix found that matches.

For example, with the container link name being _eth0_, and the specified prefix _eth_, the resulted link name will be _0_.


## Final name construction

The transformations above are applied in the following order:
. Replacements.
. Duplicated symbols removal.
. Link prefix removal.

It is possible to specify a fixed string, link index separator, in the configuration file under the key++
*link_index_separator*.
This separator will appear in front of the link index when configured.

After the final transformation the maximum allowed length _MaxLen_ for the container name part is specified as:++
_MaxLen = IFNAMSIZ - length(link index) - length(link index separator) - 2_.

The final host-side link name is constructed as a concatenation of the following elements:
. _v_ (a constant letter prefix to identify that this network link is a _veth_ peer).
. At most _MaxLen_ symbols of the transformed container name.
. Link index separator.
. Link index.

For example, with the replacements and link prefix removal being as specified above, the container name++
_mariadb-exporter_, and container-side link _eth0_, will result in the host-side link name be++
_vmadbex0_.


# AUTHORS

*docker-veth-namer* is written by Aleksei Ilin.


# COPYRIGHT

Copyright © 2026 Aleksei Ilin

*docker-veth-namer* can be used, and/or redistributed, and/or modified under the terms specified within the included license file.
