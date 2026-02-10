package common

import "strings"

// SanitizeString replaces problematic characters in descriptions
func SanitizeString(s string) string {
	// Replace problematic characters in descriptions
	s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslashes first
	s = strings.ReplaceAll(s, "\"", "\\\"") // Escape quotes
	s = strings.ReplaceAll(s, "\n", " ")    // Replace newlines with spaces
	s = strings.ReplaceAll(s, "\r", "")     // Remove carriage returns
	s = strings.ReplaceAll(s, "\t", " ")    // Replace tabs with spaces
	// Normalize multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// SplitResourceName splits a resource name into service and clean name
func SplitResourceName(name string) (string, string) {
	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "core", name // Fallback to core
}
