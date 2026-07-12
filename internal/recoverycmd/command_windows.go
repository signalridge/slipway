//go:build windows

// Package recoverycmd renders argv as an executable recovery command for the
// current operating system.
package recoverycmd

import (
	"encoding/base64"
	"strings"
	"unicode/utf16"
)

// Quote quotes one argument for cmd.exe.
func Quote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"\\&|<>()^%!") {
		return value
	}
	return quoteWindowsCommandArgument(value)
}

// Command renders arguments as one command that can be executed as written.
// Percent and exclamation marks require encoded PowerShell because cmd.exe
// expands them even inside double quotes.
func Command(arguments ...string) string {
	unsafeForCommandPrompt := false
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = Quote(argument)
		unsafeForCommandPrompt = unsafeForCommandPrompt || strings.ContainsAny(argument, "%!")
	}
	if !unsafeForCommandPrompt {
		return strings.Join(quoted, " ")
	}

	powershell := make([]string, len(arguments))
	for index, argument := range arguments {
		powershell[index] = "'" + strings.ReplaceAll(argument, "'", "''") + "'"
	}
	script := "& " + strings.Join(powershell, " ")
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
