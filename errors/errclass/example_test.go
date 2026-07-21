package errclass_test

import (
	"errors"
	"fmt"

	"github.com/StevenACoffman/toerr/errors/errclass"
)

// GetClass returns the highest severity among the members of a joined error, so a
// batch containing one persistent failure is persistent overall.
func ExampleGetClass() {
	transient := errclass.WrapAs(errors.New("connection reset"), errclass.Transient)
	fmt.Println(errclass.GetClass(transient))

	batch := errors.Join(
		errclass.WrapAs(errors.New("read timeout"), errclass.Transient),
		errclass.WrapAs(errors.New("malformed record"), errclass.Persistent),
	)
	fmt.Println(errclass.GetClass(batch))
	// Output:
	// transient
	// persistent
}
