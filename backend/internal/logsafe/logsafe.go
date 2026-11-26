// Package logsafe makes request-derived text safe to put in a log line.
package logsafe

import "strings"

// Text escapes carriage returns and line feeds in s, so a value taken from
// a request (a target name, an error that quotes user input) cannot end the
// log line and forge another one.
func Text(s string) string {
	s = strings.ReplaceAll(s, "\r", `\r`)
	return strings.ReplaceAll(s, "\n", `\n`)
}
