package errcontext

import (
	"log/slog"

	"github.com/StevenACoffman/toerr/errors/xerrors"
	"github.com/sirupsen/logrus"
)

type ErrAttr []slog.Attr

func (e ErrAttr) Attrs() []slog.Attr {
	return e
}

func (e ErrAttr) AttrsAny() []any {
	anys := make([]any, 0, len(e))
	for _, attr := range e {
		anys = append(anys, attr)
	}
	return anys
}

// Add wraps the given error with log attributes for greater context.
// If the error already has errAttr, the new errAttr is appended to the existing errAttr
// and the error wrapped again with the new errAttr.
func Add(err error, errAttr ...slog.Attr) error {
	if err == nil {
		return nil
	}

	var newContext ErrAttr

	if oldContext := Get(err); oldContext != nil {
		newContext = append(oldContext, errAttr...)
	} else {
		newContext = append(make(ErrAttr, 0, len(errAttr)), errAttr...)
	}
	return xerrors.Extend(newContext, err)
}

// Get returns the context of the given error.
// Only the newest context is returned.
func Get(err error) ErrAttr {
	if err == nil {
		return nil
	}

	if context, ok := xerrors.Extract[ErrAttr](err); ok {
		return context
	}
	return nil
}

func WithError(logger *slog.Logger, err error) *slog.Logger {
	if err == nil {
		return logger
	}
	if oldContext := Get(err); oldContext != nil {
		// Do we log key "error" with unwrapped error?
		// Short err.Error()?
		// Verbose fmt.Sprintf("%+v", err)
		return logger.
			With(oldContext.AttrsAny()...).
			With("error", oldContext) // err.Error() ?
	}

	return logger
}

func FieldsToAttrs(fields logrus.Fields) []slog.Attr {
	// hack to type cast look nice
	fieldsToMap := func(fields logrus.Fields) map[string]any {
		return fields
	}

	attrs := make([]slog.Attr, 0, len(fields))
	for k, v := range fieldsToMap(fields) {
		attrs = append(attrs, slog.Any(k, v))
	}

	return attrs
}
