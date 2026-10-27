package errgrpc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

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

// A downstream code that describes your call to the service, rather than your
// caller's request, is your service's fault; the rest pass through.
func TestFromError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want errcode.StatusCode
	}{
		"nil":          {nil, errcode.StatusUnknown},
		"not a status": {errors.New("boom"), errcode.StatusUnknown},
		"wrapped status": {
			toerr.Wrap(status.Error(codes.Unavailable, "x")),
			errcode.StatusUnavailable,
		},
		"bare context error": {toerr.Wrap(context.Canceled), errcode.StatusCanceled},
		"bare deadline":      {context.DeadlineExceeded, errcode.StatusDeadlineExceeded},
		// Our request was wrong.
		"invalid argument": {status.Error(codes.InvalidArgument, "x"), errcode.StatusInternal},
		"failed precondition": {
			status.Error(codes.FailedPrecondition, "x"),
			errcode.StatusInternal,
		},
		"out of range":  {status.Error(codes.OutOfRange, "x"), errcode.StatusInternal},
		"unimplemented": {status.Error(codes.Unimplemented, "x"), errcode.StatusInternal},
		// Our credentials or grants were wrong.
		"unauthenticated":   {status.Error(codes.Unauthenticated, "x"), errcode.StatusInternal},
		"permission denied": {status.Error(codes.PermissionDenied, "x"), errcode.StatusInternal},
		// The caller's entity, and the codes that carry retry guidance.
		"not found":      {status.Error(codes.NotFound, "x"), errcode.StatusNotFound},
		"already exists": {status.Error(codes.AlreadyExists, "x"), errcode.StatusAlreadyExists},
		"unavailable":    {status.Error(codes.Unavailable, "x"), errcode.StatusUnavailable},
		"resource exhausted": {
			status.Error(codes.ResourceExhausted, "x"),
			errcode.StatusResourceExhausted,
		},
		"aborted": {status.Error(codes.Aborted, "x"), errcode.StatusAborted},
		"deadline exceeded": {
			status.Error(codes.DeadlineExceeded, "x"),
			errcode.StatusDeadlineExceeded,
		},
		"canceled":  {status.Error(codes.Canceled, "x"), errcode.StatusCanceled},
		"data loss": {status.Error(codes.DataLoss, "x"), errcode.StatusDataLoss},
		"internal":  {status.Error(codes.Internal, "x"), errcode.StatusInternal},
		"unknown":   {status.Error(codes.Unknown, "x"), errcode.StatusUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errgrpc.FromError(tc.err); got != tc.want {
				t.Errorf("FromError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
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
		// As grpc-go's server answers a handler's bare context error, without its text.
		"context canceled": {
			toerr.Wrapf(context.Canceled, "load seat from %s", "pq:users"),
			codes.Canceled, "Canceled",
		},
		"context deadline exceeded": {
			toerr.Wrapf(context.DeadlineExceeded, "load seat from %s", "pq:users"),
			codes.DeadlineExceeded, "DeadlineExceeded",
		},
		"an attached code wins over the context error": {
			errcode.Link(errcode.StatusUnavailable, "", context.Canceled),
			codes.Unavailable, "Unavailable",
		},
		"a sealed context error stays hidden": {
			toerr.Wrap(toerr.Seal(context.Canceled)), codes.Unknown, "Unknown",
		},
		// A downstream status is another service's operator view, not ours to forward.
		"downstream status passed through": {
			toerr.Wrap(downstream), codes.Unknown, "Unknown",
		},
		"downstream status translated": {
			errcode.Link(
				errgrpc.FromError(downstream),
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

func TestAttrs(t *testing.T) {
	detailed, err := status.New(codes.ResourceExhausted, "quota").WithDetails(
		&errdetails.ErrorInfo{
			Reason: "RATE_LIMIT_EXCEEDED", Domain: "users.example.com",
			Metadata: map[string]string{"account": "acct-7"},
		},
		&errdetails.RetryInfo{RetryDelay: durationpb.New(30 * time.Second)},
	)
	if err != nil {
		t.Fatalf("WithDetails: %v", err)
	}
	cases := map[string]struct {
		err  error
		want string
	}{
		"code and details": {
			toerr.Wrap(detailed.Err()),
			"[grpc=[code=ResourceExhausted reason=RATE_LIMIT_EXCEEDED domain=users.example.com retry_delay=30s]]",
		},
		"no details":   {status.Error(codes.Unavailable, "x"), "[grpc=[code=Unavailable]]"},
		"not a status": {errors.New("boom"), "[]"},
		"nil":          {nil, "[]"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := fmt.Sprint(errgrpc.Attrs(tc.err)); got != tc.want {
				t.Errorf("Attrs = %s, want %s", got, tc.want)
			}
		})
	}
}
