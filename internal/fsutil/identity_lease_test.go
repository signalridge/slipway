package fsutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type identityLeaseCloser func() error

func (close identityLeaseCloser) Close() error {
	return close()
}

func TestTransactionIdentityLeaseAbsorbsAndClosesInReverseOrder(t *testing.T) {
	var order []int
	lease := &transactionIdentityLease{}
	scoped := &transactionIdentityLease{}
	lease.add(identityLeaseCloser(func() error { order = append(order, 1); return nil }))
	scoped.add(identityLeaseCloser(func() error { order = append(order, 2); return nil }))
	scoped.add(identityLeaseCloser(func() error { order = append(order, 3); return nil }))

	lease.absorb(scoped)
	require.NoError(t, scoped.close())
	require.NoError(t, lease.close())
	assert.Equal(t, []int{3, 2, 1}, order)
	require.NoError(t, lease.close(), "a released lease must be idempotent")
}

func TestTransactionIdentityLeaseJoinsCloseErrors(t *testing.T) {
	first := errors.New("first close")
	second := errors.New("second close")
	lease := &transactionIdentityLease{}
	lease.add(identityLeaseCloser(func() error { return first }))
	lease.add(identityLeaseCloser(func() error { return second }))

	err := lease.close()
	require.Error(t, err)
	assert.ErrorIs(t, err, first)
	assert.ErrorIs(t, err, second)
}
