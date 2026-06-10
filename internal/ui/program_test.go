package ui

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/uho-wq/tv/internal/store"
)

// TestProgramEndToEnd は tea.Program 経由でモデルを起動し、描画とキー操作・
// 終了までの一連を TTY なしで検証する。
func TestProgramEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	content := "(A) 2026-06-01 buy milk +Home @errand\n(B) write report +Work\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, New(f), teatest.WithInitialTermSize(100, 24))

	// 初期描画が落ち着くのを待ってから終了させる。
	time.Sleep(300 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tv", "buy milk", "write report"} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("final output missing %q.\n--- output ---\n%s", want, out)
		}
	}
}
