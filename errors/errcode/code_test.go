package errcode_test

import (
	"errors"
	"testing"

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
		"message only": {errcode.WithCode(errcode.StatusInternal, "boom", nil), "boom"},
		"cause only": {
			errcode.WithCode(errcode.StatusInternal, "", errors.New("root")),
			"root",
		},
		"neither": {
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
