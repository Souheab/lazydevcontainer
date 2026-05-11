package devcontainer

import (
	"testing"

	"github.com/Souheab/lazydevcontainer/internal/domain"
)

func TestDetectUsesLocalFolderLabelFirst(t *testing.T) {
	result := Detect(map[string]string{
		LabelLocalFolder:       "/home/me/project",
		LabelVSCodeLocalFolder: "/wrong",
	}, nil)

	if !result.IsDevcontainer {
		t.Fatal("expected devcontainer")
	}
	if result.Path != "/home/me/project" {
		t.Fatalf("expected local folder path, got %q", result.Path)
	}
	if result.Source != "label:"+LabelLocalFolder {
		t.Fatalf("unexpected source %q", result.Source)
	}
}

func TestDetectUsesConfigFileParent(t *testing.T) {
	result := Detect(map[string]string{
		LabelConfigFile: "/home/me/project/.devcontainer/devcontainer.json",
	}, nil)

	if !result.IsDevcontainer {
		t.Fatal("expected devcontainer")
	}
	if result.Path != "/home/me/project" {
		t.Fatalf("expected workspace parent, got %q", result.Path)
	}
}

func TestDetectUsesMetadataWithWorkspaceMount(t *testing.T) {
	result := Detect(map[string]string{
		LabelMetadata: "{}",
	}, []domain.Mount{
		{Source: "/home/me/project", Destination: "/workspaces/project"},
	})

	if !result.IsDevcontainer {
		t.Fatal("expected devcontainer")
	}
	if result.Path != "/home/me/project" {
		t.Fatalf("expected mount source path, got %q", result.Path)
	}
}

func TestDetectUsesMountFallback(t *testing.T) {
	result := Detect(nil, []domain.Mount{
		{Source: "/home/me/project", Destination: "/workspaces/project"},
	})

	if !result.IsDevcontainer {
		t.Fatal("expected devcontainer")
	}
	if result.Path != "/home/me/project" {
		t.Fatalf("expected mount source path, got %q", result.Path)
	}
}

func TestDetectIgnoresOrdinaryContainer(t *testing.T) {
	result := Detect(map[string]string{
		"com.example.service": "postgres",
	}, []domain.Mount{
		{Source: "pgdata", Destination: "/var/lib/postgresql/data"},
	})

	if result.IsDevcontainer {
		t.Fatalf("expected ordinary container, got %+v", result)
	}
}

func TestDetectDerivesWorkspaceFromDevcontainerMount(t *testing.T) {
	result := Detect(nil, []domain.Mount{
		{Source: "/home/me/project/.devcontainer", Destination: "/tmp/devcontainer"},
	})

	if !result.IsDevcontainer {
		t.Fatal("expected devcontainer")
	}
	if result.Path != "/home/me/project" {
		t.Fatalf("expected workspace parent, got %q", result.Path)
	}
}
