package errgrpc_test

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errgrpc"
)

// allCodes is every errcode status. A code added to errcode and not here leaves the
// round trip unchecked, so keep it in step with errcode's const block.
var allCodes = []errcode.StatusCode{
	errcode.StatusUnknown, errcode.StatusCanceled, errcode.StatusInvalidArgument,
	errcode.StatusDeadlineExceeded, errcode.StatusInternal, errcode.StatusNotFound,
	errcode.StatusUnauthenticated, errcode.StatusPermissionDenied, errcode.StatusAlreadyExists,
	errcode.StatusFailedPrecondition, errcode.StatusUnimplemented, errcode.StatusUnavailable,
	errcode.StatusResourceExhausted, errcode.StatusAborted, errcode.StatusOutOfRange,
	errcode.StatusDataLoss,
}

// The two sets match apart from OK, so a code that goes out and comes back must be
// the code that went out, and no two domain codes may share a gRPC code.
func TestCodeRoundTrips(t *testing.T) {
	seen := map[codes.Code]errcode.StatusCode{}
	for _, code := range allCodes {
		t.Run(code.String(), func(t *testing.T) {
			out := errgrpc.Code(code)
			if back := errgrpc.FromCode(out); back != code {
				t.Errorf("FromCode(Code(%v)) = %v", code, back)
			}
			if prev, ok := seen[out]; ok {
				t.Errorf("%v and %v both map to %v", prev, code, out)
			}
			seen[out] = code
		})
	}
}

func TestFromCodeOutsideTheDomain(t *testing.T) {
	cases := map[string]codes.Code{
		"OK is not an error": codes.OK,
		"outside the set":    codes.Code(99),
	}
	for name, code := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errgrpc.FromCode(code); got != errcode.StatusUnknown {
				t.Errorf("FromCode(%v) = %v, want StatusUnknown", code, got)
			}
		})
	}
}

func TestError(t *testing.T) {
	secret := errors.New("pq: relation \"users\" does not exist")
	downstream := status.Error(codes.NotFound, "row 7 missing in shard users_3")
	cases := map[string]struct {
		err         error
		wantCode    codes.Code
		wantMessage string
	}{
		"coded leaf": {
			errcode.New(errcode.StatusPermissionDenied, "you cannot edit this"),
			codes.PermissionDenied, "you cannot edit this",
		},
		"coded wrap without a message": {
			errcode.Link(errcode.StatusUnavailable, "", secret),
			codes.Unavailable, "Unavailable",
		},
		"uncoded error": {toerr.Wrap(secret), codes.Unknown, "Unknown"},
		// A downstream status is another service's operator view, not ours to forward.
		"downstream status passed through": {
			toerr.Wrap(downstream), codes.Unknown, "Unknown",
		},
		"downstream status translated": {
			errcode.Link(
				errgrpc.FromCode(status.Code(downstream)),
				"",
				toerr.Seal(downstream),
			),
			codes.NotFound, "NotFound",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := status.Convert(errgrpc.Error(tc.err))
			if s.Code() != tc.wantCode || s.Message() != tc.wantMessage {
				t.Errorf("status = (%v, %q), want (%v, %q)",
					s.Code(), s.Message(), tc.wantCode, tc.wantMessage)
			}
			for _, internal := range []string{"pq:", "shard"} {
				if strings.Contains(s.Message(), internal) {
					t.Errorf("message %q leaks internal detail %q", s.Message(), internal)
				}
			}
		})
	}
}

func TestErrorNil(t *testing.T) {
	if err := errgrpc.Error(nil); err != nil {
		t.Errorf("Error(nil) = %v, want nil", err)
	}
}
