package filter

import (
	"reflect"
	"testing"
	"time"

	"github.com/Souheab/lazydevcontainer/internal/domain"
)

func TestApplySortsDevcontainersFirstThenName(t *testing.T) {
	containers := []domain.Container{
		{ID: "3", Name: "zeta", IsDevcontainer: false, Created: time.Unix(3, 0)},
		{ID: "2", Name: "beta", IsDevcontainer: true, Created: time.Unix(2, 0)},
		{ID: "1", Name: "alpha", IsDevcontainer: true, Created: time.Unix(1, 0)},
	}

	visible := Apply(containers, ModeAll, "")
	got := names(visible)
	want := []string{"alpha", "beta", "zeta"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestApplyFiltersDevcontainers(t *testing.T) {
	containers := []domain.Container{
		{ID: "1", Name: "app", IsDevcontainer: true},
		{ID: "2", Name: "db", IsDevcontainer: false},
	}

	visible := Apply(containers, ModeDevcontainers, "")
	if len(visible) != 1 || visible[0].Name != "app" {
		t.Fatalf("unexpected visible containers: %+v", visible)
	}
}

func TestApplyFiltersOrdinaryContainers(t *testing.T) {
	containers := []domain.Container{
		{ID: "1", Name: "app", IsDevcontainer: true},
		{ID: "2", Name: "db", IsDevcontainer: false},
	}

	visible := Apply(containers, ModeContainers, "")
	if len(visible) != 1 || visible[0].Name != "db" {
		t.Fatalf("unexpected visible containers: %+v", visible)
	}
}

func TestMatchesSearchAcrossFields(t *testing.T) {
	container := domain.Container{
		ID:               "abcdef0123456789",
		ShortID:          "abcdef012345",
		Name:             "lazydevcontainer-app",
		Image:            "golang:latest",
		Status:           "Up 2 hours",
		DevcontainerPath: "/home/me/lazydevcontainer",
		Labels: map[string]string{
			"devcontainer.local_folder": "/home/me/lazydevcontainer",
		},
		Mounts: []domain.Mount{{Source: "/home/me/lazydevcontainer", Destination: "/workspaces/lazydevcontainer"}},
	}

	for _, query := range []string{"lazy golang", "abcdef", "workspaces", "local_folder"} {
		if !MatchesSearch(container, query) {
			t.Fatalf("expected query %q to match", query)
		}
	}

	if MatchesSearch(container, "postgres") {
		t.Fatal("did not expect postgres query to match")
	}
}

func TestModeCycles(t *testing.T) {
	if ModeAll.Next() != ModeDevcontainers || ModeDevcontainers.Next() != ModeContainers || ModeContainers.Next() != ModeAll {
		t.Fatal("unexpected next cycle")
	}
	if ModeAll.Previous() != ModeContainers || ModeContainers.Previous() != ModeDevcontainers || ModeDevcontainers.Previous() != ModeAll {
		t.Fatal("unexpected previous cycle")
	}
}

func names(containers []domain.Container) []string {
	result := make([]string, 0, len(containers))
	for _, container := range containers {
		result = append(result, container.DisplayName())
	}
	return result
}
