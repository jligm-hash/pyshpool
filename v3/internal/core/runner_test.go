package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTaskFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.dat")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadTasks(t *testing.T) {
	path := writeTaskFile(t, "echo one\n\n# comment\n  echo two  \r\necho 'three four'\n")
	tasks, err := ReadTasks(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(tasks))
	}
	if tasks[0].Index != 1 || tasks[0].Line != 1 || tasks[0].Command != "echo one" {
		t.Fatalf("unexpected first task: %+v", tasks[0])
	}
	if tasks[1].Index != 2 || tasks[1].Line != 4 || tasks[1].Command != "  echo two  " {
		t.Fatalf("unexpected second task: %+v", tasks[1])
	}
	if tasks[2].Line != 5 || tasks[2].Command != "echo 'three four'" {
		t.Fatalf("unexpected third task: %+v", tasks[2])
	}
}

func TestReadTasksRejectsEmptyFile(t *testing.T) {
	path := writeTaskFile(t, "\n# only comments\n\n")
	if _, err := ReadTasks(path); err == nil {
		t.Fatal("expected an error for a file with no commands")
	}
}

func TestCountLines(t *testing.T) {
	path := writeTaskFile(t, "a\n\n# c\nb\n")
	n, err := CountLines(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("got %d lines, want 4", n)
	}
}

func TestCreateRunDirIsUnique(t *testing.T) {
	base := t.TempDir()
	id1, dir1, err := CreateRunDir(base)
	if err != nil {
		t.Fatal(err)
	}
	id2, dir2, err := CreateRunDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 || dir1 == dir2 {
		t.Fatalf("run directories are not unique: %s %s", dir1, dir2)
	}
	if _, err := os.Stat(filepath.Join(dir1, "tasks")); err != nil {
		t.Fatalf("tasks directory missing: %v", err)
	}
}

func TestRunnerSuccess(t *testing.T) {
	dir := t.TempDir()
	taskFile := writeTaskFile(t, "echo one > out1.txt\necho two > out2.txt\n")
	tasks, err := ReadTasks(taskFile)
	if err != nil {
		t.Fatal(err)
	}
	var progress int
	runner := NewRunner(Config{
		Concurrency: 2,
		WorkDir:     filepath.Join(dir, ".pysh"),
		Cwd:         dir,
		Progress:    func(Result) { progress++ },
	})
	summary, err := runner.Run(context.Background(), tasks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Succeeded != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if progress != 2 {
		t.Fatalf("progress calls = %d, want 2", progress)
	}
	for _, name := range []string{"out1.txt", "out2.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s was not created: %v", name, err)
		}
	}
	var got Summary
	data, err := os.ReadFile(filepath.Join(summary.RunDir, "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.RunID != summary.RunID || len(got.Results) != 2 {
		t.Fatalf("summary.json does not match: %+v", got)
	}
}

func TestRunnerFailureExitCode(t *testing.T) {
	tasks, err := ReadTasks(writeTaskFile(t, "exit 3\n"))
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(Config{Concurrency: 1, WorkDir: filepath.Join(t.TempDir(), ".pysh")})
	summary, err := runner.Run(context.Background(), tasks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Failed != 1 || summary.Results[0].ExitCode != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestRunnerTimeout(t *testing.T) {
	tasks, err := ReadTasks(writeTaskFile(t, "sleep 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	runner := NewRunner(Config{
		Concurrency: 1,
		Timeout:     100 * time.Millisecond,
		WorkDir:     filepath.Join(t.TempDir(), ".pysh"),
	})
	summary, err := runner.Run(context.Background(), tasks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TimedOut != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if time.Since(start) > 1500*time.Millisecond {
		t.Fatalf("timeout did not stop the process quickly: %s", time.Since(start))
	}
}

func TestRunnerRetries(t *testing.T) {
	dir := t.TempDir()
	// Fail the first attempt, succeed on the second.
	cmd := `n=$(cat count 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > count; test "$n" -ge 2`
	tasks, err := ReadTasks(writeTaskFile(t, cmd+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(Config{
		Concurrency: 1,
		Retries:     1,
		WorkDir:     filepath.Join(dir, ".pysh"),
		Cwd:         dir,
	})
	summary, err := runner.Run(context.Background(), tasks)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Succeeded != 1 || summary.Results[0].Attempts != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestFormatCommand(t *testing.T) {
	if got := FormatCommand("abcdef", 4); got != "a..." {
		t.Fatalf("FormatCommand = %q, want %q", got, "a...")
	}
	if got := FormatCommand("a\nb", 10); got != "a b" {
		t.Fatalf("FormatCommand = %q", got)
	}
	if got := FormatCommand("abcdef", 0); got != "abcdef" {
		t.Fatalf("FormatCommand = %q", got)
	}
	if got := FormatCommand("héllo", 4); got != "h..." {
		t.Fatalf("FormatCommand = %q", got)
	}
}
