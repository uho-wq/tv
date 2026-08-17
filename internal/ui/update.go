package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/uho-wq/tv/internal/filter"
	"github.com/uho-wq/tv/internal/store"
	"github.com/uho-wq/tv/internal/todotxt"
)

// editorFinishedMsg は外部エディタ (tea.ExecProcess) の終了を通知する。
type editorFinishedMsg struct{ err error }

// Update は Bubble Tea のメッセージを処理する。
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - len(m.input.Prompt) - 4
		m.editInput.Width = msg.Width - len(m.editInput.Prompt) - 4
		m.ensureVisible()
		return m, nil

	case editorFinishedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("エディタの起動に失敗しました: %v", msg.err), true)
			return m, nil
		}
		// エディタで書き換えられている可能性があるため読み直す。
		m.reload()
		if !m.statusErr {
			m.setStatus("編集を反映しました", false)
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeFilter:
			return m.updateFilterMode(msg)
		case modeEdit:
			return m.updateEditMode(msg)
		}
		return m.updateNormalMode(msg)
	}
	return m, nil
}

func (m Model) updateNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearStatus()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.help = !m.help
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.paneSidebarActive() {
			m.paneMove(-1)
		} else {
			m.moveCursor(-1)
		}
	case key.Matches(msg, m.keys.Down):
		if m.paneSidebarActive() {
			m.paneMove(1)
		} else {
			m.moveCursor(1)
		}
	case key.Matches(msg, m.keys.Top):
		if m.paneSidebarActive() {
			m.paneSelect(0)
		} else {
			m.cursorToEdge(-1)
		}
	case key.Matches(msg, m.keys.Bottom):
		if m.paneSidebarActive() {
			m.paneSelect(len(m.paneGroups) - 1)
		} else {
			m.cursorToEdge(1)
		}

	case key.Matches(msg, m.keys.Pane):
		m.pane = !m.pane
		if m.pane {
			m.paneFocus = paneSidebar
			m.setStatus("pane 表示: on  (h/l でフォーカス移動)", false)
		} else {
			m.setStatus("pane 表示: off", false)
		}
		m.rebuild()

	case key.Matches(msg, m.keys.PaneLeft):
		if m.pane {
			m.paneFocus = paneSidebar
		}
	case key.Matches(msg, m.keys.PaneRight):
		if m.pane {
			m.paneFocus = paneList
		}

	case key.Matches(msg, m.keys.Group):
		if m.pane {
			// pane 表示中の tab はペイン間のフォーカス切替。
			if m.paneFocus == paneSidebar {
				m.paneFocus = paneList
			} else {
				m.paneFocus = paneSidebar
			}
			return m, nil
		}
		m.groupKey = m.groupKey.Next()
		m.rebuild()
		m.setStatus(fmt.Sprintf("グループ: %s", groupLabel(m.groupKey)), false)

	case key.Matches(msg, m.keys.Sort):
		m.sortKey = m.sortKey.Next()
		m.rebuild()
		s := fmt.Sprintf("ソート: %s", sortLabel(m.sortKey))
		if m.sortKey == filter.SortCompletion && !m.criteria.ShowCompleted {
			s += "  (完了タスク非表示中。c で表示)"
		}
		m.setStatus(s, false)

	case key.Matches(msg, m.keys.ToggleCompleted):
		m.criteria.ShowCompleted = !m.criteria.ShowCompleted
		m.rebuild()
		if m.criteria.ShowCompleted {
			m.setStatus("完了タスクを表示", false)
		} else {
			m.setStatus("完了タスクを非表示", false)
		}

	case key.Matches(msg, m.keys.Filter):
		m.mode = modeFilter
		m.input.Focus()
		return m, textinput.Blink

	case key.Matches(msg, m.keys.ClearFilter):
		if !m.criteria.Empty() {
			m.criteria = filter.Criteria{ShowCompleted: m.criteria.ShowCompleted}
			m.input.SetValue("")
			m.rebuild()
			m.setStatus("フィルタを解除しました", false)
		}

	case key.Matches(msg, m.keys.Reload):
		m.reload()

	case key.Matches(msg, m.keys.Edit):
		return m, m.openEditor()

	case key.Matches(msg, m.keys.EditLine):
		// サイドバーで enter したときはグループ決定 = リストへフォーカス移動。
		if m.paneSidebarActive() && msg.String() == "enter" {
			m.paneFocus = paneList
			return m, nil
		}
		return m, m.startEditLine()

	case key.Matches(msg, m.keys.AddLine):
		return m, m.startAddLine()

	case key.Matches(msg, m.keys.Copy):
		m.copyTask()

	case key.Matches(msg, m.keys.Undo):
		m.undo()

	case key.Matches(msg, m.keys.Complete):
		m.toggleComplete()

	case key.Matches(msg, m.keys.Delete):
		m.deleteTask()

	case key.Matches(msg, m.keys.Archive):
		m.archive()
	}

	return m, nil
}

