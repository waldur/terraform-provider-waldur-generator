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
