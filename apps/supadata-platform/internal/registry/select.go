package registry

import (
	"context"
	"fmt"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

func (s *Store) SelectProject(_ context.Context, id string) (project.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readLocked()
	if err != nil {
		return project.Project{}, err
	}
	found := -1
	for index := range current.Projects {
		if current.Projects[index].ID == id {
			found = index
			break
		}
	}
	if found < 0 {
		return project.Project{}, fmt.Errorf("project '%s' not found", id)
	}
	current.CurrentProjectID = id
	for index := range current.Projects {
		current.Projects[index].Current = index == found
	}
	if err := s.writeLocked(current); err != nil {
		return project.Project{}, err
	}
	return current.Projects[found], nil
}
