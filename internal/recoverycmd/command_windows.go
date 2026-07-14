//go:build windows

// Package recoverycmd renders argv as an executable recovery command for the
// current operating system.
package recoverycmd

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// Command renders arguments as one command that can be executed as written.
// It only renders text; when argv[0] is unqualified, users should invoke the
// displayed command from a trusted location where it resolves as intended.
// The encoded PowerShell launcher keeps user values out of cmd.exe parsing and
// assigns a command line composed with the native CommandLineToArgvW rules
// directly to ProcessStartInfo, bypassing PowerShell's lossy native argv binder.
func Command(arguments ...string) string {
	if len(arguments) == 0 {
		return ""
	}
	script := "$startInfo=New-Object System.Diagnostics.ProcessStartInfo;" +
		"$startInfo.UseShellExecute=$false;" +
		"$startInfo.FileName=" + powerShellUTF8Expression(arguments[0]) + ";" +
		"$startInfo.Arguments=" + powerShellUTF8Expression(composeWindowsArguments(arguments[1:])) + ";" +
		"$process=[System.Diagnostics.Process]::Start($startInfo);" +
		"if ($null -eq $process) { exit 1 };" +
		"$process.WaitForExit();" +
		"exit $process.ExitCode"
	return encodedPowerShellCommand(script)
}

func composeWindowsArguments(arguments []string) string {
	escaped := make([]string, len(arguments))
	for index, argument := range arguments {
		escaped[index] = windows.EscapeArg(argument)
	}
	return strings.Join(escaped, " ")
}

func powerShellUTF8Expression(value string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	return "[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + encoded + "'))"
}

func encodedPowerShellCommand(script string) string {
	codeUnits := utf16.Encode([]rune(script))
	encodedBytes := make([]byte, len(codeUnits)*2)
	for index, unit := range codeUnits {
		encodedBytes[index*2] = byte(unit)
		encodedBytes[index*2+1] = byte(unit >> 8)
	}
	return windows.EscapeArg(systemPowerShellPath()) + " -NoLogo -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(encodedBytes)
}

func systemPowerShellPath() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" || !filepath.IsAbs(systemRoot) {
		systemRoot = `C:\Windows`
	}
	return filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}
