package errcode_test

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

func TestCodeRoundTrip(t *testing.T) {
	cause := errors.New("sql: no rows")
	err := errcode.Link(errcode.StatusNotFound, "user not found", cause)

	code, msg := errcode.Code(err)
	if code != errcode.StatusNotFound {
		t.Errorf("Code = %v, want StatusNotFound", code)
	}
	if msg != "user not found" {
		t.Errorf("message = %q, want %q", msg, "user not found")
	}
	if errcode.Status(err) != errcode.StatusNotFound {
		t.Errorf("Status = %v, want StatusNotFound", errcode.Status(err))
	}
	if !errors.Is(err, cause) {
		t.Error("Is(err, cause) = false, want true (cause should be in the chain)")
	}
}

// Code answers what Status and Message answer: the outermost code, which a wrap may
// change, and the outermost message, which a wrap without one leaves alone.
func TestCodeAgreesWithStatusAndMessage(t *testing.T) {
	err := errcode.Link(
		errcode.StatusInvalidArgument,
		"",
		errcode.New(errcode.StatusNotFound, "no such user"),
	)

	code, msg := errcode.Code(err)
	if code != errcode.StatusInvalidArgument || msg != "no such user" {
		t.Errorf("Code = (%v, %q), want (invalid_argument, %q)", code, msg, "no such user")
	}
	if code != errcode.Status(err) || msg != errcode.Message(err) {
		t.Errorf("Code = (%v, %q), Status/Message = (%v, %q)",
			code, msg, errcode.Status(err), errcode.Message(err))
	}
}

func TestCodeMissing(t *testing.T) {
	// A plain error carries no code.
	code, msg := errcode.Code(errors.New("plain"))
	if code != errcode.StatusUnknown || msg != "" {
		t.Errorf("Code(plain) = (%v, %q), want (StatusUnknown, \"\")", code, msg)
	}
}

