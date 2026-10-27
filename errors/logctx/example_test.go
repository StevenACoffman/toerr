package logctx_test

import (
	"context"
	"log/slog"
	"os"

	errors "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/logctx"
)

// The boundary writes one line per request: the fields collected beneath it, plus the
// error's own attributes when the request failed.
func Example() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{} // keep the output stable
			}
			return a
		},
	}))

	ctx := logctx.Init(context.Background())
	err := reserveSeat(ctx, 7)

	level, msg, attrs := slog.LevelInfo, "reserved seat", logctx.Attrs(ctx)
	if err != nil {
		level, msg = slog.LevelError, err.Error()
		attrs = append(attrs, errors.Attrs(err)...)
	}
	logger.LogAttrs(ctx, level, msg, attrs...)
	// Output:
	// level=ERROR msg="charge card: declined" route=/seats seats.cache_hit=true order=7
}

func reserveSeat(ctx context.Context, order int) error {
	logctx.Add(ctx, slog.String("route", "/seats"))
	lookUpSeat(logctx.Scope(ctx, "seats"))
	return errors.WrapWithMessage(errors.New("declined"), "charge card",
		slog.Int("order", order))
}

func lookUpSeat(ctx context.Context) {
	logctx.Add(ctx, slog.Bool("cache_hit", true))
}
