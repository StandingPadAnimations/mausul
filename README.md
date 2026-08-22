# Webmentions

Webmentions implementation in Python.

## Why

Because I wanted to try implementing it myself.

## Installation

Easiest way is to build the container file directly
from source. Podman is the only officially supported
platform.

When using containers, the service must be exposed using
SystemD Socket Activation. This is because that is preferred
in rootless Podman for networking performance.

## License

```
Copyright (C) Maryam Stellamaris (Mahid Sheikh)

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
