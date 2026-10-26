# Errclass

**Should you retry? Classify an error's severity so the answer survives wrapping and joining.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errclass.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errclass)

Retry logic needs one coarse fact about a failure: is it worth trying again? A
network blip is `Transient`; a bad request is `Persistent`; a corrupted invariant is
`Panic`. `errclass` attaches that classification transparently (it does not change
the message or identity chain) and reads it back through wrapping. It folds
correctly over `errors.Join`. The class of a joined error is the **highest** class
among its members, so one persistent failure in a batch makes the whole batch
persistent, and the batch is never mistakenly retried.

## Usage

- `errclass.WrapAs(err, class) error`: tag an error with a class (`nil`-safe).
- `errclass.GetClass(err) Class`: read the class back. A joined error returns the
  highest class among its members; an unclassified non-nil error is `Unknown`; `nil`
  is `Nil`.
- Classes, in ascending severity: `Nil` < `Unknown` < `Transient` < `Persistent` <
  `Panic`.
- `Panic` is for a recovered panic. `errors.Recover` does not set it, because the
  root package imports none of its subpackages, so classify the result yourself:
  `errclass.WrapAs(errors.Recover(recover()), errclass.Panic)`.

## At a Boundary

`GetClass` finds a class anywhere in the chain, so a `Transient` set deep inside a
dependency still answers at the top. A retryable cause doesn't make the outer
operation retryable, for example when that operation isn't idempotent. Judge the
class where the error crosses into another domain: reclassify with `WrapAs`, or seal
the cause with `errors.Seal`, which `GetClass` cannot see past. Across a service or
package boundary, prefer an [`errcode`](../errcode/) status, whose codes say whether
a retry can help.

## Example

```go
package main

import (
	"errors"
	"fmt"

	"github.com/StevenACoffman/toerr/errors/errclass"
)

func main() {
	transient := errclass.WrapAs(errors.New("connection reset"), errclass.Transient)
	fmt.Println(errclass.GetClass(transient)) // transient

	// A batch: one transient, one persistent. Highest severity wins.
	batch := errors.Join(
		errclass.WrapAs(errors.New("read timeout"), errclass.Transient),
		errclass.WrapAs(errors.New("malformed record"), errclass.Persistent),
	)
	fmt.Println(errclass.GetClass(batch)) // persistent, do not retry the batch
}
```

```text
transient
persistent
```
