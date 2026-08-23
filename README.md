# Mausul

Mauul is a Webmentions implementation in Go,
compliant with the [W3C Webmentions Spec](https://www.w3.org/TR/webmention/).

## Why

Because I wanted to try implementing it myself.
Also I needed an excuse to learn Go. In addition,
I have specific requirements:

- SQLite backend for simplicity
- Small binary and container image sizes
- Podman support
- A service that takes advantage of SystemD Socket
  Activation (not just start-up, but actually exit
  as well on idle)

## Installation

There are two options to run Mausul: Podman, or bare
metal. In both cases, SystemD is _required_, as Mausul
takes advantage of SystemD Socket Activation to only
start-up when needed, and exit when idle.

In either case, Mausul is intended to be ran as a user
service, not a system service.

Both Podman and bare metal setups use the same `mausul.socket`
and `mausul-revalidate.timer` files defined in `deploy/systemd`.
Regardless of which route is taken, those file are needed.

### Building Podman Image

To build the Podman image:

> [!WARNING]
> Docker _is not supported_. Mausul uses SystemD Socket
> Activation, which is _not supported_ in Docker. Podman
> is _required_.

```sh
podman build -t mausul:latest .
```

Then copy the files in `deply/quadlet` and `deploy/systemd`
to the correct locations.

### Building Bare Metal

To build just the binary:

```sh
go build -o mausul ./src
```

Then copy the files in `deply/baremetal` and `deploy/systemd`
to the correct locations.

### Configuration

With the default SystemD/Quadlet configuration, an environment
file located at `/home/mausul_user/mausul.env` is required, and
contain the following environment variables:

```
MAUSUL_ALLOWED_TARGETS=example.com,www.example.com
MAUSUL_USER_AGENT=Mausul Webmention-Receiver Bot/1.0
MAUSUL_MAX_FETCH_SIZE_BYTES=1048576
MAUSUL_MAX_TIMEOUT=10s
MAUSUL_DB_PATH=/webmentions/webmentions.db
```

### Some Notes on SystemD Configuration

The default SystemD configurations in this repository have
a couple of parameters that may be tweaked. In both the
Quadlet and bare metal setups:

- Memory usage is limited to 128M
    - 96M is considered, "high"
- Tasks/PIDs are limited to 32
- CPU Quota is set to 100%
    - It's _highly_ unlikely to ever be an issue, but
      this may be configured just in case

This may be configured to your liking.

## License

```
Copyright (C) 2026 Maryam Stellamaris <maryam@standingpad.org>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
```