func (m Model) updateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		showCompleted := m.criteria.ShowCompleted
		m.criteria = filter.ParseQuery(m.input.Value())
		m.criteria.ShowCompleted = showCompleted
		m.mode = modeNormal
		m.input.Blur()
		m.rebuild()
		if m.criteria.Empty() {
			m.setStatus("フィルタなし", false)
		} else {
			m.setStatus("フィルタを適用しました", false)
		}
		return m, nil

	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// startEditLine はカーソル位置のタスクの原文を入力欄に載せ、行編集モードへ移る。
func (m *Model) startEditLine() tea.Cmd {
	idx := m.selectedTaskIdx()
	if idx < 0 {
		return nil
	}
	m.editIdx = idx
	m.setEditPrompt("edit> ")
	m.editInput.SetValue(m.file.Tasks[idx].Raw)
	m.editInput.CursorEnd()
	m.editInput.Focus()
	m.mode = modeEdit
	return textinput.Blink
}

// startAddLine は空の入力欄で行編集モードへ移る（editIdx = -1 が新規追加を表す）。
func (m *Model) startAddLine() tea.Cmd {
	m.editIdx = -1
	m.setEditPrompt("add> ")
	m.editInput.SetValue("")
	m.editInput.Focus()
	m.mode = modeEdit
	return textinput.Blink
}

// setEditPrompt は入力欄のプロンプトを切り替え、幅を追随させる。
func (m *Model) setEditPrompt(prompt string) {
	m.editInput.Prompt = prompt
	if m.width > 0 {
		m.editInput.Width = m.width - len(prompt) - 4
	}
}

func (m Model) updateEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
		m.editInput.Blur()
		m.commitEditLine()
		return m, nil

	case "esc":
		m.mode = modeNormal
		m.editInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

// commitEditLine は入力欄の内容を確定してファイルに保存する。
// editIdx >= 0 なら既存タスクの差し替え、-1 なら末尾への新規追加。
// 再パースにより優先度・project・context 等の構造化フィールドも追随する。
func (m *Model) commitEditLine() {
	if m.editIdx < 0 {
		m.commitAddLine()
		return
	}

	idx := m.editIdx
	if idx >= len(m.file.Tasks) {
		return
	}
	raw := m.editInput.Value()
	if raw == m.file.Tasks[idx].Raw {
		return
	}
	if strings.TrimSpace(raw) == "" {
		m.setStatus("空の行にはできません", true)
		return
	}
	if !m.guardExternalChange() {
		return
	}

	m.file.Tasks[idx] = todotxt.ParseLine(raw, m.file.Tasks[idx].LineNo)
	if err := m.file.Save(); err != nil {
		m.setStatus(fmt.Sprintf("保存失敗: %v", err), true)
		return
	}
	m.rebuild()
	m.setStatus("行を更新しました", false)
}

