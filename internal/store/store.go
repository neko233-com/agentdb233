package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/agentdb233/internal/model"
)

type Store struct {
	dataDir string
	mu      sync.Mutex
}

func New(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

func (s *Store) DataDir() string { return s.dataDir }

func (s *Store) IndexPath(project string) string {
	return filepath.Join(s.dataDir, "index", sanitizeName(project)+".json")
}

func SanitizeName(v string) string {
	return sanitizeName(v)
}

func (s *Store) Init() error {
	for _, dir := range []string{
		s.dataDir,
		filepath.Join(s.dataDir, "knowledge"),
		filepath.Join(s.dataDir, "index"),
		filepath.Join(s.dataDir, "registry"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) AddKnowledge(entry model.KnowledgeEntry) (model.KnowledgeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(entry.Project) == "" {
		return entry, errors.New("project is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if entry.ID == "" {
		entry.ID = newID()
	}
	if entry.Kind == "" {
		entry.Kind = "note"
	}
	entry.CreatedAt = now
	entry.UpdatedAt = now
	path := s.knowledgePath(entry.Project)
	return entry, appendJSONLine(path, entry)
}

func (s *Store) ListKnowledge(project, query string) ([]model.KnowledgeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.KnowledgeEntry
	projects, err := s.projectsForQuery(project)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	for _, p := range projects {
		entries, err := readJSONLines[model.KnowledgeEntry](s.knowledgePath(p))
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if q == "" || knowledgeMatches(e, q) {
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

func (s *Store) UpsertSkill(skill model.Skill) (model.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(skill.Name) == "" {
		return skill, errors.New("name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items, err := readJSONFile[[]model.Skill](s.skillsPath())
	if err != nil {
		return skill, err
	}
	if skill.ID == "" {
		skill.ID = newID()
	}
	skill.CreatedAt = now
	skill.UpdatedAt = now
	for i := range items {
		if items[i].ID == skill.ID || strings.EqualFold(items[i].Name, skill.Name) {
			skill.ID = items[i].ID
			skill.CreatedAt = items[i].CreatedAt
			skill.UpdatedAt = now
			items[i] = skill
			return skill, writeJSONFile(s.skillsPath(), items)
		}
	}
	items = append(items, skill)
	return skill, writeJSONFile(s.skillsPath(), items)
}

func (s *Store) ListSkills() ([]model.Skill, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := readJSONFile[[]model.Skill](s.skillsPath())
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (s *Store) UpsertMCP(mcp model.MCPEndpoint) (model.MCPEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(mcp.Name) == "" {
		return mcp, errors.New("name is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items, err := readJSONFile[[]model.MCPEndpoint](s.mcpPath())
	if err != nil {
		return mcp, err
	}
	if mcp.ID == "" {
		mcp.ID = newID()
	}
	mcp.CreatedAt = now
	mcp.UpdatedAt = now
	for i := range items {
		if items[i].ID == mcp.ID || strings.EqualFold(items[i].Name, mcp.Name) {
			mcp.ID = items[i].ID
			mcp.CreatedAt = items[i].CreatedAt
			mcp.UpdatedAt = now
			items[i] = mcp
			return mcp, writeJSONFile(s.mcpPath(), items)
		}
	}
	items = append(items, mcp)
	return mcp, writeJSONFile(s.mcpPath(), items)
}

func (s *Store) ListMCP() ([]model.MCPEndpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := readJSONFile[[]model.MCPEndpoint](s.mcpPath())
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (s *Store) projectsForQuery(project string) ([]string, error) {
	if strings.TrimSpace(project) != "" {
		return []string{sanitizeName(project)}, nil
	}
	entries, err := os.ReadDir(filepath.Join(s.dataDir, "knowledge"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var projects []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			projects = append(projects, strings.TrimSuffix(e.Name(), ".jsonl"))
		}
	}
	return projects, nil
}

func (s *Store) knowledgePath(project string) string {
	return filepath.Join(s.dataDir, "knowledge", sanitizeName(project)+".jsonl")
}

func (s *Store) skillsPath() string { return filepath.Join(s.dataDir, "registry", "skills.json") }
func (s *Store) mcpPath() string    { return filepath.Join(s.dataDir, "registry", "mcp.json") }

func knowledgeMatches(e model.KnowledgeEntry, q string) bool {
	hay := strings.ToLower(e.Project + "\n" + e.Kind + "\n" + e.Title + "\n" + e.Body + "\n" + strings.Join(e.Tags, "\n") + "\n" + e.Git.Branch + "\n" + e.Git.Tag + "\n" + e.Git.Commit + "\n" + e.Git.Version)
	return strings.Contains(hay, q)
}

func sanitizeName(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func newID() string {
	return strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "") + "Z"
}

func appendJSONLine[T any](path string, value T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(b, '\n'))
	return err
}

func readJSONLines[T any](path string) ([]T, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	out := make([]T, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func readJSONFile[T any](path string) (T, error) {
	var zero T
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return zero, nil
	}
	if err != nil {
		return zero, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return zero, nil
	}
	return zero, json.Unmarshal(b, &zero)
}

func writeJSONFile[T any](path string, value T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
