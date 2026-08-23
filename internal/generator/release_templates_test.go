package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// The release pipeline and goreleaser config are generated into the downstream
// repo, which the deploy job wipes and re-syncs on every run. A fix applied
// there is reverted by the next deploy -- that is how the gnupg fix came to be
// stripped by the commit that tag v8.1.2 points at, so v8.1.2 failed exactly
// like v8.1.0. These templates are the only durable copy, and nothing else in
// the suite renders them.
func renderReleaseFiles(t *testing.T) (gitlabCI, goreleaser string) {
	t.Helper()

	dir := t.TempDir()
	g := New(&config.Config{}, nil)
	g.config.Generator.OutputDir = dir
	g.config.Generator.ProviderName = "waldur"

	if err := g.generateGitLabCI(); err != nil {
		t.Fatalf("generateGitLabCI: %v", err)
	}
	if err := g.generateGoReleaser(); err != nil {
		t.Fatalf("generateGoReleaser: %v", err)
	}

	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(b)
	}
	return read(".gitlab-ci.yml"), read(".goreleaser.yml")
}

func TestGoReleaserTargetsGitHub(t *testing.T) {
	_, goreleaser := renderReleaseFiles(t)

	// The Terraform Registry only ingests GitHub releases. Without an explicit
	// target goreleaser infers the forge from the origin remote, which under
	// GitLab CI is the GitLab project, and the release never reaches the
	// Registry.
	for _, want := range []string{
		"force_token: github",
		"github:",
		"owner: waldur",
		"name: terraform-provider-waldur",
		"prerelease: auto",
	} {
		if !strings.Contains(goreleaser, want) {
			t.Errorf("rendered .goreleaser.yml is missing %q", want)
		}
	}

	// The provider name must be substituted, and goreleaser's own template
	// syntax must survive Go rendering.
	if strings.Contains(goreleaser, "{{ .ProviderName }}") {
		t.Error("ProviderName was not substituted")
	}
	if !strings.Contains(goreleaser, "{{ .Env.GPG_FINGERPRINT }}") {
		t.Error("goreleaser template syntax did not survive rendering")
	}
	if !strings.Contains(goreleaser, "terraform-provider-waldur_{{ .Version }}_SHA256SUMS") {
		t.Error("checksum name_template is not in the Registry's required form")
	}
}

func TestReleaseJobPreflight(t *testing.T) {
	gitlabCI, _ := renderReleaseFiles(t)

	// Every credential the job needs must be checked up front. The cryptic
	// "no valid OpenPGP data found" from an unset variable cost a release cycle.
	for _, want := range []string{
		"GPG_PRIVATE_KEY",
		"GPG_PASSPHRASE",
		"GITHUB_TOKEN",
		"gpg-agent", // the goreleaser image does not ship it
		"base64 -d", // GitLab masking forces single-line values
		"api.github.com",
		"goreleaser release --clean",
	} {
		if !strings.Contains(gitlabCI, want) {
			t.Errorf("rendered .gitlab-ci.yml is missing %q", want)
		}
	}

	if !strings.Contains(gitlabCI, `if: $CI_COMMIT_TAG =~ /^v/`) {
		t.Error("release job is not gated on a v* tag")
	}
}

// The provider schema's MarkdownDescription is what tfplugindocs renders as the
// Overview page on the Terraform Registry, and -- stripped of markup -- as the
// `description:` front matter that registry search results display. It was
// never set, so the published provider's Overview has been blank since the
// first release.
func TestProviderHasRegistryDescription(t *testing.T) {
	dir := t.TempDir()
	g := New(&config.Config{}, nil)
	g.config.Generator.OutputDir = dir
	g.config.Generator.ProviderName = "waldur"

	if err := g.generateProvider(); err != nil {
		t.Fatalf("generateProvider: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "internal", "provider", "provider.go"))
	if err != nil {
		// layout differs between generator versions; find it
		var found string
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
			if info != nil && info.Name() == "provider.go" {
				found = p
			}
			return nil
		})
		if found == "" {
			t.Fatalf("provider.go not generated: %v", err)
		}
		if b, err = os.ReadFile(found); err != nil {
			t.Fatal(err)
		}
	}
	src := string(b)

	// The schema-level description, not the per-attribute ones.
	idx := strings.Index(src, "resp.Schema = schema.Schema{")
	if idx < 0 {
		t.Fatal("provider Schema() not found")
	}
	head := src[idx:]
	if a := strings.Index(head, "Attributes:"); a > 0 {
		head = head[:a]
	}
	if !strings.Contains(head, "MarkdownDescription:") {
		t.Error("provider schema has no MarkdownDescription; the Registry Overview will render empty")
	}
	if !strings.Contains(head, "Waldur") {
		t.Error("provider description does not mention the product")
	}

	// tfplugindocs strips markdown for the front matter without preserving item
	// boundaries, so a bullet list here becomes one run-on sentence in registry
	// search results. Keep it prose.
	if strings.Contains(head, `\n- `) {
		t.Error("provider description contains a bullet list; it will run together in the front matter")
	}
}
