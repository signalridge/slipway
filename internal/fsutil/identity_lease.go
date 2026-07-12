package fsutil

import (
	"errors"
	"fmt"
	"io"
)

// transactionIdentityLease owns every handle that pins an identity trusted by
// a file transaction. Handles are released together, in reverse acquisition
// order, after the transaction has stopped making identity-based decisions.
type transactionIdentityLease struct {
	closers []io.Closer
}

func (lease *transactionIdentityLease) add(closer io.Closer) {
	if lease == nil || closer == nil {
		return
	}
	lease.closers = append(lease.closers, closer)
}

func (lease *transactionIdentityLease) absorb(scoped *transactionIdentityLease) {
	if lease == nil || scoped == nil || len(scoped.closers) == 0 {
		return
	}
	lease.closers = append(lease.closers, scoped.closers...)
	scoped.closers = nil
}

func (lease *transactionIdentityLease) close() error {
	if lease == nil {
		return nil
	}
	var errs []error
	for index := len(lease.closers) - 1; index >= 0; index-- {
		if err := lease.closers[index].Close(); err != nil {
			errs = append(errs, fmt.Errorf("close transaction identity handle: %w", err))
		}
	}
	lease.closers = nil
	return errors.Join(errs...)
}
