package config

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverFindsSupportedLocationsInPrecedenceOrder(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, ".devcontainer", "devcontainer.json"),
		filepath.Join(root, ".devcontainer.json"),
		filepath.Join(root, ".devcontainer", "service", "devcontainer.json"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	candidates, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != len(paths) {
		t.Fatalf("candidate count = %d, want %d: %+v", len(candidates), len(paths), candidates)
	}
	for index, want := range paths {
		if candidates[index].Path != want || !candidates[index].Exists {
			t.Fatalf("candidate %d = %+v, want existing %s", index, candidates[index], want)
		}
	}
}

func TestDiscoverReturnsDefaultTargetWhenMissing(t *testing.T) {
	root := t.TempDir()
	candidates, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".devcontainer", "devcontainer.json")
	if len(candidates) != 1 || candidates[0].Path != want || candidates[0].Exists {
		t.Fatalf("candidates = %+v, want default missing target %s", candidates, want)
	}
}

func TestLoadSavePreservesUnknownFieldsAndOmitsEmptySections(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".devcontainer", "devcontainer.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `{
  "name": "Old",
  "image": "ubuntu",
  "features": {
    "ghcr.io/devcontainers/features/go:1": {
      "version": "latest"
    }
  },
  "customizations": {
    "vscode": {
      "extensions": ["golang.Go"],
      "settings": {
        "go.useLanguageServer": true
      }
    }
  },
  "remoteUser": "vscode",
  "postCreateCommand": "go version",
  "workspaceFolder": "/workspaces/app"
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	doc.Name = "New"
	doc.Image = "mcr.microsoft.com/devcontainers/base:ubuntu-24.04"
	doc.RemoteUser = ""
	doc.PostCreateCommand = ""
	doc.Features = map[string]map[string]any{}
	doc.Extensions = nil
	doc.Settings = map[string]any{}

	if err := Save(doc); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"name": "New"`, `"image": "mcr.microsoft.com/devcontainers/base:ubuntu-24.04"`, `"workspaceFolder": "/workspaces/app"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved config missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{`"features"`, `"extensions"`, `"settings"`, `"remoteUser"`, `"postCreateCommand"`} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("saved config should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestLoadMissingFileReturnsEmptyDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".devcontainer", "devcontainer.json")
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Path != path || len(doc.Features) != 0 || len(doc.Raw) != 0 {
		t.Fatalf("unexpected missing-file document: %+v", doc)
	}
}

func TestFeatureCatalogUsesCacheWhenOnlineLoadFails(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "features.json")
	if err := writeFeatureCache(cachePath, []Feature{{ID: "ghcr.io/devcontainers/features/go:1", Name: "Go"}}); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}

	features, err := loadFeatureCatalog(context.Background(), client, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 1 || features[0].ID != "ghcr.io/devcontainers/features/go:1" {
		t.Fatalf("features = %+v", features)
	}
}

func TestSearchFeaturesMatchesEveryTerm(t *testing.T) {
	features := []Feature{
		{ID: "ghcr.io/devcontainers/features/go:1", Name: "Go"},
		{ID: "ghcr.io/devcontainers/features/node:2", Name: "Node.js"},
	}
	got := SearchFeatures(features, "go features")
	if len(got) != 1 || got[0].Name != "Go" {
		t.Fatalf("SearchFeatures returned %+v", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
