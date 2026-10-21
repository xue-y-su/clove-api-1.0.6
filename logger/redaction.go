package logger

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var sensitiveLogFieldPattern = regexp.MustCompile(`(?i)(^|[\s,;])(signature|body|payload|authorization|token|secret)=("(?:\\.|[^"\\])*"|[^\s,;]*)`)

// sensitiveJSONFieldPattern matches JSON-style "key": "value" pairs for the
// same sensitive field names so that structured/JSON log output is also
// redacted, not just key=value formatted text.
var sensitiveJSONFieldPattern = regexp.MustCompile(`(?i)"(signature|body|payload|authorization|token|secret)":\s*("(?:\\.|[^"\\])*")`)

func redactSensitiveFields(message string) string {
	// Redact key=value format
	message = sensitiveLogFieldPattern.ReplaceAllStringFunc(message, func(match string) string {
		parts := sensitiveLogFieldPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		value := parts[3]
		if strings.HasPrefix(value, `"`) {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		digest := sha256.Sum256([]byte(value))
		return fmt.Sprintf("%s%s=<redacted bytes=%d sha256=%x>", parts[1], parts[2], len([]byte(value)), digest[:8])
	})

	// Redact JSON "key": "value" format
	message = sensitiveJSONFieldPattern.ReplaceAllStringFunc(message, func(match string) string {
		parts := sensitiveJSONFieldPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		value := parts[2]
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		digest := sha256.Sum256([]byte(value))
		return fmt.Sprintf(`"%s": <redacted bytes=%d sha256=%x>`, parts[1], len([]byte(value)), digest[:8])
	})

	return message
}
