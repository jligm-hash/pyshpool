package slurm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTasks(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tasks.dat")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerateOmitsEmptyAccount(t *testing.T) {
	taskFile := writeTasks(t, "echo one\necho two\n\n# comment\n")
	script, abs, err := Generate(Config{
		TaskFile:    taskFile,
		JobName:     "demo",
		Nodes:       2,
		CPUsPerTask: 8,
		ArrayLimit:  50,
		WorkDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if abs == "" || !strings.Contains(script, "echo") {
		t.Fatal("task file was not embedded")
	}
	if strings.Contains(script, "--account") {
		t.Fatal("empty account must not emit --account")
	}
	for _, want := range []string{
		"#SBATCH --array=1-4%50",
		"#SBATCH --nodes=2",
		"#SBATCH --cpus-per-task=8",
		"SLURM_ARRAY_TASK_ID",
		`sed -n "${SLURM_ARRAY_TASK_ID}p" "$TASK_FILE"`,
		`bash -c "$cmd"`,
		"set +e",
		"exit \"$status\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestGenerateIncludesAccount(t *testing.T) {
	taskFile := writeTasks(t, "true\n")
	script, _, err := Generate(Config{TaskFile: taskFile, Account: "myacct"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "#SBATCH --account=myacct") {
		t.Fatalf("account missing:\n%s", script)
	}
}

func TestGenerateEmptyFile(t *testing.T) {
	taskFile := writeTasks(t, "")
	if _, _, err := Generate(Config{TaskFile: taskFile}); err == nil {
		t.Fatal("expected an error for an empty task file")
	}
}

func TestArrayRange(t *testing.T) {
	cases := []struct {
		lines, limit int
		want         string
	}{
		{10, 0, "1-10"},
		{10, 4, "1-10%4"},
		{0, 0, "1-0"},
		{-1, 2, "1-0%2"},
	}
	for _, c := range cases {
		if got := ArrayRange(c.lines, c.limit); got != c.want {
			t.Fatalf("ArrayRange(%d,%d) = %q, want %q", c.lines, c.limit, got, c.want)
		}
	}
}
