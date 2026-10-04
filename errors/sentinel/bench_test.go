package sentinel_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/StevenACoffman/toerr/errors/sentinel"
)

var benchBool bool

func BenchmarkIs(b *testing.B) {
	errSentinel := sentinel.New("not found")
	err := fmt.Errorf("lookup: %w", errSentinel)
	for b.Loop() {
		benchBool = errors.Is(err, errSentinel)
	}
}
