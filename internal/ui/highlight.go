package ui

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// highlightLines syntax-highlights a batch of source lines for a given filename.
// All lines are tokenised together so multi-line constructs (strings, comments)
// resolve correctly across hunk boundaries.
// Falls back to the original plain strings on any error or unknown language.
func highlightLines(filename string, lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	lexer := lexers.Match(filename)
	if lexer == nil {
		return lines
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("github-dark")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		return lines
	}

	iterator, err := lexer.Tokenise(nil, strings.Join(lines, "\n"))
	if err != nil {
		return lines
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return lines
	}

	// Split back into per-line strings.
	// Chroma appends a trailing reset + newline; trim before splitting.
	raw := strings.TrimRight(buf.String(), "\n")
	parts := strings.Split(raw, "\n")

	out := make([]string, len(lines))
	for i := range lines {
		if i < len(parts) {
			out[i] = parts[i]
		} else {
			out[i] = lines[i]
		}
	}
	return out
}
