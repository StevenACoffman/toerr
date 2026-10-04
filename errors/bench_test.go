package errors_test

import (
	stderrors "errors"
	"fmt"
	"log/slog"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
)

// Sinks keep the compiler from discarding the work under measurement.
var (
	errSink    error
	benchBool  bool
	benchAttrs []slog.Attr
	benchStr   string
)

type benchMarkerError struct{}

// BenchmarkNew and BenchmarkWrap measure the price every call site pays: the allocation
// plus the runtime.Callers walk that records the frame. It is paid even for an error the
// caller goes on to swallow, which is the cost TODO item 4 asks to put a number on.
func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		errSink = errors.New("boom")
	}
}

func BenchmarkNewWithAttrs(b *testing.B) {
	for b.Loop() {
		errSink = errors.New("boom", slog.String("op", "read"), slog.Int("id", 7))
	}
}

func BenchmarkWrap(b *testing.B) {
	leaf := errors.New("boom")
	for b.Loop() {
		errSink = errors.Wrap(leaf)
	}
}

// BenchmarkFmtErrorf is the baseline Wrap replaces, for comparison in the same run.
func BenchmarkFmtErrorf(b *testing.B) {
	leaf := stderrors.New("boom")
	for b.Loop() {
		errSink = fmt.Errorf("context: %w", leaf)
	}
}

func BenchmarkWrapWithMessage(b *testing.B) {
	leaf := errors.New("boom")
	for b.Loop() {
		errSink = errors.WrapWithMessage(leaf, "context")
	}
}

func BenchmarkMark(b *testing.B) {
	b.Run("foreign", func(b *testing.B) {
		cause, marker := stderrors.New("boom"), stderrors.New("marker")
		for b.Loop() {
			errSink = errors.Mark(cause, marker)
		}
	})
	b.Run("traced", func(b *testing.B) {
		cause, marker := errors.New("boom"), stderrors.New("marker")
		for b.Loop() {
			errSink = errors.Mark(cause, marker)
		}
	})
}

// deepChain is a chain of depth wraps over a leaf, each wrap carrying one attribute,
// with a sentinel mark halfway up.
func deepChain(depth int) (err, marker error) {
	marker = stderrors.New("marker")
	err = errors.New("leaf", slog.Int("depth", 0))
	for i := 1; i <= depth; i++ {
		err = errors.Wrap(err, slog.Int("depth", i))
		if i == depth/2 {
			err = errors.Mark(err, marker)
		}
	}
	return err, marker
}

// The readers walk the whole chain, so they are measured at a depth a real service reaches.
func BenchmarkIsMarker(b *testing.B) {
	err, marker := deepChain(10)
	for b.Loop() {
		benchBool = errors.Is(err, marker)
	}
}

func (*benchMarkerError) Error() string { return "marker" }

func BenchmarkAsType(b *testing.B) {
	err := errors.Wrap(errors.Mark(errors.Wrap(errors.New("leaf")), &benchMarkerError{}))
	for b.Loop() {
		_, benchBool = errors.AsType[*benchMarkerError](err)
	}
}

func BenchmarkAsBehavior(b *testing.B) {
	err, _ := deepChain(10)
	for b.Loop() {
		_, benchBool = errors.AsBehavior[interface{ Retryable() bool }](err)
	}
}

func BenchmarkAttrs(b *testing.B) {
	b.Run("chain", func(b *testing.B) {
		err, _ := deepChain(10)
		for b.Loop() {
			benchAttrs = errors.Attrs(err)
		}
	})
	b.Run("join", func(b *testing.B) {
		left, _ := deepChain(5)
		right, _ := deepChain(5)
		err := errors.Wrap(errors.Join(left, right))
		for b.Loop() {
			benchAttrs = errors.Attrs(err)
		}
	})
}

func BenchmarkLogValue(b *testing.B) {
	err, _ := deepChain(10)
	for b.Loop() {
		benchAttrs = errors.LogValue(err).Group()
	}
}

// BenchmarkFormatTrace measures %+v, which resolves every recorded PC to a frame. It runs
// once per logged error rather than once per hop, but it is the most expensive thing the
// package does.
func BenchmarkFormatTrace(b *testing.B) {
	b.Run("chain", func(b *testing.B) {
		err, _ := deepChain(10)
		for b.Loop() {
			benchStr = fmt.Sprintf("%+v", err)
		}
	})
	b.Run("join", func(b *testing.B) {
		left, _ := deepChain(5)
		right, _ := deepChain(5)
		err := errors.Wrap(errors.Join(left, right))
		for b.Loop() {
			benchStr = fmt.Sprintf("%+v", err)
		}
	})
}

func BenchmarkRecover(b *testing.B) {
	for b.Loop() {
		errSink = errors.Recover("boom")
	}
}
