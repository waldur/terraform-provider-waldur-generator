package common

import (
	"fmt"
	"sort"
	"strings"
)

// CollectUniqueStructs gathers all Nested structs that have a AttrTypeRef defined
func CollectUniqueStructs(params ...[]FieldInfo) []FieldInfo {
	seen := make(map[string]bool)
	var result []FieldInfo
	var traverse func([]FieldInfo)

	traverse = func(fields []FieldInfo) {
		for _, f := range fields {
			// Check object type with AttrTypeRef or RefName
			if f.GoType == TFTypeObject {
				key := f.AttrTypeRef
				if key == "" {
					key = f.RefName
				}
				if key != "" {
					if !seen[key] {
						seen[key] = true
						// Ensure AttrTypeRef is set for consistency in result
						if f.AttrTypeRef == "" {
							f.AttrTypeRef = key
						}
						result = append(result, f)
						traverse(f.Properties)
					}
				} else {
					traverse(f.Properties)
				}
			}
			// Check list/set of objects with AttrTypeRef or RefName
			if (f.GoType == TFTypeList || f.GoType == TFTypeSet) && f.ItemSchema != nil {
				key := f.ItemSchema.AttrTypeRef
				if key == "" {
					key = f.ItemSchema.RefName
				}
				if key != "" {
					if !seen[key] {
						seen[key] = true
						// Ensure AttrTypeRef is set
						if f.ItemSchema.AttrTypeRef == "" {
							f.ItemSchema.AttrTypeRef = key
						}
						result = append(result, *f.ItemSchema)
						traverse(f.ItemSchema.Properties)
					}
				} else {
					traverse(f.ItemSchema.Properties)
				}
			}
		}
	}

	for _, p := range params {
		traverse(p)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].AttrTypeRef < result[j].AttrTypeRef })
	return result
}

// AssignMissingAttrTypeRefs recursively assigns a AttrTypeRef to objects/lists of objects that lack one.
func AssignMissingAttrTypeRefs(cfg SchemaConfig, fields []FieldInfo, prefix string, seenHashes map[string]string, seenNames map[string]string) {
	for i := range fields {
		f := &fields[i]

		// Recursively process children first (Bottom-Up)
		if f.GoType == TFTypeObject {
			AssignMissingAttrTypeRefs(cfg, f.Properties, prefix+ToTitle(f.Name), seenHashes, seenNames)
		} else if (f.GoType == TFTypeList || f.GoType == TFTypeSet) && f.ItemSchema != nil {
			if f.ItemSchema.GoType == TFTypeObject {
				AssignMissingAttrTypeRefs(cfg, f.ItemSchema.Properties, prefix+ToTitle(f.Name), seenHashes, seenNames)

				// Also assign ref to ItemSchema itself
				hash := computeStructHash(*f.ItemSchema)
				if name, ok := seenHashes[hash]; ok {
					f.ItemSchema.AttrTypeRef = name
				} else {
					candidate := f.ItemSchema.RefName
					if candidate == "" {
						candidate = prefix + ToTitle(f.Name)
					}
					finalName := resolveUniqueName(candidate, hash, seenNames)
					seenHashes[hash] = finalName
					seenNames[finalName] = hash
					f.ItemSchema.AttrTypeRef = finalName
				}
			}
		}

		// Now process f itself if it is Object
		if f.GoType == TFTypeObject {
			hash := computeStructHash(*f)
			if name, ok := seenHashes[hash]; ok {
				f.AttrTypeRef = name
			} else {
				candidate := f.RefName
				if candidate == "" {
					candidate = prefix + ToTitle(f.Name)
				}
				finalName := resolveUniqueName(candidate, hash, seenNames)
				seenHashes[hash] = finalName
				seenNames[finalName] = hash
				f.AttrTypeRef = finalName
			}
		}
	}
}

func resolveUniqueName(candidate string, hash string, seenNames map[string]string) string {
	finalName := candidate
	counter := 2
	for {
		if oldHash, exists := seenNames[finalName]; exists {
			if oldHash == hash {
				return finalName
			}
			finalName = fmt.Sprintf("%s%d", candidate, counter)
			counter++
		} else {
			return finalName
		}
	}
}

func computeStructHash(f FieldInfo) string {
	var parts []string
	for _, p := range f.Properties {
		key := fmt.Sprintf("%s:%s:%s", p.Name, p.GoType, p.AttrTypeRef)
		parts = append(parts, key)
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}
