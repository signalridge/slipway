package cmd

import "github.com/signalridge/slipway/internal/model"

func appendReasonCodes(existing []model.ReasonCode, groups ...[]model.ReasonCode) []model.ReasonCode {
	if len(groups) == 0 {
		return model.NormalizeReasonCodes(existing)
	}
	merged := append([]model.ReasonCode(nil), existing...)
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return model.NormalizeReasonCodes(merged)
}
