package changelog

import (
	"fmt"
	"sort"
	"strings"
)

// AttrChange is a breaking change to an existing attribute.
type AttrChange struct {
	Path   string
	Detail string
}

// EntityDiff holds the attribute-level changes for an entity present in both
// the old and new manifests.
type EntityDiff struct {
	Name          string
	Kind          string
	Added         []string     // new optional/computed attributes (non-breaking)
	AddedRequired []string     // new required attributes (breaking)
	Removed       []string     // removed attributes (breaking)
	Changed       []AttrChange // changed attributes (breaking)
}

func (e EntityDiff) hasBreaking() bool {
	return len(e.AddedRequired) > 0 || len(e.Removed) > 0 || len(e.Changed) > 0
}

func (e EntityDiff) hasAny() bool {
	return e.hasBreaking() || len(e.Added) > 0
}

type entityRef struct {
	Name string
	Kind string
}

// Report is the result of diffing two manifests.
type Report struct {
	AddedEntities   []entityRef // new resources/data sources (non-breaking)
	RemovedEntities []entityRef // removed resources/data sources (breaking)
	Entities        []EntityDiff
}

// Diff compares two manifests and returns only the meaningful changes.
func Diff(oldM, newM *Manifest) Report {
	var r Report
	for _, name := range sortedKeys(oldM.Entities, newM.Entities) {
		oldE, inOld := oldM.Entities[name]
		newE, inNew := newM.Entities[name]
		switch {
		case !inNew:
			r.RemovedEntities = append(r.RemovedEntities, entityRef{name, oldE.Kind})
		case !inOld:
			r.AddedEntities = append(r.AddedEntities, entityRef{name, newE.Kind})
		default:
			d := EntityDiff{Name: name, Kind: newE.Kind}
			if oldE.Kind != newE.Kind {
				d.Changed = append(d.Changed, AttrChange{
					Path:   name,
					Detail: fmt.Sprintf("changed from %s to %s", oldE.Kind, newE.Kind),
				})
			}
			diffAttrs(oldE.Attributes, newE.Attributes, "", &d)
			if d.hasAny() {
				r.Entities = append(r.Entities, d)
			}
		}
	}
	return r
}

func diffAttrs(oldA, newA []Attribute, prefix string, d *EntityDiff) {
	oldIdx := indexAttrs(oldA)
	newIdx := indexAttrs(newA)
	for _, name := range sortedAttrKeys(oldIdx, newIdx) {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		o, inOld := oldIdx[name]
		n, inNew := newIdx[name]
		switch {
		case !inNew:
			d.Removed = append(d.Removed, path)
		case !inOld:
			if n.Required {
				d.AddedRequired = append(d.AddedRequired, path)
			} else {
				d.Added = append(d.Added, path)
			}
		default:
			if o.Type != n.Type {
				d.Changed = append(d.Changed, AttrChange{path, fmt.Sprintf("type changed from `%s` to `%s`", o.Type, n.Type)})
				continue // shape changed; deeper comparison is not meaningful
			}

			// Use robust state transition checks to avoid false positives/negatives

			// 1. Becoming Required (Breaking)
			if !o.Required && n.Required {
				d.Changed = append(d.Changed, AttrChange{path, "is now required"})
			}

			// 2. Becoming purely Computed / Read-Only (Breaking)
			// If it was configurable (Optional or Required) and is now strictly Computed.
			if (o.Optional || o.Required) && (!n.Optional && !n.Required && n.Computed) {
				d.Changed = append(d.Changed, AttrChange{path, "is now computed (read-only)"})
			}

			// 3. Losing Computed (Breaking)
			// If it was Optional+Computed, and loses Computed, existing configs relying on the
			// server-side default might now fail or behave differently.
			if (o.Optional && o.Computed) && (n.Optional && !n.Computed) {
				d.Changed = append(d.Changed, AttrChange{path, "no longer has a server-side default"})
			}

			diffAttrs(o.Attributes, n.Attributes, path, d)
		}
	}
}

