package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

var projectIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Options struct {
	DataDir string
}

type state struct {
	CurrentProjectID string            `json:"currentProjectId"`
	Projects         []project.Project `json:"projects"`
}

type Store struct {
	registryPath string
	projectsDir  string
	mu           sync.Mutex
}

func New(options Options) (*Store, error) {
	if strings.TrimSpace(options.DataDir) == "" {
		return nil, errors.New("data directory is required")
	}
	if err := os.MkdirAll(filepath.Join(options.DataDir, "projects"), 0o700); err != nil {
		return nil, fmt.Errorf("create registry directories: %w", err)
	}
	return &Store{
		registryPath: filepath.Join(options.DataDir, "registry.json"),
		projectsDir:  filepath.Join(options.DataDir, "projects"),
	}, nil
}

func (s *Store) CreateProject(_ context.Context, name, requestedID string) (project.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return project.Project{}, errors.New("name is required")
	}
	id := Slugify(requestedID)
	if id == "" {
		id = Slugify(cleanName)
	}
	if !projectIDPattern.MatchString(id) {
		return project.Project{}, errors.New("id must be lowercase kebab-case")
	}

	current, err := s.readLocked()
	if err != nil {
		return project.Project{}, err
	}
	for _, candidate := range current.Projects {
		if candidate.ID == id {
			return project.Project{}, fmt.Errorf("project '%s' already exists", id)
		}
	}
	created := project.Project{
		ID:        id,
		Name:      cleanName,
		Status:    "registered",
		Current:   len(current.Projects) == 0,
		CreatedAt: time.Now().UTC(),
	}
	current.Projects = append(current.Projects, created)
	if current.CurrentProjectID == "" {
		current.CurrentProjectID = id
	}
	for index := range current.Projects {
		current.Projects[index].Current = current.Projects[index].ID == current.CurrentProjectID
	}
	if err := s.writeLocked(current); err != nil {
		return project.Project{}, err
	}
	return created, nil
}

func (s *Store) ListProjects(_ context.Context) ([]project.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	for index := range current.Projects {
		current.Projects[index].Current = current.Projects[index].ID == current.CurrentProjectID
	}
	return current.Projects, nil
}

func (s *Store) CurrentProject(_ context.Context) (*project.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	for index := range current.Projects {
		if current.Projects[index].ID == current.CurrentProjectID {
			current.Projects[index].Current = true
			return &current.Projects[index], nil
		}
	}
	return nil, nil
}

func Slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		isAlphaNumeric := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if isAlphaNumeric {
			builder.WriteRune(character)
			lastDash = false
			continue
		}
		if builder.Len() > 0 && !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if len(result) > 48 {
		return result[:48]
	}
	return result
}

func (s *Store) readLocked() (state, error) {
	contents, err := os.ReadFile(s.registryPath)
	if errors.Is(err, os.ErrNotExist) {
		return state{Projects: []project.Project{}}, nil
	}
	if err != nil {
		return state{}, fmt.Errorf("read registry: %w", err)
	}
	var current state
	if err := json.Unmarshal(contents, &current); err != nil {
		return state{}, fmt.Errorf("parse registry: %w", err)
	}
	return current, nil
}

func (s *Store) writeLocked(current state) error {
	contents, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(s.registryPath), ".registry-*.tmp")
	if err != nil {
		return fmt.Errorf("create registry temp file: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect registry temp file: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write registry temp file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close registry temp file: %w", err)
	}
	if err := os.Rename(temporaryName, s.registryPath); err != nil {
		return fmt.Errorf("replace registry: %w", err)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
