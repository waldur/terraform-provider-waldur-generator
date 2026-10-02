package common

import (
	"reflect"
	"testing"
)

func securityGroupsSet() FieldInfo {
	return FieldInfo{
		Name:           "security_groups",
		GoType:         TFTypeSet,
		ServerComputed: true,
		ItemSchema: &FieldInfo{Properties: []FieldInfo{
			{Name: "url", Required: true},
			{Name: "name", ReadOnly: true, UseStateForUnknown: true},
			{Name: "rules", GoType: TFTypeList, ReadOnly: true, UseStateForUnknown: true,
				ItemSchema: &FieldInfo{Properties: []FieldInfo{{Name: "id", ReadOnly: true, UseStateForUnknown: true}}}},
		}},
	}
}

func TestPrepareSetElementMatching_KeysAreConfigurableAttributes(t *testing.T) {
	floatingIPs := FieldInfo{
		Name:   "floating_ips",
		GoType: TFTypeSet,
		ItemSchema: &FieldInfo{Properties: []FieldInfo{
			{Name: "url", ServerComputed: true},
			{Name: "subnet", Required: true},
			{Name: "ip_address"},
			{Name: "address", ReadOnly: true},
			{Name: "hidden", SchemaSkip: true},
		}},
	}
	fields := []FieldInfo{securityGroupsSet(), floatingIPs}

	PrepareSetElementMatching(fields)

	if got := fields[0].SetMatchKeys; !reflect.DeepEqual(got, []string{"url"}) {
		t.Errorf("security_groups keys = %v, want [url]", got)
	}
	if got := fields[1].SetMatchKeys; !reflect.DeepEqual(got, []string{"ip_address", "subnet", "url"}) {
		t.Errorf("floating_ips keys = %v, want [ip_address subnet url]", got)
	}
}

func TestPrepareSetElementMatching_ClearsUseStateForUnknownInsideElements(t *testing.T) {
	fields := []FieldInfo{securityGroupsSet()}
	fields[0].UseStateForUnknown = true

	PrepareSetElementMatching(fields)

	if !fields[0].UseStateForUnknown {
		t.Error("the set itself should keep UseStateForUnknown")
	}
	props := fields[0].ItemSchema.Properties
	if props[1].UseStateForUnknown || props[2].UseStateForUnknown || props[2].ItemSchema.Properties[0].UseStateForUnknown {
		t.Error("attributes inside set elements must not use UseStateForUnknown")
	}
}

func TestPrepareSetElementMatching_LeavesListsAndReadOnlySets(t *testing.T) {
	list := FieldInfo{
		Name:       "ports",
		GoType:     TFTypeList,
		ItemSchema: &FieldInfo{Properties: []FieldInfo{{Name: "subnet", Required: true}, {Name: "mac", ReadOnly: true, UseStateForUnknown: true}}},
	}
	readOnlySet := securityGroupsSet()
	readOnlySet.ReadOnly = true
	fields := []FieldInfo{list, readOnlySet}

	PrepareSetElementMatching(fields)

	if fields[0].SetMatchKeys != nil || !fields[0].ItemSchema.Properties[1].UseStateForUnknown {
		t.Error("list elements pair by index correctly and must be left alone")
	}
	if fields[1].SetMatchKeys != nil {
		t.Errorf("a read-only set has nothing to match on, got keys %v", fields[1].SetMatchKeys)
	}
	if fields[1].ItemSchema.Properties[1].UseStateForUnknown {
		t.Error("a read-only set's elements must not use UseStateForUnknown either")
	}
}
