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
- `errcode.Link(code, message, err) error`: attach a code (and optional user
  message) to an existing error. Keeps `err` in the chain, so callers can
  `errors.Is`/`As` into it (the `%w`-style contract). Returns nil for a nil `err`,
  as `errors.Wrap` does.
- `errcode.Link(code, message, errors.Seal(err))`: the same, with the cause sealed
  (the `%v`-style translation). Callers match the code and cannot match the cause;
  operators still see its text, trace, and attributes. Use it at a boundary so
  switching a dependency cannot break a caller who matched on, say, `sql.ErrNoRows`.
  Seal the cause, not the result: a seal around `Link` hides the code too.
- `errcode.WithCode` and `errcode.WithCodeOpaque` are deprecated spellings of the
  two above. They keep their behaviour, including a coded leaf for a nil cause.
- Every constructor records its call site, so a coded error appears in the
  `%+v` return trace like any other hop.
- `errcode.Status(err) StatusCode`: the outermost code, or `StatusUnknown` if none.
  A wrap can reclassify for its own callers.
- `errcode.Message(err) string`: the outermost non-empty user-facing message, or
  `""`. A coded wrap without a message, `Link(code, "", err)`, only
  reclassifies, so it does not hide the message beneath it. Show this to clients;
  never fall back to `err.Error()`.
- `errcode.Code(err) (StatusCode, string)`: `Status` and `Message` together.
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
	err := errcode.Link(errcode.StatusNotFound, "user not found", cause)

	fmt.Println(err)                 // full detail for logs
	code, msg := errcode.Code(err)
	fmt.Println(code, "/", msg)      // machine code + safe message
}
```

```text
sql: no rows in result set (user not found)
not_found / user not found
```
