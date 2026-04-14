package diff

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

type FileDiff struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
	Hunks   []Hunk `json:"hunks"`
}

type Hunk struct {
	OldStart int        `json:"oldStart"`
	NewStart int        `json:"newStart"`
	Header   string     `json:"header"`
	Lines    []DiffLine `json:"lines"`
}

type DiffLine struct {
	Type    string `json:"type"`    // "context" | "add" | "remove"
	Content string `json:"content"` // line text without +/-/space prefix
	OldNo   int    `json:"oldNo"`   // 0 if not applicable
	NewNo   int    `json:"newNo"`   // 0 if not applicable
}

// Parse parses a unified diff (from git diff or jj diff) into FileDiff structs.
func Parse(r io.Reader) ([]FileDiff, error) {
	var files []FileDiff
	var cur *FileDiff
	var hunk *Hunk
	oldNo, newNo := 0, 0

	scanner := bufio.NewScanner(r)
	// Increase buffer for very long lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "diff -r "):
			if cur != nil {
				files = append(files, *cur)
			}
			cur = &FileDiff{}
			hunk = nil
			// Seed paths from "diff --git a/X b/Y" so empty files (which have
			// no --- or +++ lines) still display a name. --- / +++ override these.
			if rest, ok := strings.CutPrefix(line, "diff --git "); ok {
				if i := strings.LastIndex(rest, " b/"); i >= 0 {
					cur.OldPath = strings.TrimPrefix(rest[:i], "a/")
					cur.NewPath = rest[i+3:]
				}
			}

		case cur != nil && strings.HasPrefix(line, "--- "):
			path := strings.TrimPrefix(line, "--- ")
			path = strings.TrimPrefix(path, "a/")
			cur.OldPath = path

		case cur != nil && strings.HasPrefix(line, "+++ "):
			path := strings.TrimPrefix(line, "+++ ")
			path = strings.TrimPrefix(path, "b/")
			cur.NewPath = path

		case cur != nil && strings.HasPrefix(line, "@@ "):
			h := parseHunkHeader(line)
			cur.Hunks = append(cur.Hunks, h)
			hunk = &cur.Hunks[len(cur.Hunks)-1]
			oldNo = hunk.OldStart
			newNo = hunk.NewStart

		case hunk != nil && len(line) > 0:
			switch line[0] {
			case '+':
				hunk.Lines = append(hunk.Lines, DiffLine{
					Type:    "add",
					Content: line[1:],
					NewNo:   newNo,
				})
				newNo++
			case '-':
				hunk.Lines = append(hunk.Lines, DiffLine{
					Type:    "remove",
					Content: line[1:],
					OldNo:   oldNo,
				})
				oldNo++
			case ' ':
				hunk.Lines = append(hunk.Lines, DiffLine{
					Type:    "context",
					Content: line[1:],
					OldNo:   oldNo,
					NewNo:   newNo,
				})
				oldNo++
				newNo++
			case '\\':
				// "\ No newline at end of file" -- skip
			}

		case hunk != nil && len(line) == 0:
			// empty context line
			hunk.Lines = append(hunk.Lines, DiffLine{
				Type:    "context",
				Content: "",
				OldNo:   oldNo,
				NewNo:   newNo,
			})
			oldNo++
			newNo++
		}
	}

	if cur != nil {
		files = append(files, *cur)
	}

	return files, scanner.Err()
}

// parseHunkHeader parses "@@ -a,b +c,d @@ ..." into a Hunk.
func parseHunkHeader(line string) Hunk {
	// Format: @@ -oldStart[,oldLines] +newStart[,newLines] @@ [context]
	h := Hunk{Header: line}
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return h
	}
	// parts[1] = "-a,b", parts[2] = "+c,d"
	h.OldStart = parseHunkNum(parts[1][1:])
	h.NewStart = parseHunkNum(parts[2][1:])
	return h
}

func parseHunkNum(s string) int {
	if i := strings.Index(s, ","); i >= 0 {
		s = s[:i]
	}
	n, _ := strconv.Atoi(s)
	return n
}
