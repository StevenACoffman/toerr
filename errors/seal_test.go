package errors_test

import (
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
)

func sealOrigin() error { return errors.New("db down", slog.Int("shard", 7)) }

// Each case asks one matching question that reaches the unsealed chain. A seal must
// turn every answer to no.
func TestSealBlocksMatching(t *testing.T) {
	cause := rateLimitError{after: 1}
	chain := errors.Mark(errors.WrapWithMessage(cause, "query"), errRateLimited)

	cases := map[string]func(error) bool{
		"Is a marker":  func(err error) bool { return errors.Is(err, errRateLimited) },
		"Is the cause": func(err error) bool { return errors.Is(err, cause) },
		"As the cause's type": func(err error) bool {
			var target rateLimitError
			return errors.As(err, &target)
		},
		"AsType": func(err error) bool {
			_, ok := errors.AsType[rateLimitError](err)
			return ok
		},
		"AsBehavior": func(err error) bool {
			_, ok := errors.AsBehavior[retryable](err)
			return ok
		},
	}
	for name, reaches := range cases {
		t.Run(name, func(t *testing.T) {
			assert(t, reaches(chain), "the unsealed chain should match")
			assert(t, !reaches(errors.Seal(chain)), "the sealed chain should not match")
			assert(t, !reaches(errors.Wrap(errors.Seal(chain))), "a wrap must not unseal")
		})
	}
	equals(t, nil, errors.Unwrap(errors.Seal(chain)))
}

func TestSealKeepsTheOperatorView(t *testing.T) {
	inner := errors.WrapWithMessage(sealOrigin(), "load", slog.String("op", "load"))
	err := errors.Wrap(errors.Seal(inner))

	equals(t, inner.Error(), err.Error())
	equals(t, fmt.Sprint(errors.Attrs(inner)), fmt.Sprint(errors.Attrs(err)))

	out := fmt.Sprintf("%+v", err)
	for _, want := range []string{"load: db down", "sealOrigin", "TestSealKeepsTheOperatorView"} {
		assert(t, strings.Contains(out, want),
			fmt.Sprintf("%%+v should show %q beneath the seal:\n%s", want, out))
	}

	// A seal returned as it is formats itself the same way.
	sealed := errors.Seal(inner)
	equals(t, inner.Error(), fmt.Sprintf("%v", sealed))
	out = fmt.Sprintf("%+v", sealed)
	assert(t, strings.Contains(out, "sealOrigin"),
		fmt.Sprintf("%%+v of a bare seal should show the trace beneath it:\n%s", out))
}

func TestSealKeepsJoinedBranches(t *testing.T) {
	err := errors.Wrap(errors.Seal(errors.Join(sealOrigin(), errors.New("cache miss"))))

	out := fmt.Sprintf("%+v", err)
	for _, want := range []string{"db down", "cache miss", "+-", "sealOrigin"} {
		assert(t, strings.Contains(out, want),
			fmt.Sprintf("%%+v should render the sealed join, missing %q:\n%s", want, out))
	}
	equals(t, 1, len(errors.Attrs(err)))
}

func TestSealNil(t *testing.T) {
	equals(t, nil, errors.Seal(nil))
}

// Mark outside a seal is how a dependency's error becomes a domain sentinel: callers
// match the mark and nothing beneath it. A seal is not a hop, so Mark gives the
// foreign error a frame here.
func TestMarkOutsideASeal(t *testing.T) {
	dependency := stderrors.New("sql: no rows in result set")
	err := errors.Mark(errors.Seal(dependency), errRateLimited)

	assert(t, errors.Is(err, errRateLimited), "the mark outside the seal should match")
	assert(t, !errors.Is(err, dependency), "the dependency inside the seal should not")
	assert(t, strings.Contains(fmt.Sprintf("%+v", err), "TestMarkOutsideASeal"),
		"Mark should record a frame for the sealed foreign error")
}
