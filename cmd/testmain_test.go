package cmd

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	original, hadOriginal := os.LookupEnv(heuristicFallbackEnv)
	_ = os.Setenv(heuristicFallbackEnv, "1")

	code := m.Run()

	if hadOriginal {
		_ = os.Setenv(heuristicFallbackEnv, original)
	} else {
		_ = os.Unsetenv(heuristicFallbackEnv)
	}
	os.Exit(code)
}
