package errhttp_test

import (
	"net/http"
	"testing"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errhttp"
)

func TestStatus(t *testing.T) {
	cases := map[string]struct {
		code errcode.StatusCode
		want int
	}{
		"not found":         {errcode.StatusNotFound, http.StatusNotFound},
		"invalid argument":  {errcode.StatusInvalidArgument, http.StatusBadRequest},
		"unauthenticated":   {errcode.StatusUnauthenticated, http.StatusUnauthorized},
		"permission denied": {errcode.StatusPermissionDenied, http.StatusForbidden},
		"already exists":    {errcode.StatusAlreadyExists, http.StatusConflict},
		"internal":          {errcode.StatusInternal, http.StatusInternalServerError},
		"unknown":           {errcode.StatusUnknown, http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errhttp.Status(tc.code); got != tc.want {
				t.Errorf("Status(%v) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

func TestStatusMessageDefaultsText(t *testing.T) {
	status, msg := errhttp.StatusMessage(errcode.StatusNotFound, "")
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", status, http.StatusNotFound)
	}
	if msg != http.StatusText(http.StatusNotFound) {
		t.Errorf(
			"message = %q, want default status text %q",
			msg,
			http.StatusText(http.StatusNotFound),
		)
	}
}

func TestErrorMapsAttachedCode(t *testing.T) {
	err := errcode.WithCode(errcode.StatusPermissionDenied, "nope", nil)
	status, msg := errhttp.Error(err)
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want %d", status, http.StatusForbidden)
	}
	if msg != "nope" {
		t.Errorf("message = %q, want %q", msg, "nope")
	}
}
