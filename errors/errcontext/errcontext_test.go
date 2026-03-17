package errcontext_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/StevenACoffman/toerr/errors/errclass"
	"github.com/StevenACoffman/toerr/errors/errcontext"
	"github.com/StevenACoffman/toerr/errors/xerrors"
	"github.com/stretchr/testify/assert"
)

var errTest = fmt.Errorf("this is a test error")

// TestErrorAs validates that the contextualized error can be cast properly.
func TestErrorAs(t *testing.T) {
	t.Parallel()

	err := errcontext.Add(errTest, slog.String("test", "test"))
	assert.ErrorIs(t, err, errTest)
	extendedError := xerrors.ExtendedError[errcontext.ErrAttr]{}
	assert.ErrorAs(t, err, &extendedError)
	assert.Equal(t, errcontext.ErrAttr{slog.String("test", "test")}, errcontext.Get(err))
}

// TestAddContext validates that context can be added and retrieved.
func TestAddContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		testName string
		err      error
		contexts []errcontext.ErrAttr
	}{
		{
			testName: "nil error",
			err:      nil,
			contexts: nil,
		},
		{
			testName: "no context",
			err:      errTest,
			contexts: nil,
		},
		{
			testName: "single context",
			err:      errTest,
			contexts: []errcontext.ErrAttr{{
				slog.String("one", "one"),
			}},
		},
		{
			testName: "double-sized single context",
			err:      errTest,
			contexts: []errcontext.ErrAttr{{
				slog.String("one", "one"),
				slog.String("two", "two"),
			}},
		},
		{
			testName: "two single contexts",
			err:      errTest,
			contexts: []errcontext.ErrAttr{
				{slog.String("one", "one")},
				{slog.String("two", "two")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			err := tc.err
			var expected errcontext.ErrAttr
			for _, context := range tc.contexts {
				expected = append(expected, context...)
				err = errcontext.Add(err, context...)
			}

			actual := errcontext.Get(err)
			assert.Equal(t, expected, actual)
		})
	}
}

// TestAddContextOverOthers validates that context can be added multiple times.
func TestAddContextOverOthers(t *testing.T) {
	t.Parallel()

	// add some context
	err := errcontext.Add(errTest, slog.String("one", "one"))

	// wrap the error in a different way (add a class)
	err = errclass.WrapAs(err, errclass.Transient)

	// add some more context
	err = errcontext.Add(err, slog.String("two", "two"))

	// ensure the class remains
	assert.Equal(t, errclass.Transient, errclass.GetClass(err))

	// ensure all added context is present
	assert.Equal(t, errcontext.ErrAttr{slog.String("one", "one"), slog.String("two", "two")}, errcontext.Get(err))
}
