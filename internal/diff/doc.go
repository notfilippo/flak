// Package diff parses unified diff output into structured FileDiff values.
//
// It handles output from both git diff and jj diff, including multi-hunk
// files, added/removed/context lines, and large line buffers.
package diff
