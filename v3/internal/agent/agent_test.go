package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jligm-hash/pyshpool/v3/internal/core"
)

func validPlan() Plan {
	return Plan{
		Goal: "test goal",
		Tasks: []Task{
			{ID: "a", Title: "A", Prompt: "do A"},
			{ID: "b", Title: "B", Prompt: "do B"},
			{ID: "c", Title: "C", Prompt: "do C", DependsOn: []string{"a", "b"}},
		},
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		plan Plan
		ok   bool
	}{
		{"valid", validPlan(), true},
		{"empty goal", Plan{Goal: "", Tasks: []Task{{ID: "a", Prompt: "p"}}}, false},
		{"no tasks", Plan{Goal: "g"}, false},
		{"bad id", Plan{Goal: "g", Tasks: []Task{{ID: "a/b", Prompt: "p"}}}, false},
		{"duplicate id", Plan{Goal: "g", Tasks: []Task{{ID: "a", Prompt: "p"}, {ID: "a", Prompt: "q"}}}, false},
		{"empty prompt", Plan{Goal: "g", Tasks: []Task{{ID: "a", Prompt: " "}}}, false},
		{"unknown dep", Plan{Goal: "g", Tasks: []Task{{ID: "a", Prompt: "p", DependsOn: []string{"missing"}}}}, false},
		{"self dep", Plan{Goal: "g", Tasks: []Task{{ID: "a", Prompt: "p", DependsOn: []string{"a"}}}}, false},
		{"duplicate dep", Plan{Goal: "g", Tasks: []Task{{ID: "a", Prompt: "p"}, {ID: "b", Prompt: "p", DependsOn: []string{"a", "a"}}}}, false},
		{"cycle", Plan{Goal: "g", Tasks: []Task{
			{ID: "a", Prompt: "p", DependsOn: []string{"b"}},
			{ID: "b", Prompt: "p", DependsOn: []string{"a"}},
		}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.plan)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestMockPlanner(t *testing.T) {
	p := &Planner{Provider: &MockProvider{}, MaxTokens: 512, Temperature: 0.2}
	plan, err := p.Plan(context.Background(), "write a demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 4 {
		t.Fatalf("got %d tasks, want 4", len(plan.Tasks))
	}
	if plan.Goal != "write a demo" {
		t.Fatalf("goal = %q", plan.Goal)
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []string{
		`{"a":1}`,
		"```json\n{\"a\":1}\n```",
		"Here is the plan:\n{\"a\":1}\nDone.",
	}
	for _, c := range cases {
		got, err := extractJSON(c)
		if err != nil {
			t.Fatalf("extractJSON(%q): %v", c, err)
		}
		if !json.Valid([]byte(got)) {
			t.Fatalf("invalid JSON: %s", got)
		}
	}
	if _, err := extractJSON("no json here"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store, err := CreateStore(filepath.Join(t.TempDir(), ".pysh-agent"))
	if err != nil {
		t.Fatal(err)
	}
	plan := validPlan()
	if err := store.WritePlan(plan); err != nil {
		t.Fatal(err)
	}
	gotPlan, err := store.ReadPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(gotPlan.Tasks) != len(plan.Tasks) || gotPlan.Goal != plan.Goal {
		t.Fatalf("plan round trip failed: %+v", gotPlan)
	}
	if err := store.WritePrompt("a", "prompt"); err != nil {
		t.Fatal(err)
	}
	outPath, err := store.WriteOutput("a", "output")
	if err != nil {
		t.Fatal(err)
	}
	out, err := store.ReadOutput("a")
	if err != nil || out != "output" {
		t.Fatalf("output round trip: %q %v", out, err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteMeta(TaskMeta{ID: "a", Status: core.StatusSuccess, Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	meta, err := store.ReadMeta("a")
	if err != nil || meta.Status != core.StatusSuccess {
		t.Fatalf("meta round trip: %+v %v", meta, err)
	}
	completed := store.LoadCompleted(plan)
	if len(completed) != 1 || completed["a"].Status != core.StatusSuccess {
		t.Fatalf("completed: %+v", completed)
	}
	if _, err := store.WriteFinal("final"); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(Event{TaskID: "a", Status: core.StatusSuccess}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSummary(Summary{RunID: store.RunID}); err != nil {
		t.Fatal(err)
	}
}

func TestResolveStore(t *testing.T) {
	base := filepath.Join(t.TempDir(), ".pysh-agent")
	store, err := CreateStore(base)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ResolveStore(base, store.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dir != store.Dir {
		t.Fatalf("got %s, want %s", got.Dir, store.Dir)
	}
	got2, err := ResolveStore(base, store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if got2.RunID != store.RunID {
		t.Fatalf("got run %s, want %s", got2.RunID, store.RunID)
	}
}

func testRunner(t *testing.T, cfg Config) *Runner {
	t.Helper()
	if cfg.ProviderName == "" {
		cfg.ProviderName = "mock"
	}
	if cfg.Worker == "" {
		cfg.Worker = "api"
	}
	if cfg.Parallel == 0 {
		cfg.Parallel = 2
	}
	if cfg.MaxOutputTokens == 0 {
		cfg.MaxOutputTokens = 256
	}
	if cfg.TaskTimeout == 0 {
		cfg.TaskTimeout = 5 * time.Second
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 2 * time.Second
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = filepath.Join(t.TempDir(), ".pysh-agent")
	}
	r, err := NewRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func newStore(t *testing.T, cfg Config, plan Plan) *Store {
	t.Helper()
	store, err := CreateStore(cfg.WorkDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WritePlan(plan); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestRunnerMockAndResume(t *testing.T) {
	cfg := Config{WorkDir: filepath.Join(t.TempDir(), ".pysh-agent")}
	r := testRunner(t, cfg)
	plan, err := r.Plan(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	store := newStore(t, cfg, plan)
	summary, err := r.Execute(context.Background(), store, plan)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "success" || summary.Succeeded != len(plan.Tasks) {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Usage.InputTokens == 0 || summary.Usage.OutputTokens == 0 {
		t.Fatalf("usage was not recorded: %+v", summary.Usage)
	}
	if summary.FinalPath == "" {
		t.Fatal("final path is empty")
	}
	final, err := os.ReadFile(summary.FinalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(final), "Mock synthesis") {
		t.Fatalf("unexpected final:\n%s", final)
	}
	if len(store.LoadCompleted(plan)) != len(plan.Tasks) {
		t.Fatal("completed tasks were not persisted")
	}

	resumed, err := r.Execute(context.Background(), store, plan)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != "success" || resumed.Succeeded != len(plan.Tasks) {
		t.Fatalf("unexpected resumed summary: %+v", resumed)
	}
}

func TestCommandWorker(t *testing.T) {
	cfg := Config{
		WorkDir:       filepath.Join(t.TempDir(), ".pysh-agent"),
		Worker:        "command",
		WorkerCommand: "cat {{prompt_file}} > {{output_file}}",
		NoSynthesize:  true,
	}
	r := testRunner(t, cfg)
	plan := Plan{Goal: "command goal", Tasks: []Task{
		{ID: "one", Title: "One", Prompt: "first prompt"},
		{ID: "two", Title: "Two", Prompt: "second prompt"},
	}}
	store := newStore(t, cfg, plan)
	summary, err := r.Execute(context.Background(), store, plan)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status != "success" || summary.Succeeded != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	final, err := os.ReadFile(summary.FinalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(final), "first prompt") || !strings.Contains(string(final), "second prompt") {
		t.Fatalf("final does not contain worker outputs:\n%s", final)
	}
}

func TestCommandFailureSkipsDependent(t *testing.T) {
	cfg := Config{
		WorkDir:       filepath.Join(t.TempDir(), ".pysh-agent"),
		Worker:        "command",
		WorkerCommand: "exit 7",
		NoSynthesize:  true,
		Retries:       0,
	}
	r := testRunner(t, cfg)
	plan := Plan{Goal: "failure goal", Tasks: []Task{
		{ID: "fail", Prompt: "fail"},
		{ID: "after", Prompt: "after", DependsOn: []string{"fail"}},
	}}
	store := newStore(t, cfg, plan)
	summary, err := r.Execute(context.Background(), store, plan)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Failed != 1 || summary.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Status != "failed" {
		t.Fatalf("status = %q", summary.Status)
	}
}

func TestTokenBudget(t *testing.T) {
	cfg := Config{
		WorkDir:   filepath.Join(t.TempDir(), ".pysh-agent"),
		MaxTokens: 1,
		Parallel:  2,
	}
	r := testRunner(t, cfg)
	plan := Plan{Goal: "budget", Tasks: []Task{
		{ID: "a", Prompt: "a"},
		{ID: "b", Prompt: "b"},
	}}
	store := newStore(t, cfg, plan)
	summary, err := r.Execute(context.Background(), store, plan)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Status == "success" {
		t.Fatalf("budget should make the run partial: %+v", summary)
	}
	if !strings.Contains(summary.Error, "token budget") {
		t.Fatalf("error = %q", summary.Error)
	}
}

func TestCommandWorkerTimeout(t *testing.T) {
	cfg := Config{
		WorkDir:       filepath.Join(t.TempDir(), ".pysh-agent"),
		Worker:        "command",
		WorkerCommand: "sleep 2",
		NoSynthesize:  true,
		TaskTimeout:   100 * time.Millisecond,
	}
	r := testRunner(t, cfg)
	plan := Plan{Goal: "timeout", Tasks: []Task{{ID: "slow", Prompt: "slow"}}}
	store := newStore(t, cfg, plan)
	start := time.Now()
	summary, err := r.Execute(context.Background(), store, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Results) == 0 || summary.Results[0].Status != core.StatusTimeout {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if time.Since(start) > 1500*time.Millisecond {
		t.Fatalf("timeout took too long: %s", time.Since(start))
	}
}

func TestOpenAIProvider(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"test-model"`) {
			t.Errorf("unexpected request body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"test-model","choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`)
	}))
	defer server.Close()

	p, err := newOpenAIProvider(ProviderConfig{BaseURL: server.URL, APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Complete(context.Background(), CompletionRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/chat/completions" || gotAuth != "Bearer secret" {
		t.Fatalf("path=%q auth=%q", gotPath, gotAuth)
	}
	if resp.Text != "hello" || resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestAnthropicProvider(t *testing.T) {
	var gotPath, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"claude-test","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":4,"output_tokens":5}}`)
	}))
	defer server.Close()

	p, err := newAnthropicProvider(ProviderConfig{BaseURL: server.URL, APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "system", Content: "be nice"}, {Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/messages" || gotKey != "secret" {
		t.Fatalf("path=%q key=%q", gotPath, gotKey)
	}
	if resp.Text != "hello" || resp.Usage.InputTokens != 4 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
