package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/uho-wq/tv/internal/filter"
	"github.com/uho-wq/tv/internal/store"
)

func setup(t *testing.T, content string) (Model, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := New(f)
	m, _ = update(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	return m, path
}

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCompleteViaKey(t *testing.T) {
	m, path := setup(t, "a +P\nb +P\nc +Q\n")

	// 決定的にするため flat 表示にする（原文順）。
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// カーソルは先頭(a)。↓ で b へ。
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	idx := m.selectedTaskIdx()
	if got := m.file.Tasks[idx].Raw; got != "b +P" {
		t.Fatalf("cursor on %q, want b +P", got)
	}

	// x で完了マーク（todo.txt に残る）
	m, _ = update(m, runes("x"))

	bt := m.file.Tasks[idx]
	if !bt.Completed {
		t.Errorf("task not completed: %+v", bt)
	}
	if !strings.HasPrefix(bt.Raw, "x ") || !strings.Contains(bt.Raw, "b +P") {
		t.Errorf("completed Raw = %q, want 'x <date> b +P'", bt.Raw)
	}
	// ファイルに反映され、他の行は不変。
	got := readFile(t, path)
	if !strings.Contains(got, bt.Raw) {
		t.Errorf("file missing completed line %q:\n%s", bt.Raw, got)
	}
	if !strings.Contains(got, "a +P") || !strings.Contains(got, "c +Q") {
		t.Errorf("other lines changed: %q", got)
	}
}

func TestCompleteArchiveUndo(t *testing.T) {
	m, path := setup(t, "a\nb\nc\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// x で a を完了（先頭にカーソル）
	m, _ = update(m, runes("x"))
	completed := m.file.Tasks[0].Raw
	if !strings.HasPrefix(completed, "x ") {
		t.Fatalf("a not completed: %q", completed)
	}

	// a で完了済みを archive.txt へ移動
	m, _ = update(m, runes("a"))
	if got := readFile(t, path); got != "b\nc\n" {
		t.Fatalf("after archive todo = %q, want %q", got, "b\nc\n")
	}
	archivePath := filepath.Join(filepath.Dir(path), "archive.txt")
	if got := readFile(t, archivePath); got != completed+"\n" {
		t.Errorf("archive.txt = %q, want %q", got, completed+"\n")
	}

	// u で取消（完了行が todo.txt 末尾に戻る）
	m, _ = update(m, runes("u"))
	if got := readFile(t, path); got != "b\nc\n"+completed+"\n" {
		t.Errorf("after undo todo = %q, want %q", got, "b\nc\n"+completed+"\n")
	}
	if got := readFile(t, archivePath); got != "" {
		t.Errorf("archive.txt = %q, want empty", got)
	}
}

func TestSpaceCompletes(t *testing.T) {
	m, path := setup(t, "only\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()
	m, _ = update(m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.file.Tasks[0].Completed {
		t.Errorf("task not completed")
	}
	if got := readFile(t, path); !strings.HasPrefix(got, "x ") || !strings.Contains(got, "only") {
		t.Errorf("file = %q, want completed 'only'", got)
	}
}

func TestArchiveWithoutCompletedIsNoop(t *testing.T) {
	m, path := setup(t, "a\nb\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()
	m, _ = update(m, runes("a")) // 完了タスクなし
	if !m.statusErr {
		t.Errorf("expected warning when no completed tasks")
	}
	if got := readFile(t, path); got != "a\nb\n" {
		t.Errorf("todo.txt changed: %q", got)
	}
}

func TestGroupToggle(t *testing.T) {
	m, _ := setup(t, "a +P @x\n")
	if m.groupKey != filter.GroupByProject {
		t.Fatalf("initial group = %v, want project", m.groupKey)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.groupKey != filter.GroupByContext {
		t.Errorf("after tab group = %v, want context", m.groupKey)
	}
}

func TestFilterFlow(t *testing.T) {
	m, _ := setup(t, "buy milk +Home\nwrite report +Work\n")

	// f でフィルタモードへ
	m, _ = update(m, runes("f"))
	if m.mode != modeFilter {
		t.Fatalf("mode = %v, want filter", m.mode)
	}
	// "+Work" と入力して enter
	for _, r := range "+Work" {
		m, _ = update(m, runes(string(r)))
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	if m.taskCount() != 1 {
		t.Errorf("filtered count = %d, want 1", m.taskCount())
	}
	idx := m.selectedTaskIdx()
	if idx < 0 || !strings.Contains(m.file.Tasks[idx].Raw, "Work") {
		t.Errorf("selected task does not match filter: idx=%d", idx)
	}
}

func TestToggleCompleted(t *testing.T) {
	m, _ := setup(t, "active task\nx 2026-06-08 done task\n")
	if m.taskCount() != 1 {
		t.Fatalf("default count = %d, want 1 (completed hidden)", m.taskCount())
	}
	m, _ = update(m, runes("c"))
	if m.taskCount() != 2 {
		t.Errorf("after toggle count = %d, want 2", m.taskCount())
	}
}

func TestViewRenders(t *testing.T) {
	m, _ := setup(t, "(A) important +Proj @ctx due:2026-06-30\n")
	out := m.View()
	if !strings.Contains(out, "tv") {
		t.Errorf("view missing header")
	}
	if !strings.Contains(out, "important") {
		t.Errorf("view missing task text")
	}
}

func TestEditorCommand(t *testing.T) {
	tests := []struct {
		name     string
		editor   string
		visual   string
		wantName string
		wantArgs []string
	}{
		{"EDITOR優先", "nvim", "vim", "nvim", []string{"/p"}},
		{"EDITORに引数", "code --wait", "", "code", []string{"--wait", "/p"}},
		{"EDITOR空はVISUAL", "", "vim", "vim", []string{"/p"}},
		{"両方空はvi", "", "", "vi", []string{"/p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args := editorCommand(tt.editor, tt.visual, "/p")
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if strings.Join(args, " ") != strings.Join(tt.wantArgs, " ") {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestEditKeyReturnsCommand(t *testing.T) {
	m, _ := setup(t, "a\n")
	if _, cmd := update(m, runes("e")); cmd == nil {
		t.Errorf("e キーで tea.Cmd が返るべき")
	}
}

func TestEditorFinishedReloads(t *testing.T) {
	m, path := setup(t, "a\nb\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()
	if m.taskCount() != 2 {
		t.Fatalf("初期 count = %d, want 2", m.taskCount())
	}

	// エディタが行を追加したと仮定して外部から書き換える。
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// エディタ終了通知で再読込される。
	m, _ = update(m, editorFinishedMsg{})
	if m.taskCount() != 3 {
		t.Errorf("再読込後 count = %d, want 3", m.taskCount())
	}
	if m.statusErr {
		t.Errorf("予期しないエラー status: %q", m.status)
	}
}

func TestEditorFinishedError(t *testing.T) {
	m, _ := setup(t, "a\n")
	m, _ = update(m, editorFinishedMsg{err: os.ErrNotExist})
	if !m.statusErr {
		t.Errorf("エディタ起動失敗時はエラー status になるべき")
	}
}

func TestWriteDetectsExternalChange(t *testing.T) {
	m, path := setup(t, "a\nb\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// 外部から書き換え（mtime を進めるため内容変更）
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// mtime 解像度対策で明示的に未来へ
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(path, future, future)

	// x（完了 = 書き込み）は外部変更を検知して中断する。
	m, _ = update(m, runes("x"))
	if !m.statusErr {
		t.Errorf("expected external change warning, status=%q err=%v", m.status, m.statusErr)
	}
	// todo.txt は書き換えられていない（外部の内容のまま）
	if got := readFile(t, path); got != "a\nb\nc\n" {
		t.Errorf("todo.txt was overwritten: %q", got)
	}
}
