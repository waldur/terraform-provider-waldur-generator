package changelog

import (
	"strings"
	"testing"
)

// Helper builders for test attributes
func reqAttr(name, typ string, nested ...Attribute) Attribute {
	return Attribute{Name: name, Type: typ, Required: true, Optional: false, Computed: false, Attributes: nested}
}

func optAttr(name, typ string, nested ...Attribute) Attribute {
	return Attribute{Name: name, Type: typ, Required: false, Optional: true, Computed: false, Attributes: nested}
}

func compAttr(name, typ string, nested ...Attribute) Attribute {
	return Attribute{Name: name, Type: typ, Required: false, Optional: false, Computed: true, Attributes: nested}
}

func optCompAttr(name, typ string, nested ...Attribute) Attribute {
	return Attribute{Name: name, Type: typ, Required: false, Optional: true, Computed: true, Attributes: nested}
}

func manifest(entities map[string]Entity) *Manifest {
	return &Manifest{Provider: "waldur", Entities: entities}
}

func res(attrs ...Attribute) Entity { return Entity{Kind: "resource", Attributes: attrs} }

// findEntity returns the EntityDiff for name, or fails.
func findEntity(t *testing.T, r Report, name string) EntityDiff {
	t.Helper()
	for _, e := range r.Entities {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no EntityDiff for %q", name)
	return EntityDiff{}
}

func TestRemovedEntityIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"))})
	nw := manifest(map[string]Entity{})
	r := Diff(old, nw)
	if !r.HasBreaking() || len(r.RemovedEntities) != 1 || r.RemovedEntities[0].Name != "order" {
		t.Fatalf("expected removed entity breaking, got %+v", r)
	}
}

func TestAddedEntityIsNotBreaking(t *testing.T) {
	old := manifest(map[string]Entity{})
	nw := manifest(map[string]Entity{"order": res(reqAttr("name", "types.String"))})
	r := Diff(old, nw)
	if r.HasBreaking() || len(r.AddedEntities) != 1 {
		t.Fatalf("added entity must not be breaking, got %+v", r)
	}
}

func TestRemovedAttributeIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"), optAttr("plan", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !contains(e.Removed, "plan") || !r.HasBreaking() {
		t.Fatalf("expected removed attr plan, got %+v", e)
	}
}

func TestNewRequiredAttributeIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"), reqAttr("plan", "types.String"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !contains(e.AddedRequired, "plan") || !r.HasBreaking() {
		t.Fatalf("expected added required plan, got %+v", e)
	}
}

func TestNewOptionalAttributeIsNotBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(optAttr("name", "types.String"), optAttr("note", "types.String"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if r.HasBreaking() || !contains(e.Added, "note") {
		t.Fatalf("optional add must not be breaking, got %+v", e)
	}
}

func TestTypeChangeIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("size", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(optAttr("size", "types.Int64"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !r.HasBreaking() || len(e.Changed) != 1 || !strings.Contains(e.Changed[0].Detail, "type changed") {
		t.Fatalf("expected type change, got %+v", e)
	}
}

func TestOptionalToRequiredIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("plan", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(reqAttr("plan", "types.String"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !r.HasBreaking() || len(e.Changed) != 1 || !strings.Contains(e.Changed[0].Detail, "now required") {
		t.Fatalf("expected now-required, got %+v", e)
	}
}

func TestSettableToComputedIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("plan", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(compAttr("plan", "types.String"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !r.HasBreaking() || len(e.Changed) != 1 || !strings.Contains(e.Changed[0].Detail, "computed") {
		t.Fatalf("expected now-computed, got %+v", e)
	}
}

func TestOptionalToOptionalComputedIsNotBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optAttr("plan", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(optCompAttr("plan", "types.String"))})
	r := Diff(old, nw)
	if r.HasBreaking() {
		t.Fatalf("optional to optional+computed must not be breaking, got %+v", r)
	}
}

func TestLosingComputedIsBreaking(t *testing.T) {
	old := manifest(map[string]Entity{"order": res(optCompAttr("plan", "types.String"))})
	nw := manifest(map[string]Entity{"order": res(optAttr("plan", "types.String"))})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !r.HasBreaking() || len(e.Changed) != 1 || !strings.Contains(e.Changed[0].Detail, "server-side default") {
		t.Fatalf("expected losing computed to be breaking, got %+v", e)
	}
}

func TestNestedAttributeChangesAreDetected(t *testing.T) {
	oldNested := optAttr("mappings", "types.Object",
		optAttr("networks", "types.List"),
		optAttr("subnets", "types.List"),
	)
	newNested := optAttr("mappings", "types.Object",
		reqAttr("networks", "types.List"),
	)
	old := manifest(map[string]Entity{"order": res(oldNested)})
	nw := manifest(map[string]Entity{"order": res(newNested)})
	r := Diff(old, nw)
	e := findEntity(t, r, "order")
	if !contains(e.Removed, "mappings.subnets") {
		t.Fatalf("expected nested removal mappings.subnets, got %+v", e)
	}
	foundReq := false
	for _, c := range e.Changed {
		if c.Path == "mappings.networks" && strings.Contains(c.Detail, "now required") {
			foundReq = true
		}
	}
	if !foundReq {
		t.Fatalf("expected nested now-required mappings.networks, got %+v", e)
	}
}

func TestNoChangesEmpty(t *testing.T) {
	m := manifest(map[string]Entity{"order": res(reqAttr("name", "types.String"))})
	r := Diff(m, m)
	if !r.Empty() || r.HasBreaking() {
		t.Fatalf("identical manifests must be empty, got %+v", r)
	}
	if r.CommitSubject() != "Update generated provider code" {
		t.Fatalf("empty subject mismatch: %q", r.CommitSubject())
	}
}

func TestCommitSubjectAndMarkdown(t *testing.T) {
	old := manifest(map[string]Entity{
		"order": res(optAttr("name", "types.String"), optAttr("plan", "types.String")),
		"gone":  res(optAttr("x", "types.String")),
	})
	nw := manifest(map[string]Entity{
		"order": res(optAttr("name", "types.String"), optAttr("note", "types.String")),
		"new":   res(optAttr("x", "types.String")),
	})
	r := Diff(old, nw)
	subj := r.CommitSubject()
	if !strings.HasPrefix(subj, "Update provider:") || !strings.Contains(subj, "breaking") {
		t.Fatalf("unexpected subject: %q", subj)
	}
	md := r.Markdown("2026-06-18")
	for _, want := range []string{"## 2026-06-18", "Breaking changes", "Removed resource `gone`", "New resource `new`", "removed attribute `plan`"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
