package cmd

import (
	"github.com/signalridge/slipway/internal/model"
)

type changeProfileView struct {
	QualityMode     string
	NeedsDiscovery  bool
	ComplexityLevel string
	GuardrailDomain string
}

func buildChangeProfileView(change model.Change) changeProfileView {
	return changeProfileView{
		QualityMode:     string(change.EffectiveQualityMode()),
		NeedsDiscovery:  change.NeedsDiscovery,
		ComplexityLevel: change.ComplexityLevel,
		GuardrailDomain: change.GuardrailDomain,
	}
}
