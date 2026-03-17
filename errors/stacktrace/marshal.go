package stacktrace

import "strconv"

const (
	stackSourceFileName     = "source"
	stackSourceLineName     = "line"
	stackSourceFunctionName = "func"
)

// StackTraceMarshaler nicely formats stacktraces for logging.
func StackTraceMarshaler(err error) any {
	trace := Extract(err)
	if trace == nil {
		return nil
	}

	out := make([]map[string]string, 0, len(trace))
	for _, frame := range trace {
		out = append(out, map[string]string{
			stackSourceFileName:     frame.File,
			stackSourceLineName:     strconv.Itoa(frame.LineNumber),
			stackSourceFunctionName: frame.Function,
		})
	}
	return out
}
