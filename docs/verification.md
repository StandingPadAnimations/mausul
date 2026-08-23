# Verification Process

As per the W3C spec, Mausul verifies Webmentions by
checking if a link to the target URL is present in the
content of the source URL. The following elements and
their attributes are checked:

| Element  | Attribute |
|----------|-----------|
| `a`      | `href`    |
| `link`   | `href`    |
| `img`    | `src`     |
| `video`  | `src`     |
| `source` | `src`     |
| `iframe` | `src`     |
| `embed`  | `src`     |
| `audio`  | `src`     |
| `area`   | `href`    |
| `input`  | `src`     |
| `script` | `src`     |

If the target URL is present in any of those elements,
Mausul will consider the Webmention valid.
