# Webmentions

Webmentions implementation in ~~Python~~ Go.

## Why

Because I wanted to try implementing it myself.
Also I needed an excuse to learn Go.

## Installation Notes

> [!WARNING]
> I do not recommend using this, as it was designed
> around my personal website. That said, here's
> some information for those that _really_ want to
> use this.

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

Also, the accepted target domains and routes are hardcoded.
I might expose these in a JSON/TOML configuration in the future,
but do be aware of that. Same with database paths, which
are intended to be used in a container. Again, maybe I'll
write configuration for this in the future, but for now, be
aware of that.

## License

```
Copyright (C) 2026 Maryam Stellamaris

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
