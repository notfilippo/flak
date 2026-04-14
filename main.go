package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/notfilippo/flak/internal/diff"
	"github.com/notfilippo/flak/internal/ui"
)

func main() {
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
		fmt.Fprintln(os.Stderr, "usage: git diff | flak")
		os.Exit(1)
	}

	raw, readErr := io.ReadAll(os.Stdin)
	if readErr != nil {
		fmt.Fprintln(os.Stderr, readErr)
		os.Exit(1)
	}

	files, parseErr := diff.Parse(strings.NewReader(string(raw)))
	if parseErr != nil {
		fmt.Fprintln(os.Stderr, parseErr)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "flak: no diff found")
		return
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "flak: could not open terminal (is stdin a pipe without a controlling TTY?)", err)
		os.Exit(1)
	}
	defer tty.Close()

	comments, err := ui.Run(files, tty)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(comments) == 0 {
		fmt.Println("=== flak review: LGTM (no comments) ===")
		return
	}
	fmt.Println("=== flak review comments ===")
	fmt.Println()
	for _, c := range comments {
		fmt.Printf("%s:%d (%s)\n%s\n\n", c.File, c.Line, c.Side, c.Body)
	}
	fmt.Println("=== end ===")
}