func TestErrorMessage(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"message and cause": {
			errcode.Link(errcode.StatusInternal, "boom", errors.New("root")),
			"root (boom)",
		},
		"leaf": {errcode.New(errcode.StatusInternal, "boom"), "boom"},
		"leaf without message": {
			errcode.New(errcode.StatusNotFound, ""),
			"not found",
		},
		"leaf without message, two-word code": {
			errcode.New(errcode.StatusFailedPrecondition, ""),
			"failed precondition",
		},
		"leaf without message in a chain": {
			toerr.Wrapf(errcode.New(errcode.StatusNotFound, ""), "load seat %d", 7),
			"load seat 7: not found",
		},
		"cause only": {
			errcode.Link(errcode.StatusInternal, "", errors.New("root")),
			"root",
		},
		// The deprecated leaf spelling keeps its meaning for good.
		"deprecated leaf": {errcode.WithCode(errcode.StatusInternal, "boom", nil), "boom"},
		"deprecated leaf without message": {
			errcode.WithCode(errcode.StatusNotFound, "", nil),
			"not found",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"with message": {
			errcode.New(errcode.StatusNotFound, "user not found"),
			"user not found",
		},
		"empty message": {errcode.Link(errcode.StatusInternal, "", errors.New("boom")), ""},
		// A coded wrap without a message only reclassifies; the message beneath shows.
		"classify-only wrap": {
			errcode.Link(
				errcode.StatusInvalidArgument,
				"",
				errcode.New(errcode.StatusNotFound, "no such user"),
			),
			"no such user",
		},
		"classify-only wrap under a plain wrap": {
			toerr.WrapWithMessage(
				errcode.Link(
					errcode.StatusInvalidArgument,
					"",
					errcode.New(errcode.StatusNotFound, "no such user"),
				),
				"svc.Get",
			),
			"no such user",
		},
		"outer message wins": {
			errcode.Link(
				errcode.StatusInternal,
				"try again later",
				errcode.New(errcode.StatusNotFound, "no such user"),
			),
			"try again later",
		},
		// A seal hides the message beneath it along with everything else callers match.
		"sealed cause hides the message beneath": {
			errcode.Link(
				errcode.StatusInternal,
				"",
				toerr.Seal(errcode.New(errcode.StatusNotFound, "no such user")),
			),
			"",
		},
		"deprecated opaque hides the message beneath": {
			errcode.WithCodeOpaque(
				errcode.StatusInternal,
				"",
				errcode.New(errcode.StatusNotFound, "no such user"),
			),
			"",
		},
		"no code": {errors.New("plain"), ""},
		"nil":     {nil, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errcode.Message(tc.err); got != tc.want {
				t.Errorf("Message() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLinkNil(t *testing.T) {
	if err := errcode.Link(errcode.StatusInternal, "boom", nil); err != nil {
		t.Errorf("Link(…, nil) = %v, want nil", err)
	}
}

// A sealed cause is how a dependency's error becomes a domain code: callers match
// the code and nothing beneath it, while operators keep the whole chain. Each case
// builds the same failure; the deprecated WithCodeOpaque must agree with Link and
// Seal in everything but its spelling.
func TestLinkSealedCause(t *testing.T) {
	dependency := errors.New("sql: no rows in result set")
	cases := map[string]func(cause error) error{
		"Link and Seal": func(cause error) error {
			return errcode.Link(errcode.StatusNotFound, "user not found", toerr.Seal(cause))
		},
		"deprecated WithCodeOpaque": func(cause error) error {
			return errcode.WithCodeOpaque(errcode.StatusNotFound, "user not found", cause)
		},
	}
	// view is everything each audience observes about one error.
	type view struct {
		ReachesDependency bool
		Status            errcode.StatusCode
		Message, Text     string
		Attrs             string
		Frames            int
	}
	want := view{
		Status:  errcode.StatusNotFound,
		Message: "user not found",
		Text:    "query: sql: no rows in result set (user not found)",
		Attrs:   "[shard=7]",
		Frames:  2, // the cause's and the boundary's
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			err := build(toerr.WrapWithMessage(dependency, "query", slog.Int("shard", 7)))
			trace := fmt.Sprintf("%+v", err)
			got := view{
				ReachesDependency: errors.Is(err, dependency),
				Status:            errcode.Status(err),
				Message:           errcode.Message(err),
				Text:              err.Error(),
				Attrs:             fmt.Sprint(toerr.Attrs(err)),
				Frames:            strings.Count(trace, "code_test.go:"),
			}
			if got != want {
				t.Errorf("got  %+v\nwant %+v\n%s", got, want, trace)
			}
		})
	}
}

// Sealing the result instead of the cause hides the code just attached. This pins
// the mistake the Link and Seal docs warn about, so a change to it is deliberate.
func TestSealingTheResultHidesTheCode(t *testing.T) {
	err := toerr.Seal(errcode.Link(errcode.StatusNotFound, "user not found", errors.New("boom")))

	if errcode.Status(err) != errcode.StatusUnknown {
		t.Errorf("Status = %v, want StatusUnknown", errcode.Status(err))
	}
	if errcode.Message(err) != "" {
		t.Errorf("Message = %q, want empty", errcode.Message(err))
	}
}

func TestStatusCodeString(t *testing.T) {
	cases := map[string]struct {
		code errcode.StatusCode
		want string
	}{
		"one word":           {errcode.StatusUnavailable, "unavailable"},
		"two words":          {errcode.StatusNotFound, "not_found"},
		"resource exhausted": {errcode.StatusResourceExhausted, "resource_exhausted"},
		"aborted":            {errcode.StatusAborted, "aborted"},
		"out of range":       {errcode.StatusOutOfRange, "out_of_range"},
		"data loss":          {errcode.StatusDataLoss, "data_loss"},
		"outside the set":    {errcode.StatusCode(999), "unknown"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.code.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A coded error is a hop like any other, so it must appear in the %+v return trace,
// at its caller's line rather than inside this package. A coded leaf is the hop a
// reader most needs: it is where the failure was understood.
func TestACodedErrorContributesItsCallSite(t *testing.T) {
	cases := map[string]struct {
		err    error
		frames int
	}{
		"leaf": {errcode.New(errcode.StatusNotFound, "no such user"), 1},
		"deprecated leaf": {
			errcode.WithCode(errcode.StatusNotFound, "no such user", nil),
			1,
		},
		"leaf under a wrap": {
			toerr.WrapWithMessage(errcode.New(errcode.StatusNotFound, "no such user"), "svc.Get"),
			2,
		},
		"coded wrap": {
			errcode.Link(errcode.StatusInternal, "", toerr.New("disk full")),
			2,
		},
		// A seal keeps the frames beneath it for operators.
		"sealed cause": {
			errcode.Link(errcode.StatusInternal, "", toerr.Seal(toerr.New("disk full"))),
			2,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := fmt.Sprintf("%+v", tc.err)
			if n := strings.Count(got, "code_test.go:"); n != tc.frames {
				t.Errorf("%d frames at the call site, want %d:\n%s", n, tc.frames, got)
			}
			if strings.Contains(got, "errcode/code.go") {
				t.Errorf("a frame points inside errcode:\n%s", got)
			}
			if !strings.HasPrefix(got, tc.err.Error()+"\n") {
				t.Errorf("%%+v does not lead with the message %q:\n%s", tc.err.Error(), got)
			}
		})
	}
}
