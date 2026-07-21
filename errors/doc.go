// Package errors is a drop-in replacement for the standard library errors package
// that records where each error was created and wrapped.
//
// fmt.Errorf with %w records what failed, not where. New and Wrap capture the
// file, line, and function at each call, forming a return trace printed under %+v.
// Constructors also accept trailing slog.Attr values for structured logging, and
// Mark/AsType tag an error by type for control flow. Is, As, Unwrap, and Join are
// re-exported from the standard library unchanged.
package errors
