# Mechanism and Domain

*Where error logic lives when you use these packages in your application.*

Ben Johnson's essay *Failure Is Your Domain* makes a claim worth taking seriously:
your errors are part of your domain, as much as your `Customer` and `Order` types,
so your error *type* and error *codes* belong in your application's own root
package — not imported from a third party. A shared errors package, on that view,
is "external to your domain."

This repository is a shared errors package. That is not a contradiction, but it
does require a clear division of labor, and this is the rule the library follows:

> **The library owns the mechanism. Your application owns the meaning.**

## The Split

**Mechanism — what these packages provide.** Generic, reusable, and free of any
knowledge about your domain:

- capturing the call site and building a return trace (`errors.New`, `errors.Wrap`);
- carrying structured context as `slog.Attr` and rendering it for logs;
- a container that holds a status code, a user message, and a cause (`errcode`);
- the three role-based views — application code, end user, operator;
- transport adapters that map a code to a status (`errhttp`).

None of that encodes what *your* failures are. It is plumbing.

**Meaning — what your application owns.** This stays in your root package, in your
domain language:

- which conditions count as errors at all;
- the sentinels that name your specific conditions
  (`var ErrSeatTaken = sentinel.New("seat already taken")`);
- what each `errcode` status *means* for your business, and which of your failures
  map to it;
- where the translation boundaries are — the exact points where an external error
  becomes a domain error;
- the user-facing messages, which are part of your product's voice.

## A Recipe

Declare your domain's conditions in your root package, using the library only as
the mechanism:

```go
package myapp

import (
	errors "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/sentinel"
)

// A domain sentinel — its meaning belongs to myapp, not to the library.
var ErrSeatTaken = sentinel.New("seat already taken")
```

Translate and code at the boundary, in terms your domain defines:

```go
func (s *Store) ReserveSeat(ctx context.Context, id int) error {
	if err := s.insert(ctx, id); err != nil {
		if isUniqueViolation(err) {
			// external error -> domain condition -> transport-neutral code + message
			return errcode.WithCode(errcode.StatusAlreadyExists,
				"that seat is already taken", errors.Wrap(err))
		}
		return errcode.WithCode(errcode.StatusInternal, "", errors.Wrap(err))
	}
	return nil
}
```

Each consumer then reads the view it needs — the application branches on the code,
the user sees the message, the operator sees the trace — exactly as in
[the error-handling guide](philosophy.md).

## On Error Codes: Start Small

`errcode` ships a generic set drawn from HTTP and gRPC. Treat it the way Ben
Johnson treats codes: start with the few you actually need, and expand only when a
caller must branch on a distinction it cannot yet make. If your domain needs a
category the generic set does not capture, do not stretch a status to fit — name it
with a domain sentinel or a `Mark`, which live in your package and mean exactly
what you say they mean.

A fully domain-owned code *type* — a generic `WithCode[C]` parameterized over your
own `Code` type — is also possible, but it earns its keep rarely. It costs a type
parameter at every extraction site (`Status[myapp.Code](err)`, which fails silently
if you name the wrong type) and forfeits the ready-made `errhttp` adapter, since the
library can no longer map a code type it does not know. Reach for a sentinel or a
`Mark` first; prefer a generic coded type only when you have many domain categories
that must all round-trip as one strongly-typed set.

## Why the Library Stays Domain-Agnostic

Ben Johnson's specific objection to an `errors` subpackage is the stutter of
`errors.Error` and the pull of meaning out of the domain. This library sidesteps
the first — the error type is unexported, and you call `errors.New` / `errors.Wrap`,
which read like the standard library — and it avoids the second by refusing to
encode any domain knowledge at all. The generic pieces stay in the library so that
the words that carry meaning — your sentinels, your reading of each code, your
messages — stay in your domain, as the single source of truth.
