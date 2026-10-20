package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jligm-hash/pyshpool/v3/internal/core"
)

// Store persists one agent run.
type Store struct {
	RunID string
	Dir   string
	mu    sync.Mutex
}

// CreateStore creates a new run directory below base/runs.
func CreateStore(base string) (*Store, error) {
	id, dir, err := core.CreateRunDir(base)
	if err != nil {
		return nil, err
	}
	return &Store{RunID: id, Dir: dir}, nil
}

// OpenStore opens an existing run directory.
func OpenStore(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve run directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open run directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}
	return &Store{RunID: filepath.Base(abs), Dir: abs}, nil
}

// ResolveStore accepts a run directory or a run ID below base/runs.
func ResolveStore(base, ref string) (*Store, error) {
	if info, err := os.Stat(ref); err == nil && info.IsDir() {
		return OpenStore(ref)
	}
	if base == "" {
		base = ".pysh-agent"
	}
	return OpenStore(filepath.Join(base, "runs", ref))
}

func (s *Store) path(parts ...string) string {
	all := append([]string{s.Dir}, parts...)
	return filepath.Join(all...)
}

func (s *Store) TaskDir(id string) string { return s.path("tasks", id) }

// PrepareTask creates the task directory.
func (s *Store) PrepareTask(id string) error {
	if err := os.MkdirAll(s.TaskDir(id), 0o755); err != nil {
		return fmt.Errorf("create task directory %s: %w", id, err)
	}
	return nil
}

func (s *Store) WritePlan(plan Plan) error {
	return writeJSONAtomic(s.path("plan.json"), plan)
}

func (s *Store) ReadPlan() (Plan, error) {
	var plan Plan
	if err := readJSON(s.path("plan.json"), &plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func (s *Store) WritePrompt(id, prompt string) error {
	if err := s.PrepareTask(id); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.TaskDir(id), "prompt.txt"), []byte(prompt))
}

func (s *Store) WriteOutput(id, output string) (string, error) {
	if err := s.PrepareTask(id); err != nil {
		return "", err
	}
	path := filepath.Join(s.TaskDir(id), "output.md")
	if err := writeFileAtomic(path, []byte(output)); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) ReadOutput(id string) (string, error) {
	data, err := os.ReadFile(filepath.Join(s.TaskDir(id), "output.md"))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Store) WriteMeta(meta TaskMeta) error {
	if err := s.PrepareTask(meta.ID); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(s.TaskDir(meta.ID), "meta.json"), meta)
}

func (s *Store) ReadMeta(id string) (TaskMeta, error) {
	var meta TaskMeta
	if err := readJSON(filepath.Join(s.TaskDir(id), "meta.json"), &meta); err != nil {
		return TaskMeta{}, err
	}
	return meta, nil
}

func (s *Store) WriteFinal(text string) (string, error) {
	path := s.path("final.md")
	if err := writeFileAtomic(path, []byte(text)); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) WriteSummary(summary Summary) error {
	return writeJSONAtomic(s.path("summary.json"), summary)
}

func (s *Store) AppendEvent(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path("events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open events: %w", err)
	}
	defer f.Close()
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return nil
}

// LoadCompleted returns successful tasks from a previous run. It is the
// resume path: a task is reused only when meta says success and output exists.
func (s *Store) LoadCompleted(plan Plan) map[string]TaskResult {
	done := make(map[string]TaskResult)
	for _, task := range plan.Tasks {
		meta, err := s.ReadMeta(task.ID)
		if err != nil || meta.Status != core.StatusSuccess {
			continue
		}
		output, err := s.ReadOutput(task.ID)
		if err != nil || strings.TrimSpace(output) == "" {
			continue
		}
		done[task.ID] = TaskResult{
			ID:         task.ID,
			Title:      task.Title,
			Status:     core.StatusSuccess,
			Attempts:   meta.Attempts,
			DurationMS: meta.DurationMS,
			OutputPath: filepath.Join(s.TaskDir(task.ID), "output.md"),
			Usage:      meta.Usage,
		}
	}
	return done
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	return writeFileAtomic(path, append(data, '\n'))
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}
	return nil
}
