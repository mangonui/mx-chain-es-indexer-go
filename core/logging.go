package core

import "unicode/utf8"

const maxSanitizedLogValueLength = 1024

// SanitizeLogValue flattens control characters and bounds external strings
// before they are emitted into structured logs.
func SanitizeLogValue(value string) string {
	if value == "" {
		return ""
	}

	out := make([]rune, 0, len(value))
	for _, ch := range value {
		if ch == '\n' || ch == '\r' || ch == '\t' {
			ch = ' '
		} else if ch < 0x20 || ch == utf8.RuneError {
			ch = '?'
		}

		out = append(out, ch)
		if len(out) >= maxSanitizedLogValueLength {
			return string(out) + "..."
		}
	}

	return string(out)
}

// SanitizeLogError returns a bounded, single-line error message.
func SanitizeLogError(err error) string {
	if err == nil {
		return ""
	}

	return SanitizeLogValue(err.Error())
}
