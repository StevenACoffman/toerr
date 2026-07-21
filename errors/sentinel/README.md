# sentinel

**Cheap, stack-free sentinel errors that match by identity — like `io.EOF`.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/sentinel.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/sentinel)

The sibling [`errors`](../) package makes `New` capture a program counter. That is
exactly wrong for a package-level sentinel: the counter would point at package
initialization, not at any failure, and you would pay to record it on every
declaration. `sentinel.New` captures nothing. It has no `Is` method either, so
`errors.Is` falls back to pointer equality — a sentinel matches *only itself*, never
another value that merely shares its text. That is what lets a sentinel mean one
specific condition, and it is why it must live in a package-level variable.

## Usage

- `sentinel.New(text) error` — a stack-free error value. Store it at package scope.
- `errors.Is(err, ErrX)` — matches by identity, through wrapping.
- Every sentinel has concrete type `*sentinel.Sentinel` and an `IsSentinel() bool`
  marker, so `AsType[*sentinel.Sentinel]` distinguishes this package's sentinels
  from arbitrary third-party errors.

## Example

```go
package main

import (
	"errors"
	"fmt"

	"github.com/StevenACoffman/toerr/errors/sentinel"
)

var ErrNotFound = sentinel.New("not found")

func main() {
	err := fmt.Errorf("lookup user 42: %w", ErrNotFound)

	fmt.Println(errors.Is(err, ErrNotFound))               // true  — matches through wrapping
	fmt.Println(errors.Is(err, sentinel.New("not found"))) // false — same text, distinct value
}
```

```text
true
false
```
