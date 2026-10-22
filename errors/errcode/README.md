# Errcode

**Tag an error with what kind of failure it is, without your domain code knowing about HTTP or gRPC.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errcode.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errcode)

To turn a failure into a response, the boundary needs to know its *category*: not
found, permission denied, already exists. Baking an HTTP status into the error
couples your domain to a transport, and matching on message strings is fragile. `errcode`
attaches a transport-neutral `StatusCode` and an optional user-facing message at the
point the failure is understood. An adapter such as [`errhttp`](../errhttp/) maps the
code to a transport later. `Message` gives you the safe, client-facing string: it
never exposes the wrapped internal detail that `err.Error()` would.

## Usage

- `errcode.New(code, message) error`: a coded leaf, where the failure is first
  understood. It has a code and a user message and no cause.
- `errcode.WithCode(code, message, cause) error`: attach a code (and optional
  user message) to an existing error. Keeps `cause` in the chain, so callers can
  `errors.Is`/`As` into it (the `%w`-style contract). It is a wrap: a nil `cause`
  still makes a leaf but is deprecated, and a future release will return nil for
  it, as `errors.Wrap` does. Use `New` for leaves.
- `errcode.WithCodeOpaque(code, message, cause) error`: same, but **severs** the
  chain (the `%v`-style translation). The cause's text survives in `Error()` for
  operators, but callers cannot match its type. Use it at a boundary so switching a
  dependency cannot break a caller who matched on, say, `sql.ErrNoRows`.
- Every constructor records its call site, so a coded error appears in the
  `%+v` return trace like any other hop.
- `errcode.Code(err) (StatusCode, string)` / `errcode.Status(err) StatusCode`:
  read the code back, or `StatusUnknown` if none.
- `errcode.Message(err) string`: the user-facing message, or `""`. Show this to
  clients; never fall back to `err.Error()`.
- Codes: `StatusInvalidArgument`, `StatusNotFound`, `StatusUnauthenticated`,
  `StatusPermissionDenied`, `StatusAlreadyExists`, `StatusInternal`, and more.

## Example

```go
package main

import (
	"errors"
	"fmt"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

func main() {
	cause := errors.New("sql: no rows in result set")
	err := errcode.WithCode(errcode.StatusNotFound, "user not found", cause)

	fmt.Println(err)                 // full detail for logs
	code, msg := errcode.Code(err)
	fmt.Println(code, "/", msg)      // machine code + safe message
}
```

```text
sql: no rows in result set (user not found)
not_found / user not found
```
