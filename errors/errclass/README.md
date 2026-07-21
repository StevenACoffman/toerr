# errclass

**Should you retry? Classify an error's severity so the answer survives wrapping and joining.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errclass.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errclass)

Retry logic needs one coarse fact about a failure: is it worth trying again? A
network blip is `Transient`; a bad request is `Persistent`; a corrupted invariant is
`Panic`. `errclass` attaches that classification transparently (it does not change
the message or identity chain) and reads it back through wrapping. Crucially, it
folds correctly over `errors.Join`: the class of a joined error is the **highest**
class among its members, so a batch containing one persistent failure is persistent
overall — never mistakenly retried.

## Usage

- `errclass.WrapAs(err, class) error` — tag an error with a class (`nil`-safe).
- `errclass.GetClass(err) Class` — read the class back. A joined error returns the
  highest class among its members; an unclassified non-nil error is `Unknown`; `nil`
  is `Nil`.
- Classes, in ascending severity: `Nil` < `Unknown` < `Transient` < `Persistent` <
  `Panic`.

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
	fmt.Println(errclass.GetClass(batch)) // persistent — do not retry the batch
}
```

```text
transient
persistent
```
