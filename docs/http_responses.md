# HTTP Responses

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