// commitAddLine は入力欄の内容を新規タスクとして末尾に追加し、保存する。
// 空入力は追加せず黙ってキャンセルする。
func (m *Model) commitAddLine() {
	raw := m.editInput.Value()
	if strings.TrimSpace(raw) == "" {
		return
	}
	if !m.guardExternalChange() {
		return
	}

	newIdx := len(m.file.Tasks)
	m.file.Tasks = append(m.file.Tasks, todotxt.ParseLine(raw, newIdx))
	if err := m.file.Save(); err != nil {
		m.file.Tasks = m.file.Tasks[:newIdx] // 保存できなかった行は残さない
		m.setStatus(fmt.Sprintf("保存失敗: %v", err), true)
		return
	}
	m.rebuild()
	if !m.cursorToTask(newIdx) {
		m.setStatus("タスクを追加しました  (フィルタにより非表示)", false)
		return
	}
	m.setStatus("タスクを追加しました", false)
}

// copyTask はカーソル位置のタスクの原文をクリップボードへコピーする。
func (m *Model) copyTask() {
	idx := m.selectedTaskIdx()
	if idx < 0 {
		return
	}
	if err := m.copyText(m.file.Tasks[idx].Raw); err != nil {
		m.setStatus(fmt.Sprintf("コピー失敗: %v", err), true)
		return
	}
	m.setStatus("タスクをコピーしました", false)
}

// toggleComplete はカーソル位置のタスクの完了状態を切り替え、ファイルに保存する。
// 完了マーク (x + 完了日) を付与/除去するだけで、行は todo.txt に残る。
func (m *Model) toggleComplete() {
	idx := m.selectedTaskIdx()
	if idx < 0 {
		return
	}
	if !m.guardExternalChange() {
		return
	}

	t := &m.file.Tasks[idx]
	wasCompleted := t.Completed
	if wasCompleted {
		t.Uncomplete()
	} else {
		t.Complete(time.Now(), todotxt.CompleteOptions{})
	}

	if err := m.file.Save(); err != nil {
		m.setStatus(fmt.Sprintf("保存失敗: %v", err), true)
		return
	}
	m.rebuild()
	if wasCompleted {
		m.setStatus("未完了に戻しました", false)
	} else {
		m.setStatus("完了にしました  (a でアーカイブ)", false)
	}
}

// deleteTask はカーソル位置のタスクを todo.txt から削除する。
// アーカイブ (a) と違い archive.txt には残らないため、直前の 1 件だけ原文と位置を
// 保持し、u で元の位置に復元できるようにする。
func (m *Model) deleteTask() {
	idx := m.selectedTaskIdx()
	if idx < 0 {
		return
	}
	if !m.guardExternalChange() {
		return
	}

	raw, err := m.file.Delete(idx)
	if err != nil {
		m.setStatus(fmt.Sprintf("削除失敗: %v", err), true)
		return
	}
	m.lastOp = undoDelete
	m.lastDelete = raw
	m.lastDeleteIdx = idx
	m.rebuild() // カーソルは同じ序数に留まる = 次のタスクを指す
	m.setStatus("タスクを削除しました  (u で取消)", false)
}

