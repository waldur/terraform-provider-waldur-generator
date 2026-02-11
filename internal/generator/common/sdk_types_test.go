package common

import "testing"

func TestCalculateSDKType(t *testing.T) {
	tests := []struct {
		name      string
		field     FieldInfo
		expected  string
		isPointer bool
	}{
		{
			name: "required string",
			field: FieldInfo{
				Type:     OpenAPITypeString,
				Required: true,
			},
			expected:  GoTypeString,
			isPointer: true, // Special case: strings are almost always pointers
		},
		{
			name: "optional integer",
			field: FieldInfo{
				Type:     OpenAPITypeInteger,
				Required: false,
			},
			expected:  GoTypeInt64,
			isPointer: true,
		},
		{
			name: "list of strings",
			field: FieldInfo{
				Type:     OpenAPITypeArray,
				ItemType: OpenAPITypeString,
				Required: true,
			},
			expected:  "[]string",
			isPointer: false,
		},
		{
			name: "map of numbers",
			field: FieldInfo{
				Type:     OpenAPITypeObject,
				GoType:   TFTypeMap,
				ItemType: OpenAPITypeNumber,
			},
			expected:  "map[string]float64",
			isPointer: false,
		},
		{
			name: "object with ref",
			field: FieldInfo{
				Type:    OpenAPITypeObject,
				RefName: "MyObject",
			},
			expected:  "MyObject",
			isPointer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := tt.field
			CalculateSDKType(&field)
			if field.SDKType != tt.expected {
				t.Errorf("SDKType = %q, expected %q", field.SDKType, tt.expected)
			}
			if field.IsPointer != tt.isPointer {
				t.Errorf("IsPointer = %v, expected %v", field.IsPointer, tt.isPointer)
			}
		})
	}
}
