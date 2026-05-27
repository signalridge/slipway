package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationRecordValidateRequiresVerdict(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Blockers:  []ReasonCode{},
		Timestamp: time.Now(),
	}
	err := rec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verdict is required")
}

func TestVerificationRecordValidateRejectsInvalidVerdict(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:   "maybe",
		Blockers:  []ReasonCode{},
		Timestamp: time.Now(),
	}
	err := rec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be")
}

func TestVerificationRecordValidateRequiresBlockers(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:   VerificationVerdictPass,
		Timestamp: time.Now(),
	}
	err := rec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blockers must be present")
}

func TestVerificationRecordValidateRequiresTimestamp(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:  VerificationVerdictPass,
		Blockers: []ReasonCode{},
	}
	err := rec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timestamp is required")
}

func TestVerificationRecordValidatePassesMinimal(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:   VerificationVerdictPass,
		Blockers:  []ReasonCode{},
		Timestamp: time.Now(),
	}
	require.NoError(t, rec.Validate())
}

func TestVerificationRecordIsPassingTrue(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:  VerificationVerdictPass,
		Blockers: []ReasonCode{},
	}
	assert.True(t, rec.IsPassing())
}

func TestVerificationRecordIsPassingFalseWithBlockers(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:  VerificationVerdictPass,
		Blockers: []ReasonCode{NewReasonCode("needs_review", "")},
	}
	assert.False(t, rec.IsPassing())
}

func TestVerificationRecordIsPassingFalseWithFailVerdict(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:  VerificationVerdictFail,
		Blockers: []ReasonCode{},
	}
	assert.False(t, rec.IsPassing())
}

func TestVerificationRecordNormalize(t *testing.T) {
	t.Parallel()
	rec := VerificationRecord{
		Verdict:   VerificationVerdictPass,
		Timestamp: time.Now(),
	}
	rec.Normalize()
	assert.NotNil(t, rec.Blockers)
	assert.NotNil(t, rec.References)
}
