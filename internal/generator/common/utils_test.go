package common

import "testing"

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple string", "simple string"},
		{"string with \"quotes\"", "string with \\\"quotes\\\""},
		{"string with \\backslashes\\", "string with \\\\backslashes\\\\"},
		{"string with\nnewlines", "string with newlines"},
		{"string with\r\nwindows newlines", "string with windows newlines"},
		{"string with\ttabs", "string with tabs"},
		{"multiple  spaces", "multiple spaces"},
		{"  trimmed spaces  ", "trimmed spaces"},
	}

	for _, tt := range tests {
		result := SanitizeString(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeString(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSplitResourceName(t *testing.T) {
	tests := []struct {
		input        string
		expectedServ string
		expectedName string
	}{
		{"openstack_instance", "openstack", "instance"},
		{"marketplace_order", "marketplace", "order"},
		{"no_prefix", "no", "prefix"},
		{"single", "core", "single"},
	}

	for _, tt := range tests {
		serv, name := SplitResourceName(tt.input)
		if serv != tt.expectedServ || name != tt.expectedName {
			t.Errorf("SplitResourceName(%q) = (%q, %q), expected (%q, %q)", tt.input, serv, name, tt.expectedServ, tt.expectedName)
		}
	}
}

func TestToTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"snake_case", "SnakeCase"},
		{"multiple_word_snake_case", "MultipleWordSnakeCase"},
		{"alreadyTitle", "AlreadyTitle"},
		{"", ""},
	}

	for _, tt := range tests {
		result := ToTitle(tt.input)
		if result != tt.expected {
			t.Errorf("ToTitle(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestHumanize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"snake_case", "Snake Case"},
		{"multiple_word_snake_case", "Multiple Word Snake Case"},
		{"alreadyTitle", "AlreadyTitle"},
		{"", ""},
	}

	for _, tt := range tests {
		result := Humanize(tt.input)
		if result != tt.expected {
			t.Errorf("Humanize(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openstack_instance", "Instance"},
		{"marketplace_order", "Order"},
		{"no_prefix", "Prefix"},
		{"single", "Single"},
	}

	for _, tt := range tests {
		result := DisplayName(tt.input)
		if result != tt.expected {
			t.Errorf("DisplayName(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
