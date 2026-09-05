package registry

import (
	"context"
	"testing"
)

func TestRegistrySelectsAndPersistsAnotherProject(t *testing.T) {
	store, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := store.CreateProject(context.Background(), "First", "first"); err != nil {
		t.Fatalf("CreateProject(first) error = %v", err)
	}
	if _, err := store.CreateProject(context.Background(), "Second", "second"); err != nil {
		t.Fatalf("CreateProject(second) error = %v", err)
	}

	selected, err := store.SelectProject(context.Background(), "second")
	if err != nil {
		t.Fatalf("SelectProject() error = %v", err)
	}
	if selected.ID != "second" || !selected.Current {
		t.Fatalf("unexpected selected project: %+v", selected)
	}
	current, err := store.CurrentProject(context.Background())
	if err != nil {
		t.Fatalf("CurrentProject() error = %v", err)
	}
	if current == nil || current.ID != "second" {
		t.Fatalf("selection was not persisted: %+v", current)
	}
}
