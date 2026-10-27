# Errgrpc

**Turn a coded error into a gRPC status and back, without your domain code importing gRPC.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errgrpc.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errgrpc)

[`errcode`](../errcode/) keeps error categories transport-neutral. `errgrpc` is the
adapter at a gRPC boundary, and it maps in both directions. Its codes and errcode's
are the same set apart from `OK`, so the mapping loses nothing.

`errgrpc` is a Go module of its own, so only a program that imports it depends on
gRPC:

```sh
go get github.com/StevenACoffman/toerr/errors/errgrpc
```

## Usage

- `errgrpc.Error(err) error`: for a handler to return. The status carries the
  error's code and its user-facing message (`errcode.Message`), or the gRPC code's
  name when there is none. It never uses `err.Error()`. Without it, gRPC sends
  `err.Error()` itself to the client for any error that is not a bare status. Log
  `err` before you convert it, because the status keeps only the code and message.
- `errgrpc.FromError(err) errcode.StatusCode`: for an error a gRPC client call
  returned. Translate and seal it where it enters your code, so callers branch on
  the domain code and cannot match the gRPC status:

  ```go
  resp, err := client.GetUser(ctx, req)
  if err != nil {
      return nil, errcode.Link(errgrpc.FromError(err), "", errors.Seal(err))
  }
  ```

  A downstream code describes your call to that service, so `FromError` adjusts the
  party responsible, as Google's error model asks of a service that depends on
  others. `InvalidArgument`, `FailedPrecondition`, `OutOfRange`, and `Unimplemented`
  mean your request was wrong, and `Unauthenticated` and `PermissionDenied` that your
  service's credentials were: all become `Internal`. `NotFound`, `AlreadyExists`, and
  the codes that carry retry guidance pass through. The remote message stays in the
  operator view (`Error()` and `%+v`) and out of `errcode.Message`, because the
  remote service wrote it for its own operators.
- `errgrpc.FromCode(code) errcode.StatusCode`: the raw table, the domain code of the
  same name, for a call that forwarded the caller's input unchanged.
- `errgrpc.Code(code) codes.Code`: map an `errcode.StatusCode` to a gRPC code.

A gRPC status already in an error's chain is not passed through by `Error`. It
becomes `Unknown`, unless it was translated with `FromCode` on the way in.

An error without a code that wraps `context.Canceled` or `context.DeadlineExceeded`
becomes `Canceled` or `DeadlineExceeded`, as grpc-go's server answers a handler's
bare context error with `status.FromContextError`. Its message is the code's name,
never `err.Error()`. An attached code wins, and a sealed context error stays
`Unknown`.

## Example

```go
func (s *server) ReserveSeat(ctx context.Context, req *pb.ReserveSeatRequest) (*pb.Seat, error) {
	seat, err := s.store.ReserveSeat(ctx, req.GetId())
	if err != nil {
		s.logger.LogAttrs(ctx, slog.LevelError, "reserve seat failed",
			slog.Any("error", errors.LogValue(err)))
		return nil, errgrpc.Error(err)
	}
	return seat, nil
}
```
