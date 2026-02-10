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

// ToTitle converts a string to title case for use in templates
func ToTitle(s string) string {
	// Convert snake_case to TitleCase
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// Humanize converts snake_case to Title Case with spaces
func Humanize(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// DisplayName strips module prefix and converts to title case
func DisplayName(s string) string {
	name := s
	if idx := strings.Index(s, "_"); idx != -1 {
		name = s[idx+1:]
	}
	return ToTitle(name)
}
