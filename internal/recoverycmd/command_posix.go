//go:build !windows

// Package recoverycmd renders argv as an executable recovery command for the
// current operating system.
package recoverycmd

import "strings"

// quote quotes one argument for a POSIX shell.
func quote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n\"'\\$`;&|<>()[]{}*?!#~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Command renders arguments as one command that can be executed as written.
func Command(arguments ...string) string {
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = quote(argument)
	}
	return strings.Join(quoted, " ")
}
