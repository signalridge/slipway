package cmd

import (
	"fmt"
	"io"
)

type formatWriter struct {
	w   io.Writer
	err error
}

func newFormatWriter(w io.Writer) *formatWriter {
	return &formatWriter{w: w}
}

func (w *formatWriter) Writef(format string, args ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.w, format, args...)
}

func (w *formatWriter) Writeln(args ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(w.w, args...)
}

func (w *formatWriter) Err() error {
	return w.err
}
