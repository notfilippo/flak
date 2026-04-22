# flak

![demo](docs/demo.png)

A fast, local diff review tool for the terminal. Pipe a unified diff in, leave inline comments, get them back on stdout. No IDE, no GitHub round-trips.

```
git diff main | flak
```

## Install

```sh
go install github.com/notfilippo/flak@latest
```

Or build from source:

```sh
git clone https://github.com/notfilippo/flak
cd flak
go build -o flak .
```

## Usage

```sh
# Review staged changes
git diff --cached | flak

# Review against a branch
git diff main..HEAD | flak

# Works with jj too
jj diff -r @ --git | flak

# Review against a branch
jj diff --from main --git | flak
```

When done, comments are printed to stdout:

```
=== flak review comments ===

src/main.go:42 (new)
Rename this variable, "x" is too generic here.

src/server.go:15 (new)
This function is doing too much. Split out the auth logic.

=== end ===
```

If you leave no comments: `=== flak review: LGTM (no comments) ===`

## Keybindings

| Key                    | Action                                           |
| ---------------------- | ------------------------------------------------ |
| `j` / `k` or `↑` / `↓` | Scroll line by line                              |
| `ctrl+d` / `ctrl+u`    | Scroll half page down / up                       |
| `ctrl+f` / `ctrl+b`    | Scroll full page down / up                       |
| `g` / `G`              | Jump to top / bottom                             |
| `]` / `[`              | Next / previous file                             |
| `f`                    | Fuzzy file picker                                |
| `/`                    | Search (confirm with `enter`, cancel with `esc`) |
| `n` / `N`              | Next / previous search match                     |
| `c`                    | Add inline comment on current line               |
| `e`                    | Edit comment under cursor                        |
| `d` or `x`             | Delete comment under cursor                      |
| `o`                    | Open current file in `$EDITOR`                   |
| `q`                    | Quit and print comments                          |
| `?`                    | Show keybinding help                             |

## Releasing

1. Describe and tag the new version:
   ```sh
   jj describe -m "chore: release vX.Y.Z"
   jj tag set vX.Y.Z
   ```
2. Push the bookmark and the tag (jj does not push tags via `git push`):
   ```sh
   jj bookmark set main -r vX.Y.Z
   jj git push --all
   git push origin vX.Y.Z
   ```

`go install github.com/notfilippo/flak@latest` picks up the new version automatically.
