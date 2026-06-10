// Command tv は todo.txt フォーマットのタスクを閲覧する TUI ツール。
//
// 使い方:
//
//	tv [path]
//
// path 省略時は環境変数 (TODOTXT_FILE / TODO_FILE / TODO_DIR) を参照し、
// 見つからなければ ~/todo.txt を開く。
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uho-wq/tv/internal/config"
	"github.com/uho-wq/tv/internal/store"
	"github.com/uho-wq/tv/internal/ui"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tv - todo.txt 閲覧 TUI\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [path]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "path 省略時は TODOTXT_FILE / TODO_FILE / TODO_DIR / ~/todo.txt の順に解決します。\n")
	}
	flag.Parse()

	var arg string
	if flag.NArg() > 0 {
		arg = flag.Arg(0)
	}

	if err := run(arg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(arg string) error {
	path, err := config.ResolvePath(arg)
	if err != nil {
		return err
	}

	f, err := store.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("todo.txt が見つかりません: %s\n引数でパスを指定するか ~/todo.txt を作成してください", path)
		}
		return err
	}

	p := tea.NewProgram(ui.New(f), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
