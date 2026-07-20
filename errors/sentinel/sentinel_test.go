package sentinel_test

import (
	"errors"
	"testing"

	"github.com/StevenACoffman/toerr/errors/sentinel"
)

func TestMatchesByIdentityNotText(t *testing.T) {
	errNotFound := sentinel.New("not found")

	if !errors.Is(errNotFound, errNotFound) {
		t.Error("a sentinel should match itself")
	}
	// A distinct value with the same text must not match.
	if errors.Is(errNotFound, sentinel.New("not found")) {
		t.Error("distinct sentinels with the same text should not match")
	}
}

func TestMatchableThroughWrapping(t *testing.T) {
	errNotFound := sentinel.New("not found")
	wrapped := errors.Join(errors.New("context"), errNotFound)
	if !errors.Is(wrapped, errNotFound) {
		t.Error("a wrapped sentinel should still match via errors.Is")
	}
}

func TestIsSentinelMarker(t *testing.T) {
	var s interface{ IsSentinel() bool }
	if !errors.As(sentinel.New("x"), &s) || !s.IsSentinel() {
		t.Error("sentinel.New should produce a value reporting IsSentinel() == true")
	}
}