// HasBreaking reports whether the diff contains any backward-incompatible change.
func (r Report) HasBreaking() bool {
	if len(r.RemovedEntities) > 0 {
		return true
	}
	for _, e := range r.Entities {
		if e.hasBreaking() {
			return true
		}
	}
	return false
}

// Empty reports whether there are no changes at all.
func (r Report) Empty() bool {
	return len(r.AddedEntities) == 0 && len(r.RemovedEntities) == 0 && len(r.Entities) == 0
}

// CommitSubject returns a single-line summary suitable for a git commit message.
func (r Report) CommitSubject() string {
	if r.Empty() {
		return "Update generated provider code"
	}
	var parts []string
	if n := len(r.AddedEntities); n > 0 {
		parts = append(parts, fmt.Sprintf("+%d %s", n, plural(n, "entity", "entities")))
	}
	added, breaking := r.attributeCounts()
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d %s", added, plural(added, "attribute", "attributes")))
	}
	if breaking > 0 {
		parts = append(parts, fmt.Sprintf("%d breaking %s", breaking, plural(breaking, "change", "changes")))
	}
	if len(parts) == 0 {
		return "Update generated provider code"
	}
	return "Update provider: " + strings.Join(parts, ", ")
}

func (r Report) attributeCounts() (added, breaking int) {
	breaking += len(r.RemovedEntities)
	for _, e := range r.Entities {
		added += len(e.Added)
		breaking += len(e.Removed) + len(e.AddedRequired) + len(e.Changed)
	}
	return added, breaking
}

// Markdown renders a CHANGELOG entry for the given date heading. Returns "" when
// there are no changes.
func (r Report) Markdown(date string) string {
	if r.Empty() {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n", date)

	if breaking := r.breakingLines(); len(breaking) > 0 {
		b.WriteString("\n### ⚠️ Breaking changes\n\n")
		for _, l := range breaking {
			fmt.Fprintf(&b, "- %s\n", l)
		}
	}
	if added := r.addedLines(); len(added) > 0 {
		b.WriteString("\n### Added\n\n")
		for _, l := range added {
			fmt.Fprintf(&b, "- %s\n", l)
		}
	}
	return b.String()
}

func (r Report) breakingLines() []string {
	var lines []string
	for _, e := range r.RemovedEntities {
		lines = append(lines, fmt.Sprintf("Removed %s `%s`", kindLabel(e.Kind), e.Name))
	}
	for _, e := range r.Entities {
		for _, a := range e.Removed {
			lines = append(lines, fmt.Sprintf("`%s`: removed attribute `%s`", e.Name, a))
		}
		for _, a := range e.AddedRequired {
			lines = append(lines, fmt.Sprintf("`%s`: new required attribute `%s`", e.Name, a))
		}
		for _, c := range e.Changed {
			lines = append(lines, fmt.Sprintf("`%s`: attribute `%s` %s", e.Name, c.Path, c.Detail))
		}
	}
	sort.Strings(lines)
	return lines
}

func (r Report) addedLines() []string {
	var lines []string
	for _, e := range r.AddedEntities {
		lines = append(lines, fmt.Sprintf("New %s `%s`", kindLabel(e.Kind), e.Name))
	}
	for _, e := range r.Entities {
		for _, a := range e.Added {
			lines = append(lines, fmt.Sprintf("`%s`: new attribute `%s`", e.Name, a))
		}
	}
	sort.Strings(lines)
	return lines
}

// --- helpers ---------------------------------------------------------------

func indexAttrs(attrs []Attribute) map[string]Attribute {
	m := make(map[string]Attribute, len(attrs))
	for _, a := range attrs {
		m[a.Name] = a
	}
	return m
}

func sortedKeys(a, b map[string]Entity) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedAttrKeys(a, b map[string]Attribute) []string {
	seen := map[string]struct{}{}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func kindLabel(kind string) string {
	if kind == "data_source" {
		return "data source"
	}
	return "resource"
}
