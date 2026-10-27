package errclass_test

import (
	"encoding/json"
	"errors"
	"testing"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errclass"
)

var (
	errTest    = errors.New("this is a test error")
	errTestToo = errors.New("this is also test error")
)

func TestErrClass(t *testing.T) {
	testCases := []struct {
		testName string
		err      error
		class    errclass.Class
	}{
		{
			testName: "nil error",
			err:      nil,
			class:    errclass.Nil,
		},
		{
			testName: "unknown error",
			err:      errTest,
			class:    errclass.Unknown,
		},
		{
			testName: "panic error",
			err:      errTest,
			class:    errclass.Panic,
		},
		{
			testName: "transient error",
			err:      errTest,
			class:    errclass.Transient,
		},
		{
			testName: "persistent error",
			err:      errTest,
			class:    errclass.Persistent,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			err := errclass.WrapAs(tc.err, tc.class)
			class := errclass.GetClass(err)
			if class != tc.class {
				t.Errorf("unexpected error class: want: %s got %s", tc.class, class)
			}
		})
	}
}

func TestErrClassUnknown(t *testing.T) {
	// errTest doesn't have a class assigned
	class := errclass.GetClass(errTest)
	if class != errclass.Unknown {
		t.Errorf("unexpected error class: want: %s got %s", errclass.Unknown, class)
	}
}

func TestErrClassJoined(t *testing.T) {
	testCases := []struct {
		testName      string
		classA        errclass.Class
		classB        errclass.Class
		expectedClass errclass.Class
	}{
		{
			testName:      "nil nil",
			classA:        errclass.Nil,
			classB:        errclass.Nil,
			expectedClass: errclass.Nil,
		},
		{
			testName:      "nil unknown",
			classA:        errclass.Nil,
			classB:        errclass.Unknown,
			expectedClass: errclass.Unknown,
		},
		{
			testName:      "nil transient",
			classA:        errclass.Nil,
			classB:        errclass.Transient,
			expectedClass: errclass.Transient,
		},
		{
			testName:      "nil persistent",
			classA:        errclass.Nil,
			classB:        errclass.Persistent,
			expectedClass: errclass.Persistent,
		},
		{
			testName:      "nil panic",
			classA:        errclass.Nil,
			classB:        errclass.Panic,
			expectedClass: errclass.Panic,
		},
		{
			testName:      "unknown unknown",
			classA:        errclass.Unknown,
			classB:        errclass.Unknown,
			expectedClass: errclass.Unknown,
		},
		{
			testName:      "unknown transient",
			classA:        errclass.Unknown,
			classB:        errclass.Transient,
			expectedClass: errclass.Transient,
		},
		{
			testName:      "unknown persistent",
			classA:        errclass.Unknown,
			classB:        errclass.Persistent,
			expectedClass: errclass.Persistent,
		},
		{
			testName:      "unknown panic",
			classA:        errclass.Unknown,
			classB:        errclass.Panic,
			expectedClass: errclass.Panic,
		},
		{
			testName:      "transient transient",
			classA:        errclass.Transient,
			classB:        errclass.Transient,
			expectedClass: errclass.Transient,
		},
		{
			testName:      "transient persistent",
			classA:        errclass.Transient,
			classB:        errclass.Persistent,
			expectedClass: errclass.Persistent,
		},
		{
			testName:      "transient panic",
			classA:        errclass.Transient,
			classB:        errclass.Panic,
			expectedClass: errclass.Panic,
		},
		{
			testName:      "persistent persistent",
			classA:        errclass.Persistent,
			classB:        errclass.Persistent,
			expectedClass: errclass.Persistent,
		},
		{
			testName:      "persistent panic",
			classA:        errclass.Persistent,
			classB:        errclass.Panic,
			expectedClass: errclass.Panic,
		},
		{
			testName:      "panic panic",
			classA:        errclass.Panic,
			classB:        errclass.Panic,
			expectedClass: errclass.Panic,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			errA := errclass.WrapAs(errTest, tc.classA)
			if tc.classA == errclass.Nil {
				errA = nil
			}
			errB := errclass.WrapAs(errTestToo, tc.classB)
			if tc.classB == errclass.Nil {
				errB = nil
			}

			errC := errors.Join(errA, errB)
			class := errclass.GetClass(errC)
			if class != tc.expectedClass {
				t.Errorf("unexpected error class: want: %s got %s", tc.expectedClass, class)
			}

			// Joining in the opposite order should yield the same result
			errD := errors.Join(errB, errA)
			class = errclass.GetClass(errD)
			if class != tc.expectedClass {
				t.Errorf("unexpected error class: want: %s got %s", tc.expectedClass, class)
			}
		})
	}
}

// A boundary judges the class itself, so a seal must hide the class beneath it and
// leave the one set outside it.
func TestSealHidesTheClassBeneath(t *testing.T) {
	inner := errclass.WrapAs(errTest, errclass.Transient)
	cases := map[string]struct {
		err  error
		want errclass.Class
	}{
		"unsealed": {toerr.Wrap(inner), errclass.Transient},
		"sealed":   {toerr.Wrap(toerr.Seal(inner)), errclass.Unknown},
		"reclassified outside the seal": {
			errclass.WrapAs(toerr.Seal(inner), errclass.Persistent),
			errclass.Persistent,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errclass.GetClass(tc.err); got != tc.want {
				t.Errorf("GetClass = %s, want %s", got, tc.want)
			}
		})
	}
}

// errors.LogValue gives a classified error a layer that names the class.
func TestLogValueShowsTheClass(t *testing.T) {
	err := errclass.WrapAs(errTest, errclass.Transient)

	data, _ := toerr.LogValue(err).Any().(json.Marshaler).MarshalJSON()
	want := `{"msg":"this is a test error","chain":[{"class":"transient"},` +
		`{"msg":"this is a test error"}]}`
	if string(data) != want {
		t.Errorf("got\n%s\nwant\n%s", data, want)
	}
}
