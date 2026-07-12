//go:build windows

// Package recoverycmd renders argv as an executable recovery command for the
// current operating system.
package recoverycmd

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// Quote quotes one argument for cmd.exe.
func Quote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"\\&|<>()^%!") {
		return value
	}
	return quoteWindowsCommandArgument(value)
}

// Command renders arguments as one command that can be executed as written.
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
	return "powershell.exe -NoLogo -NoProfile -NonInteractive -EncodedCommand " + base64.StdEncoding.EncodeToString(encodedBytes)
}

// quoteWindowsCommandArgument follows the CommandLineToArgvW escaping rules
// while always adding double quotes so cmd.exe cannot interpret metacharacters
// in the argument as command separators.
func quoteWindowsCommandArgument(value string) string {
	var quoted strings.Builder
	quoted.Grow(len(value) + 2)
	quoted.WriteByte('"')
	backslashes := 0
	for _, character := range value {
		switch character {
		case '\\':
			backslashes++
		case '"':
			quoted.WriteString(strings.Repeat("\\", backslashes*2+1))
			quoted.WriteRune(character)
			backslashes = 0
		default:
			quoted.WriteString(strings.Repeat("\\", backslashes))
			quoted.WriteRune(character)
			backslashes = 0
		}
	}
	quoted.WriteString(strings.Repeat("\\", backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}