// archive は完了済みタスクをすべて archive.txt へ「行そのまま」移動する。
func (m *Model) archive() {
	var idxs []int
	for i, t := range m.file.Tasks {
		if t.Completed {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) == 0 {
		m.setStatus("アーカイブする完了タスクがありません", true)
		return
	}
	if !m.guardExternalChange() {
		return
	}

	archived, err := m.file.Archive(idxs...)
	if err != nil {
		m.setStatus(fmt.Sprintf("アーカイブ失敗: %v", err), true)
		return
	}
	m.lastOp = undoArchive
	m.lastArchive = archived
	m.rebuild()
	m.setStatus(fmt.Sprintf("%d件をアーカイブしました  (u で取消)", len(archived)), false)
}

// guardExternalChange は書き込み前に外部変更を検知する。
// 変更があれば警告して false を返す（呼び出し側は処理を中断する）。
func (m *Model) guardExternalChange() bool {
	mod, err := m.file.ExternallyModified()
	if err != nil {
		m.setStatus(fmt.Sprintf("状態確認に失敗: %v", err), true)
		return false
	}
	if mod {
		m.setStatus("ファイルが外部で変更されています。r で再読込してください", true)
		return false
	}
	return true
}

// undo は直前のアーカイブまたは削除を取り消す。
// どちらも todo.txt を書き戻すため、先に外部変更を検知する。
func (m *Model) undo() {
	if m.lastOp == undoNone {
		m.setStatus("取り消せる操作がありません", true)
		return
	}
	if !m.guardExternalChange() {
		return
	}
	if m.lastOp == undoDelete {
		m.undoDelete()
		return
	}
	m.undoArchive()
}

// undoArchive は直前のアーカイブを取り消す。復元行は todo.txt の末尾に戻る。
func (m *Model) undoArchive() {
	if err := m.file.UndoArchive(m.lastArchive); err != nil {
		m.setStatus(fmt.Sprintf("取消失敗: %v", err), true)
		return
	}
	m.lastOp = undoNone
	m.lastArchive = nil
	m.rebuild()
	m.cursorToEdge(1) // 復元タスクは末尾に戻る
	m.setStatus("アーカイブを取り消しました", false)
}

// undoDelete は直前の削除を取り消し、元の位置に行を戻す。
// 削除後に他の行を追加/削除していた場合、位置は目安であり原文の復元のみを保証する。
func (m *Model) undoDelete() {
	idx := m.lastDeleteIdx
	if err := m.file.Insert(idx, m.lastDelete); err != nil {
		m.setStatus(fmt.Sprintf("取消失敗: %v", err), true)
		return
	}
	m.lastOp = undoNone
	m.lastDelete = ""
	m.rebuild()
	if !m.cursorToTask(idx) {
		m.setStatus("削除を取り消しました  (フィルタにより非表示)", false)
		return
	}
	m.setStatus("削除を取り消しました", false)
}

// openEditor は表示中のファイルを外部エディタで開く tea.Cmd を返す。
// TUI は一時的に中断され、エディタ終了後に editorFinishedMsg が届く。
func (m Model) openEditor() tea.Cmd {
	name, args := editorCommand(os.Getenv("EDITOR"), os.Getenv("VISUAL"), m.file.Path)
	c := exec.Command(name, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// editorCommand は起動するエディタのコマンド名と引数を決める。
// $EDITOR を最優先し、無ければ $VISUAL、それも無ければ vi にフォールバックする。
// 環境変数が引数を含む場合 (例: "code --wait") も分割して扱う。
func editorCommand(editorEnv, visualEnv, path string) (string, []string) {
	editor := editorEnv
	if editor == "" {
		editor = visualEnv
	}
	if editor == "" {
		editor = "vi"
	}
	fields := strings.Fields(editor)
	return fields[0], append(fields[1:], path)
}

// reload はファイルを読み直す。
func (m *Model) reload() {
	nf, err := store.Load(m.file.Path)
	if err != nil {
		m.setStatus(fmt.Sprintf("再読込失敗: %v", err), true)
		return
	}
	m.file = nf
	m.lastOp = undoNone
	m.lastArchive = nil
	m.lastDelete = ""
	m.rebuild()
	m.setStatus("再読込しました", false)
}

func (m *Model) setStatus(s string, isErr bool) {
	m.status = s
	m.statusErr = isErr
}

func (m *Model) clearStatus() {
	m.status = ""
	m.statusErr = false
}

func sortLabel(k filter.SortKey) string {
	if k == filter.SortCompletion {
		return "completed"
	}
	return "priority"
}

func groupLabel(k filter.GroupKey) string {
	switch k {
	case filter.GroupByProject:
		return "project"
	case filter.GroupByContext:
		return "context"
	case filter.GroupByPriority:
		return "priority"
	default:
		return "flat"
	}
}
