package errcode_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

func TestCodeRoundTrip(t *testing.T) {
	cause := errors.New("sql: no rows")
	err := errcode.WithCode(errcode.StatusNotFound, "user not found", cause)

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
			errcode.WithCode(errcode.StatusInternal, "boom", errors.New("root")),
			"root (boom)",
		},
		"leaf": {errcode.New(errcode.StatusInternal, "boom"), "boom"},
		"leaf without message": {
			errcode.New(errcode.StatusNotFound, ""),
			"error: not_found",
		},
		"cause only": {
			errcode.WithCode(errcode.StatusInternal, "", errors.New("root")),
			"root",
		},
		// The deprecated leaf spelling, pinned until WithCode becomes nil-in nil-out.
		"deprecated leaf": {errcode.WithCode(errcode.StatusInternal, "boom", nil), "boom"},
		"deprecated leaf without message": {
			errcode.WithCode(errcode.StatusNotFound, "", nil),
			"error: not_found",
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
		"empty message": {errcode.WithCode(errcode.StatusInternal, "", errors.New("boom")), ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errcode.Message(tc.err); got != tc.want {
				t.Errorf("Message() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWithCodeOpaqueSeversChain(t *testing.T) {
	cause := errors.New("sql: no rows in result set")

	// WithCode keeps the cause reachable.
	transparent := errcode.WithCode(errcode.StatusNotFound, "user not found", cause)
	if !errors.Is(transparent, cause) {
		t.Error("WithCode should keep the cause in the chain")
	}

	// WithCodeOpaque severs it, while code, message, and operator text survive.
	opaque := errcode.WithCodeOpaque(errcode.StatusNotFound, "user not found", cause)
	if errors.Is(opaque, cause) {
		t.Error("WithCodeOpaque should sever the cause from errors.Is")
	}
	if errcode.Status(opaque) != errcode.StatusNotFound {
		t.Errorf("Status = %v, want StatusNotFound", errcode.Status(opaque))
	}
	if errcode.Message(opaque) != "user not found" {
		t.Errorf("Message = %q, want %q", errcode.Message(opaque), "user not found")
	}
	if got, want := opaque.Error(), "sql: no rows in result set (user not found)"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestStatusCodeString(t *testing.T) {
	if errcode.StatusNotFound.String() != "not_found" {
		t.Errorf(
			"StatusNotFound.String() = %q, want %q",
			errcode.StatusNotFound.String(),
			"not_found",
		)
	}
	if errcode.StatusCode(999).String() != "unknown" {
		t.Errorf("unknown code String() = %q, want %q", errcode.StatusCode(999).String(), "unknown")
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
			errcode.WithCode(errcode.StatusInternal, "", toerr.New("disk full")),
			2,
		},
		// Severing flattens the cause to text, frames included; the boundary keeps one.
		"opaque": {
			errcode.WithCodeOpaque(errcode.StatusInternal, "", toerr.New("disk full")),
			1,
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
