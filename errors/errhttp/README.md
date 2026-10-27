# Errhttp

**Turn a coded error into an HTTP status and a client-safe message, in one call.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errhttp.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errhttp)

[`errcode`](../errcode/) keeps error categories transport-neutral so your domain
never imports `net/http`. `errhttp` is the adapter that lives at the boundary and
does the mapping: give it an error and it returns the HTTP status plus a message
safe to send to a client. Unknown or unset codes fall back to `500` and the standard
HTTP status text, and it never returns `err.Error()`, so wrapped internal detail is
not leaked to callers.

The mapping is the one `google/rpc/code.proto` gives each gRPC code, which
grpc-gateway also uses. A cancelled request answers `499 Client Closed Request`
(`errhttp.StatusClientClosedRequest`, since `net/http` has no constant for it), and an
error without a code that wraps `context.Canceled` or `context.DeadlineExceeded`
answers `499` or `504`, not `500`: the client left, or time ran out.

A proxy that retries on status reads less than the code said. `409` is both
`AlreadyExists`, which no retry fixes, and `Aborted`, which a retry of the whole
transaction can, and `504` is a `DeadlineExceeded` that only an idempotent request
should repeat. Turn proxy retries on only for idempotent routes; see the guide's
[Deciding Retries at a Boundary](../../philosophy.md#deciding-retries-at-a-boundary).

## Usage

- `errhttp.Error(err) (int, string)`: map the code attached to `err` to an HTTP
  status and message. Use this at the transport boundary.
- `errhttp.Status(code) int`: map an `errcode.StatusCode` to an HTTP status.
- `errhttp.StatusMessage(code, message) (int, string)`: same, defaulting an empty
  message to the standard status text.

## Example

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errhttp"
)

func handle(w http.ResponseWriter, err error) {
	status, msg := errhttp.Error(err)
	http.Error(w, msg, status)
	fmt.Println(status, msg) // for illustration
}

func main() {
	handle(nil, errcode.New(errcode.StatusPermissionDenied, "you cannot edit this"))
	handle(nil, errcode.New(errcode.StatusNotFound, "")) // no message → default text
}
```

```text
403 you cannot edit this
404 Not Found
```
