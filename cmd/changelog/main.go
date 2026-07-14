// Command changelog diffs two provider manifests and reports what changed.
//
// It prints a one-line git commit subject to stdout (so CI can capture it) and,
// when -changelog is given, prepends a dated entry to that CHANGELOG.md file.
//
//	go run ./cmd/changelog -old old-manifest.json -new output/provider-manifest.json -changelog CHANGELOG.md
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/waldur/terraform-provider-waldur-generator/internal/changelog"
)

const changelogHeader = "# Changelog\n"

func main() {
	oldPath := flag.String("old", "", "path to the previous provider-manifest.json (missing = treated as empty)")
	newPath := flag.String("new", "", "path to the current provider-manifest.json (required)")
	clPath := flag.String("changelog", "", "optional CHANGELOG.md to prepend a dated entry to")
	date := flag.String("date", "", "date heading for the entry (default: today, UTC)")
	flag.Parse()

	if *newPath == "" {
		log.Fatal("-new is required")
	}

	oldM, err := changelog.Load(*oldPath)
	if err != nil {
		log.Fatalf("reading old manifest: %v", err)
	}
	newM, err := changelog.Load(*newPath)
	if err != nil {
		log.Fatalf("reading new manifest: %v", err)
	}

	report := changelog.Diff(oldM, newM)

	if *clPath != "" && !report.Empty() {
		day := *date
		if day == "" {
			day = time.Now().UTC().Format("2006-01-02")
		}
		if err := prependChangelog(*clPath, report.Markdown(day)); err != nil {
			log.Fatalf("writing changelog: %v", err)
		}
	}

	// Commit subject on stdout — the only thing printed, so CI can `$(...)` it.
	fmt.Println(report.CommitSubject())
}

// prependChangelog inserts entry just below the "# Changelog" header, preserving
// existing history.
func prependChangelog(path, entry string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	body := strings.TrimPrefix(string(existing), changelogHeader)
	body = strings.TrimLeft(body, "\n")
	out := changelogHeader + "\n" + strings.TrimRight(entry, "\n") + "\n"
	if body != "" {
		out += "\n" + body
	}
	return os.WriteFile(path, []byte(out), 0o644)
}
