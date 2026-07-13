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
		if m.mode == modeFilter {
			return m.updateFilterMode(msg)
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
		m.moveCursor(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.Top):
		m.cursorToEdge(-1)
	case key.Matches(msg, m.keys.Bottom):
		m.cursorToEdge(1)

	case key.Matches(msg, m.keys.Group):
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

	case key.Matches(msg, m.keys.Undo):
		m.undo()

	case key.Matches(msg, m.keys.Complete):
		m.toggleComplete()

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

// undo は直前のアーカイブを取り消す。
func (m *Model) undo() {
	if len(m.lastArchive) == 0 {
		m.setStatus("取り消せるアーカイブがありません", true)
		return
	}
	if err := m.file.UndoArchive(m.lastArchive); err != nil {
		m.setStatus(fmt.Sprintf("取消失敗: %v", err), true)
		return
	}
	m.lastArchive = nil
	m.rebuild()
	m.cursorToEdge(1) // 復元タスクは末尾に戻る
	m.setStatus("アーカイブを取り消しました", false)
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
	m.lastArchive = nil
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
