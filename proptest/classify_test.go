package proptest_test

import (
	"cmp"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errclass"
	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errhttp"
)

var (
	classes = []errclass.Class{
		errclass.Unknown,
		errclass.Transient,
		errclass.Persistent,
		errclass.Panic,
	}
	codes = []errcode.StatusCode{
		errcode.StatusUnknown, errcode.StatusCanceled, errcode.StatusInvalidArgument,
		errcode.StatusDeadlineExceeded, errcode.StatusInternal, errcode.StatusNotFound,
		errcode.StatusUnauthenticated, errcode.StatusPermissionDenied, errcode.StatusAlreadyExists,
		errcode.StatusFailedPrecondition, errcode.StatusUnimplemented,
	}
)

// TestJoinedClassIsTheMostSevere checks that a joined error is classified by its most severe
// member, and that an unclassified member counts as Unknown rather than being skipped.
//
// Retry and alerting decisions read the class of whatever came back, and a Join is how
// concurrent work comes back. If one Panic among several Transients classified as Transient,
// the caller would retry a crash.
func TestJoinedClassIsTheMostSevere(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(t, "members")
		members := make([]error, n)
		want := errclass.Nil
		for i := range members {
			err := errors.New("member")
			class := errclass.Unknown
			if rapid.Bool().Draw(t, "classified") {
				class = rapid.SampledFrom(classes).Draw(t, "class")
				err = errclass.WrapAs(err, class)
			}
			// Wrapping after classification must not hide the class.
			members[i] = errors.Wrap(err)
			want = max(want, class)
		}

		if got := errclass.GetClass(errors.Join(members...)); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

// TestOpaqueCodeSeversIdentityButKeepsText checks WithCodeOpaque's contract for any cause:
// the code and message survive, the cause's text survives for operators, and the cause itself
// is no longer reachable by errors.Is.
func TestOpaqueCodeSeversIdentityButKeepsText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cause := stderrors.New(rapid.String().Draw(t, "cause"))
		code := rapid.SampledFrom(codes).Draw(t, "code")
		message := rapid.String().Draw(t, "message")

		err := errors.Wrap(errcode.WithCodeOpaque(code, message, cause))

		if errors.Is(err, cause) {
			t.Fatalf("errors.Is still reaches the cause through WithCodeOpaque")
		}
		if !strings.Contains(err.Error(), cause.Error()) {
			t.Fatalf("Error() %q dropped the cause text %q", err.Error(), cause.Error())
		}
		if gotCode, gotMessage := errcode.Code(err); gotCode != code || gotMessage != message {
			t.Fatalf("Code() = (%v, %q), want (%v, %q)", gotCode, gotMessage, code, message)
		}
	})
}

// TestHTTPErrorNeverExposesTheCause checks errhttp.Error's safety contract: the message it
// returns is the user-facing message attached with errcode, or the standard status text, and
// never the wrapped internal detail.
//
// The cause text is drawn so that it cannot coincide with a status text, which makes "the
// cause leaked" and "the message is exactly what was allowed" the same check.
func TestHTTPErrorNeverExposesTheCause(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		secret := "secret:" + rapid.String().Draw(t, "secret")
		err := errors.WrapWithMessage(errors.New(secret), "loading row")

		var code errcode.StatusCode
		var message string
		if rapid.Bool().Draw(t, "coded") {
			code = rapid.SampledFrom(codes).Draw(t, "code")
			message = rapid.SampledFrom([]string{"", "Try again later."}).Draw(t, "message")
			err = errors.Wrap(errcode.WithCode(code, message, err))
		}

		status, got := errhttp.Error(err)

		if status < 400 || status > 599 {
			t.Fatalf("status %d is not an error status", status)
		}
		want := message
		if want == "" {
			want = http.StatusText(status)
		}
		if got != want {
			t.Fatalf("message %q, want %q (cause was %q)", got, want, secret)
		}
	})
}

// TestCodedChainReportsOutermostCodeAndMessage checks how Status and Message read any chain
// of plain wraps, coded leaves and coded wraps. The code is the outermost one, because a wrap
// reclassifies for its own callers. The message is the outermost non-empty one, because a
// coded wrap without a message only reclassifies and must not hide the text an end user would
// otherwise see.
func TestCodedChainReportsOutermostCodeAndMessage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		messages := []string{"", "Not found.", "Try again later."}
		wantCode, wantMessage := errcode.StatusUnknown, ""
		err := stderrors.New("root")
		if rapid.Bool().Draw(t, "coded leaf") {
			wantCode = rapid.SampledFrom(codes).Draw(t, "leaf code")
			wantMessage = rapid.SampledFrom(messages).Draw(t, "leaf message")
			err = errcode.New(wantCode, wantMessage)
		}

		// Layers apply inside out, so the last coded layer is the outermost.
		for range rapid.IntRange(0, 6).Draw(t, "layers") {
			switch rapid.IntRange(0, 2).Draw(t, "layer") {
			case 0:
				err = errors.Wrap(err)
			case 1:
				err = errors.WrapWithMessage(err, "op")
			default:
				code := rapid.SampledFrom(codes).Draw(t, "code")
				message := rapid.SampledFrom(messages).Draw(t, "message")
				err = errcode.WithCode(code, message, err)
				wantCode = code
				wantMessage = cmp.Or(message, wantMessage)
			}
		}

		if got := errcode.Status(err); got != wantCode {
			t.Fatalf("Status = %v, want %v in %q", got, wantCode, err)
		}
		if got := errcode.Message(err); got != wantMessage {
			t.Fatalf("Message = %q, want %q in %q", got, wantMessage, err)
		}
		if gotCode, gotMessage := errcode.Code(
			err,
		); gotCode != wantCode ||
			gotMessage != wantMessage {
			t.Fatalf("Code = (%v, %q), want (%v, %q)", gotCode, gotMessage, wantCode, wantMessage)
		}
	})
}
