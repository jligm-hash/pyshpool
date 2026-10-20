// Package core contains the shared scheduler primitives used by both pysh
// and pysh-agent. It intentionally depends only on the Go standard library.
package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Task is one complete shell command read from a task file.
type Task struct {
	// Index is the 1-based position among non-empty, non-comment tasks.
	Index int `json:"index"`
	// Line is the 1-based line number in the source file.
	Line int `json:"line"`
	// Command is the original line, preserving quoting and redirections.
	Command string `json:"command"`
}

// ReadTasks reads one shell command per line. Blank lines and lines whose
// first non-space character is '#' are ignored. Commands are not trimmed, so
// leading whitespace and redirections are preserved.
func ReadTasks(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open task file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var tasks []Task
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		tasks = append(tasks, Task{
			Index:   len(tasks) + 1,
			Line:    lineNo,
			Command: raw,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("task file %q contains no commands", path)
	}
	return tasks, nil
}

// CountLines returns the number of lines in a file, including blank lines and
// comments. Slurm array indices are line-based, so this is the count used for
// array generation.
func CountLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open task file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	n := 0
	for scanner.Scan() {
		n++
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read task file: %w", err)
	}
	return n, nil
}
