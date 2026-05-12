package templates

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCatalogIDsAreUniqueAndJSONIsValid(t *testing.T) {
	seen := map[string]bool{}
	for _, template := range Catalog() {
		if template.ID == "" {
			t.Fatal("template ID must not be empty")
		}
		if seen[template.ID] {
			t.Fatalf("duplicate template ID %q", template.ID)
		}
		seen[template.ID] = true

		data, err := template.JSON()
		if err != nil {
			t.Fatalf("%s JSON returned error: %v", template.ID, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("%s generated invalid JSON: %v", template.ID, err)
		}
		if decoded["name"] == "" || decoded["image"] == "" {
			t.Fatalf("%s missing required name or image: %v", template.ID, decoded)
		}
	}
}

func TestRepresentativeTemplatesIncludeExpectedFields(t *testing.T) {
	for _, tc := range []struct {
		id       string
		contains []string
	}{
		{id: "base-ubuntu", contains: []string{"mcr.microsoft.com/devcontainers/base:ubuntu-24.04"}},
		{id: "go", contains: []string{"ghcr.io/devcontainers/features/go:1", "golang.Go"}},
		{id: "node-typescript", contains: []string{"mcr.microsoft.com/devcontainers/typescript-node", "dbaeumer.vscode-eslint"}},
		{id: "python", contains: []string{"mcr.microsoft.com/devcontainers/python", "ms-python.python"}},
		{id: "rust", contains: []string{"mcr.microsoft.com/devcontainers/rust", "rust-lang.rust-analyzer"}},
	} {
		template, ok := findByID(tc.id)
		if !ok {
			t.Fatalf("template %q not found", tc.id)
		}
		rendered, err := template.Render()
		if err != nil {
			t.Fatalf("%s render returned error: %v", tc.id, err)
		}
		for _, want := range tc.contains {
			if !strings.Contains(rendered, want) {
				t.Fatalf("%s render missing %q:\n%s", tc.id, want, rendered)
			}
		}
	}
}

func TestFilterMatchesNameTagsAndDescription(t *testing.T) {
	matches := Filter(Catalog(), "typescript web")
	if len(matches) != 1 || matches[0].ID != "node-typescript" {
		t.Fatalf("unexpected filter matches: %+v", matches)
	}
}

func findByID(id string) (Template, bool) {
	for _, template := range Catalog() {
		if template.ID == id {
			return template, true
		}
	}
	return Template{}, false
}
