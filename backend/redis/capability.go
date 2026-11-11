package redis

import (
	"context"
	"errors"
	"strings"
)

// CapabilityErrorCode is for reads/probes, not write outcome classification.
func CapabilityErrorCode(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToUpper(err.Error())
	switch {
	case strings.Contains(text, "NOPERM"), strings.Contains(text, "NOAUTH"), strings.Contains(text, "WRONGPASS"):
		return "permission_denied"
	case strings.Contains(text, "DISABLED"), strings.Contains(text, "NOT ALLOWED"):
		return "command_disabled"
	case isUnknownCommandError(err):
		return "unsupported_or_renamed"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "read_failed"
	}
}
