package errclass_test

import (
	"errors"
	"fmt"

	toerr "github.com/StevenACoffman/toerr/errors"
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

// A recovered panic is classified by the code that recovers it, so the most severe
// class reaches whoever reads the batch.
func ExampleWrapAs_panic() {
	recovered := func() (err error) {
		defer func() {
			err = errclass.WrapAs(toerr.Recover(recover()), errclass.Panic)
		}()
		panic("index out of range")
	}()
	fmt.Println(errclass.GetClass(recovered))
	// Output: panic
}
