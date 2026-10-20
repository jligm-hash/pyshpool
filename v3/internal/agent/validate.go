package agent

import (
	"fmt"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

const maxPlanTasks = 32

// Validate checks IDs, dependencies, and cycles. The planner's output is
// untrusted, so this runs before anything is written or executed.
func Validate(plan Plan) error {
	if strings.TrimSpace(plan.Goal) == "" {
		return fmt.Errorf("plan goal is empty")
	}
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan has no tasks")
	}
	if len(plan.Tasks) > maxPlanTasks {
		return fmt.Errorf("plan has %d tasks; the maximum is %d", len(plan.Tasks), maxPlanTasks)
	}

	byID := make(map[string]Task, len(plan.Tasks))
	for i, task := range plan.Tasks {
		if !idPattern.MatchString(task.ID) {
			return fmt.Errorf("task %d has invalid id %q (use letters, digits, dot, underscore, or dash; max 64 chars)", i+1, task.ID)
		}
		if _, exists := byID[task.ID]; exists {
			return fmt.Errorf("duplicate task id %q", task.ID)
		}
		if strings.TrimSpace(task.Prompt) == "" {
			return fmt.Errorf("task %q has an empty prompt", task.ID)
		}
		byID[task.ID] = task
	}

	for _, task := range plan.Tasks {
		seen := make(map[string]bool, len(task.DependsOn))
		for _, dep := range task.DependsOn {
			if dep == task.ID {
				return fmt.Errorf("task %q depends on itself", task.ID)
			}
			if _, ok := byID[dep]; !ok {
				return fmt.Errorf("task %q depends on unknown task %q", task.ID, dep)
			}
			if seen[dep] {
				return fmt.Errorf("task %q lists dependency %q more than once", task.ID, dep)
			}
			seen[dep] = true
		}
	}

	// Kahn's algorithm: if not all nodes are visited, there is a cycle.
	indegree := make(map[string]int, len(plan.Tasks))
	adj := make(map[string][]string, len(plan.Tasks))
	for _, task := range plan.Tasks {
		indegree[task.ID] = len(task.DependsOn)
		for _, dep := range task.DependsOn {
			adj[dep] = append(adj[dep], task.ID)
		}
	}
	queue := make([]string, 0, len(plan.Tasks))
	for id, deg := range indegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[id] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if visited != len(plan.Tasks) {
		return fmt.Errorf("plan contains a dependency cycle")
	}
	return nil
}
