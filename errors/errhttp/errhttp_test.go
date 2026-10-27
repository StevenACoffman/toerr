package errhttp_test

import (
	"context"
	"net/http"
	"testing"

	toerr "github.com/StevenACoffman/toerr/errors"
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
		"unavailable":       {errcode.StatusUnavailable, http.StatusServiceUnavailable},
		"exhausted":         {errcode.StatusResourceExhausted, http.StatusTooManyRequests},
		"aborted":           {errcode.StatusAborted, http.StatusConflict},
		"out of range":      {errcode.StatusOutOfRange, http.StatusBadRequest},
		"data loss":         {errcode.StatusDataLoss, http.StatusInternalServerError},
		// google/rpc/code.proto's HTTP mapping, which grpc-gateway follows.
		"canceled":          {errcode.StatusCanceled, errhttp.StatusClientClosedRequest},
		"deadline exceeded": {errcode.StatusDeadlineExceeded, http.StatusGatewayTimeout},
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

// An error without a code that wraps a context error is not a server failure.
func TestErrorAnswersAContextError(t *testing.T) {
	cases := map[string]struct {
		err        error
		wantStatus int
		wantMsg    string
	}{
		"canceled": {
			toerr.Wrapf(context.Canceled, "load seat"),
			errhttp.StatusClientClosedRequest, "Client Closed Request",
		},
		"deadline exceeded": {
			toerr.Wrapf(context.DeadlineExceeded, "load seat"),
			http.StatusGatewayTimeout, http.StatusText(http.StatusGatewayTimeout),
		},
		"an attached code wins": {
			errcode.Link(errcode.StatusUnavailable, "", context.Canceled),
			http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			status, msg := errhttp.Error(tc.err)
			if status != tc.wantStatus || msg != tc.wantMsg {
				t.Errorf("Error = (%d, %q), want (%d, %q)", status, msg, tc.wantStatus, tc.wantMsg)
			}
		})
	}
}

func TestErrorMapsAttachedCode(t *testing.T) {
	err := errcode.New(errcode.StatusPermissionDenied, "nope")
	status, msg := errhttp.Error(err)
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want %d", status, http.StatusForbidden)
	}
	if msg != "nope" {
		t.Errorf("message = %q, want %q", msg, "nope")
	}
}
