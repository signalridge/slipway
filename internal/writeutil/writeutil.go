package writeutil

import (
	"fmt"
	"io"
)

// BestEffortf is for non-critical diagnostic output where write failures
// should not change control flow.
func BestEffortf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
