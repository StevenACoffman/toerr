# errhttp

**Turn a coded error into an HTTP status and a client-safe message, in one call.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errhttp.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errhttp)

[`errcode`](../errcode/) keeps error categories transport-neutral so your domain
never imports `net/http`. `errhttp` is the adapter that lives at the boundary and
does the mapping: give it an error and it returns the HTTP status plus a message
safe to send to a client. Unknown or unset codes fall back to `500` and the standard
HTTP status text, and it never returns `err.Error()`, so wrapped internal detail is
not leaked to callers.

## Usage

- `errhttp.Error(err) (int, string)` — map the code attached to `err` to an HTTP
  status and message. Use this at the transport boundary.
- `errhttp.Status(code) int` — map an `errcode.StatusCode` to an HTTP status.
- `errhttp.StatusMessage(code, message) (int, string)` — same, defaulting an empty
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
	handle(nil, errcode.WithCode(errcode.StatusPermissionDenied, "you cannot edit this", nil))
	handle(nil, errcode.WithCode(errcode.StatusNotFound, "", nil)) // no message → default text
}
```

```text
403 you cannot edit this
404 Not Found
```
