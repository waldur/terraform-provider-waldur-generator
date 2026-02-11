package common

// MergeFields combines two lists of fields, deduplicating by name.
// Fields from the first list take precedence for shared properties,
// but ReadOnly status is taken from either.
func MergeFields(primary, secondary []FieldInfo) []FieldInfo {
	fieldIdx := make(map[string]int)
	var merged []FieldInfo

	// Add primary fields first
	for _, f := range primary {
		fieldIdx[f.Name] = len(merged)
		merged = append(merged, f)
	}

	// Add secondary fields if not present
	for _, f := range secondary {
		if idx, ok := fieldIdx[f.Name]; ok {
			existing := merged[idx]
			// Preserve IsPathParam from primary - path params should keep their Required state
			if existing.IsPathParam {
				existing.ReadOnly = false // Path params are always writable
			} else if f.ReadOnly {
				existing.ReadOnly = true
			}
			if f.ServerComputed {
				existing.ServerComputed = true
			}

			// Recursively merge nested properties if present in both
			if len(existing.Properties) > 0 && len(f.Properties) > 0 {
				existing.Properties = MergeFields(existing.Properties, f.Properties)
			}
			if existing.ItemSchema != nil && f.ItemSchema != nil && len(existing.ItemSchema.Properties) > 0 && len(f.ItemSchema.Properties) > 0 {
				existing.ItemSchema.Properties = MergeFields(existing.ItemSchema.Properties, f.ItemSchema.Properties)
			}

			// Update in slice
			merged[idx] = existing
		} else {
			fieldIdx[f.Name] = len(merged)
			merged = append(merged, f)
		}
	}

	return merged
}

// MergeOrderFields combines offering (input) and resource (output) fields for Order resources.
// Input fields take precedence and determine writability.
// Output fields not in input are marked as ReadOnly (Computed).
// Nested objects/lists are merged recursively.
func MergeOrderFields(input, output []FieldInfo) []FieldInfo {
	merged := mergeOrderedFieldsRecursive(input, output)

	// Map for quick lookup
	fieldMap := make(map[string]int)
	for i, f := range merged {
		fieldMap[f.Name] = i
	}

	// Ensure project and offering fields exist and are configured correctly
	ensureOrderField(fieldMap, &merged, "project", "Project URL")
	ensureOrderField(fieldMap, &merged, "offering", "Offering URL")

	return merged
}

func ensureOrderField(fieldMap map[string]int, merged *[]FieldInfo, name, description string) {
	if idx, ok := fieldMap[name]; ok {
		(*merged)[idx].Required = true
		(*merged)[idx].ReadOnly = false
	} else {
		*merged = append(*merged, FieldInfo{
			Name:        name,
			Type:        OpenAPITypeString,
			Required:    true,
			ReadOnly:    false,
			Description: description,
			GoType:      TFTypeString,
			SDKType:     GoTypeString,
			IsPointer:   true,
		})
		fieldMap[name] = len(*merged) - 1
	}
}

func mergeOrderedFieldsRecursive(input, output []FieldInfo) []FieldInfo {
	fieldIdx := make(map[string]int)
	var merged []FieldInfo

	// Add input fields first
	for _, f := range input {
		merged = append(merged, f)
		fieldIdx[f.Name] = len(merged) - 1
	}

	// Add output fields if not present in input, or merge if present
	for _, f := range output {
		if idx, ok := fieldIdx[f.Name]; ok {
			// Merge nested properties
			existing := merged[idx]
			updated := false

			// If it appears in both input and output, it's server-computed
			// BUT if it's required in input, it should stay required (not computed+optional)
			if !existing.Required { // Check the 'input' field's Required status
				existing.ServerComputed = true
				updated = true
			}

			// Update description if output has one and input doesn't
			if existing.Description == "" && f.Description != "" {
				existing.Description = f.Description
				updated = true
			}

			// Merge nested lists of objects
			if existing.ItemType == OpenAPITypeObject && f.ItemType == OpenAPITypeObject && existing.ItemSchema != nil && f.ItemSchema != nil {
				existing.ItemSchema.Properties = mergeOrderedFieldsRecursive(existing.ItemSchema.Properties, f.ItemSchema.Properties)
				updated = true
			} else if existing.GoType == TFTypeObject && f.GoType == TFTypeObject {
				// Merge nested objects
				existing.Properties = mergeOrderedFieldsRecursive(existing.Properties, f.Properties)
				updated = true
			}

			if updated {
				merged[idx] = existing
			}
		} else {
			// Output-only fields are ReadOnly (Computed)
			f.ReadOnly = true
			f.Required = false
			merged = append(merged, f)
			fieldIdx[f.Name] = len(merged) - 1
		}
	}

	return merged
}
