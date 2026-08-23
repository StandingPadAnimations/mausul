# Mausul

Webmentions implementation in Go. Compliant
with the [W3C Webmentions Spec](https://www.w3.org/TR/webmention/).

This uses SQLite as the underlying database,
to keep things minimal.

## Why

Because I wanted to try implementing it myself.
Also I needed an excuse to learn Go.

## Installation Notes

> [!WARNING]
> This project was designed around my personal website,
> so I cannot guarantee easy deployment. That said, here's
> some important details.

Easiest way is to build the container file directly
from source. Podman is the only officially supported
platform (as in, _**this will not work with Docker**_),
since it depends on SystemD Socket Activation.

When using containers, the service must be exposed using
SystemD Socket Activation, as that is preferred when
using rootless Podman for network performance reasons.
Note that the container will terminate after five minutes
of inactivity. This is normal, SystemD Socket Activation
will handle bringing it back when needed.

As for configuration, the following environment variables
can be set:

```
WEBMENTIONS_ALLOWED_TARGETS=example.com,www.example.com
WEBMENTIONS_USER_AGENT=Webmention-Receiver/1.0
WEBMENTIONS_MAX_FETCH_SIZE_BYTES=1048576
WEBMENTIONS_MAX_TIMEOUT=10s
WEBMENTIONS_DB_PATH=/webmentions/webmentions.db
```

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
