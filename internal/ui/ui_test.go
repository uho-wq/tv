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

func TestDeleteViaKey(t *testing.T) {
	m, path := setup(t, "a\nb\nc\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// カーソルを b へ移して d
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = update(m, runes("d"))

	if got := readFile(t, path); got != "a\nc\n" {
		t.Fatalf("after delete todo = %q, want %q", got, "a\nc\n")
	}
	if m.statusErr {
		t.Errorf("unexpected error status: %q", m.status)
	}
	// カーソルは同じ序数 = 次のタスク (c) に留まる。
	if got := m.file.Tasks[m.selectedTaskIdx()].Raw; got != "c" {
		t.Errorf("cursor on %q, want c", got)
	}
}

func TestDeleteUndoRestoresPosition(t *testing.T) {
	m, path := setup(t, "a\nb\nc\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = update(m, runes("d"))
	m, _ = update(m, runes("u"))

	if got := readFile(t, path); got != "a\nb\nc\n" {
		t.Fatalf("after undo todo = %q, want %q", got, "a\nb\nc\n")
	}
	// 復元した行にカーソルが戻る。
	if got := m.file.Tasks[m.selectedTaskIdx()].Raw; got != "b" {
		t.Errorf("cursor on %q, want b", got)
	}
	// 二度目の u は取り消し対象なし。
	m, _ = update(m, runes("u"))
	if !m.statusErr {
		t.Errorf("expected warning on second undo, status=%q", m.status)
	}
}

func TestDeleteLastTaskLeavesEmptyList(t *testing.T) {
	m, path := setup(t, "only\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m, _ = update(m, runes("d"))
	if got := readFile(t, path); got != "" {
		t.Fatalf("after delete todo = %q, want empty", got)
	}
	if m.selectedTaskIdx() != -1 {
		t.Errorf("cursor should be invalid, got %d", m.selectedTaskIdx())
	}
	// タスクが無い状態でも d は何もしない（パニックしない）。
	m, _ = update(m, runes("d"))
	if got := readFile(t, path); got != "" {
		t.Errorf("todo.txt changed: %q", got)
	}
}

func TestDeleteDetectsExternalChange(t *testing.T) {
	m, path := setup(t, "a\nb\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(path, future, future)

	m, _ = update(m, runes("d"))
	if !m.statusErr {
		t.Errorf("expected external change warning, status=%q", m.status)
	}
	if got := readFile(t, path); got != "a\nb\nc\n" {
		t.Errorf("todo.txt was overwritten: %q", got)
	}
}

func TestCopyTask(t *testing.T) {
	m, _ := setup(t, "a +P\n(A) b @home\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// クリップボードは環境依存なのでフェイクに差し替える。
	var copied string
	m.copyText = func(s string) error {
		copied = s
		return nil
	}

	// priority ソートで (A) b @home が先頭。↓ で a +P へ移動し、ctrl+o でコピー。
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})

	if copied != "a +P" {
		t.Errorf("copied = %q, want %q", copied, "a +P")
	}
	if m.statusErr {
		t.Errorf("unexpected error status: %q", m.status)
	}
	if !strings.Contains(m.status, "コピー") {
		t.Errorf("status = %q, want copy message", m.status)
	}
}

func TestCopyTaskError(t *testing.T) {
	m, _ := setup(t, "a\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m.copyText = func(string) error { return os.ErrPermission }
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlO})

	if !m.statusErr {
		t.Errorf("コピー失敗時はエラー status になるべき: %q", m.status)
	}
}

func TestGroupToggle(t *testing.T) {
	m, _ := setup(t, "a +P @x\n")
	if m.groupKey != filter.GroupFlat {
		t.Fatalf("initial group = %v, want flat (起動時はグルーピングしない)", m.groupKey)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.groupKey != filter.GroupByProject {
		t.Errorf("after tab group = %v, want project", m.groupKey)
	}
}

func TestGroupHeaderVisibleAtTop(t *testing.T) {
	// WindowSizeMsg 前 (表示高さ 1 扱い) にグルーピング済みで rebuild されると、
	// offset が先頭見出しを飛ばした位置に固定される旧バグのリグレッションテスト。
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	if err := os.WriteFile(path, []byte("a +P\nb +P\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	m := New(f)
	m.groupKey = filter.GroupByProject
	m.rebuild() // height=0 のままグルーピング

	m, _ = update(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.offset != 0 {
		t.Errorf("offset = %d, want 0 (先頭のグループ見出しが見えるべき)", m.offset)
	}
	if out := m.View(); !strings.Contains(out, "+P (2)") {
		t.Errorf("view missing first group header:\n%s", out)
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

func TestPaneToggle(t *testing.T) {
	m, _ := setup(t, "a +Work\nb @errand\nc\n")

	m, _ = update(m, runes("p"))
	if !m.pane {
		t.Fatalf("pane mode not enabled")
	}
	if m.paneFocus != paneSidebar {
		t.Errorf("paneFocus = %v, want sidebar", m.paneFocus)
	}
	// サイドバー: +Work / @errand / (other)
	titles := make([]string, len(m.paneGroups))
	for i, g := range m.paneGroups {
		titles[i] = g.Title
	}
	want := []string{"+Work", "@errand", "(other)"}
	if strings.Join(titles, ",") != strings.Join(want, ",") {
		t.Errorf("paneGroups = %v, want %v", titles, want)
	}
	// リストは先頭グループ (+Work) のタスクのみ。
	if len(m.rows) != 1 || m.file.Tasks[m.rows[0].taskIdx].Raw != "a +Work" {
		t.Errorf("rows = %+v, want only 'a +Work'", m.rows)
	}
	// 描画にサイドバーとセパレータが含まれる。
	out := m.View()
	for _, s := range []string{"+Work (1)", "@errand (1)", "(other) (1)", "│"} {
		if !strings.Contains(out, s) {
			t.Errorf("view missing %q:\n%s", s, out)
		}
	}

	// もう一度 p で通常表示へ戻る。
	m, _ = update(m, runes("p"))
	if m.pane {
		t.Errorf("pane mode not disabled")
	}
}

func TestPaneSidebarNavigation(t *testing.T) {
	m, _ := setup(t, "a +Work\nb @errand\nc @errand\n")
	m, _ = update(m, runes("p"))

	// j で @errand グループへ。リストが追従する。
	m, _ = update(m, runes("j"))
	if got := m.paneGroups[m.paneSel].Title; got != "@errand" {
		t.Fatalf("selected group = %q, want @errand", got)
	}
	if len(m.rows) != 2 {
		t.Fatalf("rows = %d, want 2 (@errand のタスク)", len(m.rows))
	}
	// カーソルは新グループの先頭タスク。
	if got := m.file.Tasks[m.selectedTaskIdx()].Raw; got != "b @errand" {
		t.Errorf("cursor on %q, want 'b @errand'", got)
	}

	// enter でリストへフォーカス移動し、j はタスクカーソルを動かす。
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.paneFocus != paneList {
		t.Fatalf("paneFocus = %v, want list", m.paneFocus)
	}
	m, _ = update(m, runes("j"))
	if got := m.file.Tasks[m.selectedTaskIdx()].Raw; got != "c @errand" {
		t.Errorf("cursor on %q, want 'c @errand'", got)
	}

	// h でサイドバーへ戻り、tab でもフォーカスが切り替わる。
	m, _ = update(m, runes("h"))
	if m.paneFocus != paneSidebar {
		t.Errorf("after h paneFocus = %v, want sidebar", m.paneFocus)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.paneFocus != paneList {
		t.Errorf("after tab paneFocus = %v, want list", m.paneFocus)
	}
}

func TestPaneCompleteKeepsGroupSelection(t *testing.T) {
	m, path := setup(t, "a +Work\nb @errand\nc @errand\n")
	m, _ = update(m, runes("p"))
	m, _ = update(m, runes("j")) // @errand へ
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	// x で b を完了。グループ選択は @errand のまま、リストから消える。
	m, _ = update(m, runes("x"))
	if got := m.paneGroups[m.paneSel].Title; got != "@errand" {
		t.Errorf("selected group = %q, want @errand", got)
	}
	if len(m.rows) != 1 || m.file.Tasks[m.rows[0].taskIdx].Raw != "c @errand" {
		t.Errorf("rows should hold only 'c @errand': %+v", m.rows)
	}
	if !strings.Contains(readFile(t, path), "x ") {
		t.Errorf("file missing completed line")
	}
}

func TestPaneGroupDisappearsClampsSelection(t *testing.T) {
	m, _ := setup(t, "a +Work\nb @errand\n")
	m, _ = update(m, runes("p"))
	m, _ = update(m, runes("j")) // @errand へ

	// @errand 唯一のタスクを削除するとグループが消え、選択はクランプされる。
	m, _ = update(m, runes("d"))
	if len(m.paneGroups) != 1 {
		t.Fatalf("paneGroups = %+v, want only +Work", m.paneGroups)
	}
	if got := m.paneGroups[m.paneSel].Title; got != "+Work" {
		t.Errorf("selected group = %q, want +Work", got)
	}
	if got := m.file.Tasks[m.selectedTaskIdx()].Raw; got != "a +Work" {
		t.Errorf("cursor on %q, want 'a +Work'", got)
	}
}

func TestPaneEmptyFile(t *testing.T) {
	m, _ := setup(t, "")
	m, _ = update(m, runes("p"))
	if len(m.paneGroups) != 0 {
		t.Errorf("paneGroups = %+v, want empty", m.paneGroups)
	}
	// 空でもパニックせず描画できる。
	if out := m.View(); !strings.Contains(out, "グループなし") {
		t.Errorf("view missing empty sidebar message:\n%s", out)
	}
	// 空のままサイドバー操作してもパニックしない。
	m, _ = update(m, runes("j"))
	m, _ = update(m, runes("G"))
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

func TestEditLineFlow(t *testing.T) {
	m, path := setup(t, "a +P\nb +P\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// i で行編集モードへ。入力欄には原文が載る。
	m, _ = update(m, runes("i"))
	if m.mode != modeEdit {
		t.Fatalf("mode = %v, want edit", m.mode)
	}
	if got := m.editInput.Value(); got != "a +P" {
		t.Fatalf("editInput = %q, want %q", got, "a +P")
	}

	// " @home" を追記して enter で保存。
	for _, r := range " @home" {
		m, _ = update(m, runes(string(r)))
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	et := m.file.Tasks[0]
	if et.Raw != "a +P @home" {
		t.Errorf("Raw = %q, want %q", et.Raw, "a +P @home")
	}
	// 再パースで構造化フィールドも追随する。
	if !et.HasContext("home") {
		t.Errorf("contexts not reparsed: %+v", et.Contexts)
	}
	// ファイルに反映され、他の行は不変。
	if got := readFile(t, path); got != "a +P @home\nb +P\n" {
		t.Errorf("file = %q, want %q", got, "a +P @home\nb +P\n")
	}
}

func TestEditLineEscCancels(t *testing.T) {
	m, path := setup(t, "a\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m, _ = update(m, runes("i"))
	for _, r := range " changed" {
		m, _ = update(m, runes(string(r)))
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	if m.file.Tasks[0].Raw != "a" {
		t.Errorf("Raw = %q, want unchanged %q", m.file.Tasks[0].Raw, "a")
	}
	if got := readFile(t, path); got != "a\n" {
		t.Errorf("file = %q, want unchanged", got)
	}
}

func TestEditLineRejectsEmpty(t *testing.T) {
	m, path := setup(t, "a\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m, _ = update(m, runes("i"))
	m.editInput.SetValue("")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.statusErr {
		t.Errorf("空行への編集はエラー status になるべき")
	}
	if got := readFile(t, path); got != "a\n" {
		t.Errorf("file = %q, want unchanged", got)
	}
}

func TestAddLineFlow(t *testing.T) {
	m, path := setup(t, "a\nb\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// o で追加モードへ。入力欄は空。
	m, _ = update(m, runes("o"))
	if m.mode != modeEdit {
		t.Fatalf("mode = %v, want edit", m.mode)
	}
	if got := m.editInput.Value(); got != "" {
		t.Fatalf("editInput = %q, want empty", got)
	}
	if got := m.editInput.Prompt; got != "add> " {
		t.Errorf("prompt = %q, want %q", got, "add> ")
	}

	for _, r := range "new task +P" {
		m, _ = update(m, runes(string(r)))
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	// ファイル末尾に追記され、既存行は不変。
	if got := readFile(t, path); got != "a\nb\nnew task +P\n" {
		t.Errorf("file = %q, want %q", got, "a\nb\nnew task +P\n")
	}
	// カーソルは追加したタスクを指す。
	idx := m.selectedTaskIdx()
	if idx < 0 || m.file.Tasks[idx].Raw != "new task +P" {
		t.Errorf("cursor not on new task: idx=%d", idx)
	}
}

func TestAddLineEmptyCancels(t *testing.T) {
	m, path := setup(t, "a\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m, _ = update(m, runes("o"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.mode != modeNormal {
		t.Fatalf("mode = %v, want normal", m.mode)
	}
	if m.statusErr {
		t.Errorf("空入力の追加はエラーにせずキャンセル扱い: %q", m.status)
	}
	if got := readFile(t, path); got != "a\n" {
		t.Errorf("file = %q, want unchanged", got)
	}
}

func TestAddLinePromptRestoredOnEdit(t *testing.T) {
	m, _ := setup(t, "a\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	// o → esc の後に i で編集すると、プロンプトは edit> に戻る。
	m, _ = update(m, runes("o"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	m, _ = update(m, runes("i"))
	if got := m.editInput.Prompt; got != "edit> " {
		t.Errorf("prompt = %q, want %q", got, "edit> ")
	}
	if got := m.editInput.Value(); got != "a" {
		t.Errorf("editInput = %q, want %q", got, "a")
	}
}

func TestEditLineDetectsExternalChange(t *testing.T) {
	m, path := setup(t, "a\n")
	m.groupKey = filter.GroupFlat
	m.rebuild()

	m, _ = update(m, runes("i"))

	// 編集中に外部から書き換え。
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(path, future, future)

	for _, r := range "!" {
		m, _ = update(m, runes(string(r)))
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.statusErr {
		t.Errorf("外部変更の警告が出るべき, status=%q", m.status)
	}
	if got := readFile(t, path); got != "a\nb\n" {
		t.Errorf("todo.txt was overwritten: %q", got)
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
