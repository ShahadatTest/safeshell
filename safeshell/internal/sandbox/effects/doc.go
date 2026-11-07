// Package effects holds the adapters for side effects the file sandbox cannot see:
// network attempts, process/service targets, database statements, git, package
// managers, disks. Each reports an Effect with an Evidence level
// (Observed, Previewed, Inspected, Estimated).
package effects
