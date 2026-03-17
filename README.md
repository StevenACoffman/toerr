# Errors

Go errors library.

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors)

## Features

- [Message](#message)
- [Stack trace](#stack-trace)
- [Verbose message](#verbose-message)
- [Drop-in replacement of the std `errors` package](#migrate-from-the-std-errors-package)
- [Easy to extend](#extend)

## Message

[`New()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#New) creates an error with a message.

[`Wrap()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Wrap) adds a message to an error.

```go
err := errors.New("error")
err = errors.Wrap(err, "message")
fmt.Println(err) // "message: error"
```

## Stack trace

Errors created by [`New()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#New) and wrapped by [`Wrap()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Wrap) have a stack trace.

The error [verbose message](#verbose-message) includes the stack trace.

[`errstack.Frames()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errstack#Frames) returns the [stack frames](https://pkg.go.dev/runtime#Frames) of the error.

```go
frames := errors.StackFrames(err)
```

It's compatible with [Sentry](https://pkg.go.dev/github.com/getsentry/sentry-go).

## Verbose message

The error verbose message shows additional information about the error.
Wrapping functions may provide a verbose message (stack, tag, value, etc.)

The [`Write()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errverbose#Write)/[`String()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errverbose#String)/[`Formatter()`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errverbose#Formatter) functions write/return/format the error verbose message.

The first line is the error's message.
The following lines are the verbose message of the error chain.

Example:

```text
test: error
value c = d
tag a = b
temporary = true
ignored
stack
    github.com/StevenACoffman/toerr/errors/integration_test_test.Test integration_test.go:17
    testing.tRunner testing.go:1576
    runtime.goexit asm_amd64.s:1598
```

## Extend

Create a custom error type:

- Create a type implementing the [`error`](https://pkg.go.dev/builtin#error) interface
- Optionally implement the [`Unwrap() error`](https://pkg.go.dev/errors#Unwrap) method
- Optionally implement the [`errverbose.Interface`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errverbose#Interface) interface

See the provided packages as example:

- [`errbase`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errbase): create a base error (e.g. sentinel error)
- [`errmsg`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errmsg): add a message to an error
- [`errstack`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errstack): add a stack trace to an error
- [`errtag`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errtag): add a tag to an error
- [`errval`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errval): add a value to an error
- [`errignore`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errignore): mark an error as ignored
- [`errtmp`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errtmp): mark an error as temporary

## Migrate from the std `errors` package

- Replace the import `errors` with `github.com/StevenACoffman/toerr/errors`
- Replace `fmt.Errorf("some wessage: %w", err)` with `errors.Wrap(err, "some message")`
- Use `errbase.New()` for sentinel error

## Prior Art
https://github.com/Danlock/pkg/blob/main/errors/attr.go

[xerrors](https://github.com/zircuit-labs/zkr-go-common-public/blob/dc1effe2259f5592f9c38fcc4079aeca0f555cd9/xerrors/errcontext/errcontext.go#L14) Comes from @alif-zrc
```
func Add(err error, context ...slog.Attr) error {
```
[AnnotatedError](https://github.com/myrjola/sheerluck/blob/ba6715f2118eba0677889afb58d77f6f3f33f345/internal/errors/annotatederror.go#L24
) comes from @myrjola
```
// AnnotatedError includes more context than a plain error that is useful for troubleshooting.
type AnnotatedError struct {
	// msg is the error message.
	msg string
	// pc is the program counter for the location of the error provided by runtime.Callers.
	pc uintptr
	// attrs are slog attributes that are added to the log event to provide more context for the error.
	attrs []slog.Attr
}

// New creates a new [AnnotatedError] with the given message and attributes.
func New(msg string, attrs ...slog.Attr) AnnotatedError {
```
