package cmd

import (
	"encoding/json"
	"io"

	"github.com/signalridge/slipway/internal/autopilot"
)

const machineContractVersion = autopilot.ContractVersion

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
