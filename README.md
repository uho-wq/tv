# tv

A TUI tool for viewing tasks written in the [todo.txt format](https://github.com/todotxt/todo.txt).
Built with Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Installation

```sh
go install github.com/uho-wq/tv@latest
```


## Usage

```sh
tv [path]
```

If `path` is omitted, todo.txt is located in the following order of priority:

1. CLI argument
2. Environment variable `TODOTXT_FILE`
3. Environment variable `TODO_FILE` (todo.txt-cli compatible)
4. Environment variable `TODO_DIR` + `/todo.txt`
5. `~/todo.txt`

## Key bindings

| Key | Action |
|------|------|
| `↑`/`k`, `↓`/`j` | Move cursor |
| `g` / `G` | Jump to top / bottom |
| `x` / `Space` | Toggle completion (add/remove `x` + completion date) |
| `i` / `Enter` | Edit the focused line in place (`enter` to save, `esc` to cancel) |
| `o` | Add a new task at the end of the file (`enter` to save, `esc` to cancel) |
| `Ctrl+O` | Copy the focused task's raw text to the clipboard |
| `a` | Archive all completed tasks to `archive.txt` |
| `u` | Undo the last archive |
| `f` / `/` | Filter input (`+proj @ctx (A) keyword`) |
| `esc` | Clear filter |
| `tab` | Switch grouping (flat → project → context → priority) |
| `s` | Toggle sort (priority ⇔ completed: most recently completed first) |
| `c` | Show/hide completed tasks |
| `r` | Reload file |
| `e` | Open the file in `$EDITOR` (reloads on exit) |
| `?` | Help |
| `q` / `Ctrl+C` | Quit |
