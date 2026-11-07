// Package undo builds undo plans and VERIFIES them: run command then plan in the
// sandbox and check the tree matches the original. Unverified plans are never offered.
package undo
