// Package version holds build-time version metadata.
package version

import "fmt"

var (
	// Version is set at build time with -ldflags.
	Version = "dev"
	// Commit is the short git commit, set at build time.
	Commit = "none"
	// Date is the build date, set at build time.
	Date = "unknown"
)

// String returns a human-readable version line for pysh.
func String() string { return StringFor("pysh") }

// StringFor returns a version line for a named binary.
func StringFor(name string) string {
	if name == "" {
		name = "pysh"
	}
	return fmt.Sprintf("%s %s (commit %s, built %s)", name, Version, Commit, Date)
}
