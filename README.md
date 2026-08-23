# Mausul

Webmentions implementation in Go. Compliant
with the [W3C Webmentions Spec](https://www.w3.org/TR/webmention/).

This uses SQLite as the underlying database,
to keep things minimal.

## Why

Because I wanted to try implementing it myself.
Also I needed an excuse to learn Go.

## Transparency Surrounding HTTP Responses

While this project attempts to be W3C-compliant,
there is one area that I cannot scratch my head
around.

When discussing responses, [Section 3.2](https://www.w3.org/TR/webmention/#receiving-webmentions)
states:

> If the receiver processes the request asynchronously
> but does not return a status URL, the receiver `MUST`
> reply with an `HTTP 202 Accepted` response. The response
> body `MAY` contain content, in which case a human-readable
> response is recommended.

However, in [Section 3.2.3](https://www.w3.org/TR/webmention/#error-responses),
it states:

> If the Webmention was not successful because of something
> the *sender* did, it `MUST` return a `400 Bad Request` status
> code and `MAY` include a description of the error in the response
> body.
>
> Possible sender-related errors that can be returned synchronously
> before making a GET request to the source: 
>
> - Specified `target` URL not found.
> - Specified `target` URL does not accept Webmentions.
> - `source` URL was malformed or is not a supported URL scheme
    (e.g. a mailto: link)
>
> Possible sender-related errors that can occur after fetching the
> contents of the source URL:
>
> - `source` URL not found.
> - `source` URL does not contain a link to the `target` URL.

These two introduce a gray-area: if a receiver processes requests
_asynchronously_, while not returning a status URL, then it _must_
return a `HTTP 202 Accepted` response. However, if the same receiver
finds a Webmention not successful because of something the sender did,
then it _must_ return a `400 Bad Request` response. In addition, the
spec _specifically_ mentions not finding the target URL in the content
of the source URL. However, if the source URL is processed asynchronously
(including parsing of HTML to validate the presence of a link to the
target URL in the content of the source URL), an intersting question arises:
if the receiver finds the content of the source URL to _not_ contain a
link to the target URL, but the processing of the content of the source
URL is asynchronous, then does the receiver:

- Return `202 Accepted` to indicate the Webmention was received (regardless
  of whether validation is successful or not)?
- Return `400 Bad Request` to indicate the Webmention was not successful?
    - If so, does it not return `202 Accepted`?

"Synchronous" checks are a little easier to interpret: checks that aren't
performed in a background thread. For this implementation, that would
be:

- Validating `source` and `target` are valid URLs with the `http` or
  `https` scheme
- Validating `source` and `target` are distinct URLs
- Validating `target` accepts Webmentions

These checks are performed right after receiving the request, and
before creating a separate goroutine. However, by nature of creating
a goroutine for processing the other checks, we cannot return a `400 Bad
Request` for checks like:

- Validating `source` contains a link to `target`
- Validating IP for `source` is not a loopback or private IP
- Validating presence of `source`

Is that not compliant to the W3C Webmentions Spec? I legitimately
do not know; my best interpretation is:

- Any checks that can be performed synchronously _must_ return a `400 Bad Request`
  if unsuccessful
- Otherwise, send a `202 Accepted`, and move on with asynchronous checks.

I would say that's compliant, as there is legitimately no way for us to
return a `400 Bad Request` for asynchronous checks, but ultimately, who
knows?

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
