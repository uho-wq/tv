package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/uho-wq/tv/internal/filter"
	"github.com/uho-wq/tv/internal/todotxt"
)

func homeDir() (string, error) { return os.UserHomeDir() }

// View はモデルを文字列に描画する。
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading…"
	}

	header := m.renderHeader()
	var body string
	if m.help {
		body = m.renderHelp()
	} else {
		body = m.renderBody()
	}
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) renderHeader() string {
	left := m.st.header.Render("tv")

	meta := []string{
		shortenPath(m.file.Path),
		fmt.Sprintf("%d tasks", m.taskCount()),
	}
	if m.pane {
		meta = append(meta, "pane")
	} else if m.groupKey != filter.GroupFlat {
		meta = append(meta, "group:"+groupLabel(m.groupKey))
	}
	if m.sortKey != filter.SortPriority {
		meta = append(meta, "sort:"+sortLabel(m.sortKey))
	}
	if !m.criteria.Empty() {
		meta = append(meta, "filter:"+m.criteriaLabel())
	}
	if m.criteria.ShowCompleted {
		meta = append(meta, "+完了")
	}
	right := m.st.headerMeta.Render("  " + strings.Join(meta, "  ·  "))

	line := left + right
	return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
}

func (m Model) renderBody() string {
	if m.pane {
		return m.renderPaneBody()
	}

	bh := m.bodyHeight()
	clip := lipgloss.NewStyle().MaxWidth(m.width)

	if len(m.rows) == 0 || m.cursor < 0 {
		empty := "  (表示するタスクがありません)"
		lines := []string{empty}
		return padLines(lines, bh)
	}

	end := min(m.offset+bh, len(m.rows))
	lines := make([]string, 0, bh)
	for i := m.offset; i < end; i++ {
		r := m.rows[i]
		switch r.kind {
		case rowHeader:
			lines = append(lines, clip.Render(m.st.groupTitle.Render(fmt.Sprintf("%s (%d)", r.title, r.count))))
		case rowTask:
			selected := i == m.cursor
			lines = append(lines, clip.Render(m.renderTask(r.taskIdx, selected)))
		}
	}
	return padLines(lines, bh)
}

// renderPaneBody は 2 ペイン表示（左: グループ, 右: タスク）を描画する。
func (m Model) renderPaneBody() string {
	bh := m.bodyHeight()
	sw := m.sidebarWidth()
	lw := m.width - sw - 3 // セパレータ " │ " の分
	if lw < 1 {
		lw = 1
	}

	side := m.renderSidebarLines(sw, bh)
	list := m.renderListLines(lw, bh)

	sep := m.st.paneSep.Render("│")
	lines := make([]string, bh)
	for i := range bh {
		lines[i] = side[i] + " " + sep + " " + list[i]
	}
	return strings.Join(lines, "\n")
}

// sidebarWidth はサイドバーの表示幅を返す。画面の 1/4 を基本に上下限を設ける。
func (m Model) sidebarWidth() int {
	w := m.width / 4
	w = max(w, 14)
	w = min(w, 28)
	// 端末が極端に狭い場合はさらに半分へ。
	if w > m.width/2 {
		w = m.width / 2
	}
	return max(w, 1)
}

// renderSidebarLines はサイドバー（グループ一覧）を bh 行ぶん描画する。
func (m Model) renderSidebarLines(sw, bh int) []string {
	pad := lipgloss.NewStyle().Width(sw).MaxWidth(sw)
	lines := make([]string, 0, bh)

	if len(m.paneGroups) == 0 {
		lines = append(lines, pad.Render("  (グループなし)"))
	}
	end := min(m.paneOffset+bh, len(m.paneGroups))
	for i := m.paneOffset; i < end; i++ {
		g := m.paneGroups[i]
		label := fmt.Sprintf("%s (%d)", g.Title, len(g.Indices))
		var line string
		switch {
		case i == m.paneSel && m.paneFocus == paneSidebar:
			line = m.st.cursor.Render("❯ " + label)
		case i == m.paneSel:
			line = m.st.groupTitle.Render("❯ " + label)
		default:
			line = "  " + m.paneEntryStyle(g.Title).Render(label)
		}
		lines = append(lines, pad.Render(line))
	}
	for len(lines) < bh {
		lines = append(lines, pad.Render(""))
	}
	return lines
}

// paneEntryStyle はサイドバー項目の色を見出しの種類で選ぶ。
// タスク本文中の +project / @context の色と揃え、一目で区別できるようにする。
func (m Model) paneEntryStyle(title string) lipgloss.Style {
	switch {
	case strings.HasPrefix(title, "+"):
		return m.st.project
	case strings.HasPrefix(title, "@"):
		return m.st.context
	default:
		return m.st.meta
	}
}

// renderListLines は選択中グループのタスクリストを bh 行ぶん描画する。
func (m Model) renderListLines(lw, bh int) []string {
	clip := lipgloss.NewStyle().MaxWidth(lw)
	lines := make([]string, 0, bh)

	if len(m.rows) == 0 || m.cursor < 0 {
		lines = append(lines, "  (表示するタスクがありません)")
	}
	end := min(m.offset+bh, len(m.rows))
	for i := m.offset; i < end; i++ {
		selected := i == m.cursor
		lines = append(lines, clip.Render(m.renderTask(m.rows[i].taskIdx, selected)))
	}
	for len(lines) < bh {
		lines = append(lines, "")
	}
	return lines
}

func (m Model) renderTask(idx int, selected bool) string {
	t := m.file.Tasks[idx]

	if selected {
		// 選択行は内側の色付けと衝突しないよう、原文を反転表示する。
		return m.st.cursor.Render("❯ " + t.Raw)
	}

	if t.Completed {
		return "  " + m.st.completed.Render(t.Raw)
	}
	return "  " + m.colorize(t)
}

// colorize は未完了タスクの原文をトークン単位で色付けする。
func (m Model) colorize(t todotxt.Task) string {
	var b strings.Builder
	first := true
	for tok := range strings.FieldsSeq(t.Raw) {
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(m.styleToken(tok))
	}
	return b.String()
}

func (m Model) styleToken(tok string) string {
	switch {
	case len(tok) == 3 && tok[0] == '(' && tok[2] == ')' && tok[1] >= 'A' && tok[1] <= 'Z':
		return m.st.priorityStyle(todotxt.Priority(tok[1])).Render(tok)
	case len(tok) > 1 && tok[0] == '+':
		return m.st.project.Render(tok)
	case len(tok) > 1 && tok[0] == '@':
		return m.st.context.Render(tok)
	case isKeyValue(tok):
		return m.st.meta.Render(tok)
	default:
		return tok
	}
}

func (m Model) renderFooter() string {
	clip := lipgloss.NewStyle().MaxWidth(m.width)

	if m.mode == modeFilter {
		return clip.Render(m.input.View())
	}
	if m.mode == modeEdit {
		return clip.Render(m.editInput.View())
	}
	if m.status != "" {
		st := m.st.status
		if m.statusErr {
			st = m.st.statusErr
		}
		return clip.Render(st.Render(m.status))
	}

	keybar := "? ヘルプ"
	return clip.Render(m.st.footer.Render(keybar))
}

func (m Model) renderHelp() string {
	bh := m.bodyHeight()
	help := []string{
		"",
		"  キーバインド",
		"  ─────────────────────────────",
		"  ↑/k, ↓/j      カーソル移動",
		"  g / G         先頭 / 末尾へ",
		"  x / Space     完了トグル (x + 完了日を付与/除去。行は残る)",
		"  i / Enter     カーソル行をその場で編集 (enter 保存 / esc 取消)",
		"  o             新規タスクを末尾に追加 (enter 保存 / esc 取消)",
		"  Ctrl+O        カーソル行の原文をクリップボードへコピー",
		"  d             カーソル行を削除 (u で取消)",
		"  a             完了済みタスクを archive.txt へ一括移動",
		"  u             直前の削除/アーカイブを取り消し",
		"  f / /         フィルタ入力 (+proj @ctx (A) keyword)",
		"  esc           フィルタ解除",
		"  p             pane 表示切替 (左: +project / project なしは @context 別)",
		"  h / l         pane 間のフォーカス移動 (pane 表示中は tab でも切替)",
		"  tab           グルーピング切替 (flat→project→context→priority)",
		"  s             ソート切替 (priority⇔completed: 完了日の新しい順)",
		"  c             完了タスクの表示/非表示",
		"  r             ファイル再読込",
		"  e             $EDITOR でファイルを開く (終了後に再読込)",
		"  ?             このヘルプを閉じる",
		"  q / Ctrl+C    終了",
		"",
		"  完了 (x) … todo.txt 内で完了マークを付ける/外す。",
		"  アーカイブ (a) … 完了行を原文のまま archive.txt へ移動する。",
		"  削除 (d) … 行を消す。archive.txt には残らず、直後の u でのみ復元。",
	}
	return padLines(help, bh)
}

func (m Model) criteriaLabel() string {
	var parts []string
	for _, p := range m.criteria.Projects {
		parts = append(parts, "+"+p)
	}
	for _, c := range m.criteria.Contexts {
		parts = append(parts, "@"+c)
	}
	for _, pr := range m.criteria.Priorities {
		parts = append(parts, pr.String())
	}
	if m.criteria.Query != "" {
		parts = append(parts, "\""+m.criteria.Query+"\"")
	}
	return strings.Join(parts, " ")
}

// --- ヘルパ ---

func isKeyValue(tok string) bool {
	idx := strings.IndexByte(tok, ':')
	if idx <= 0 || idx >= len(tok)-1 {
		return false
	}
	return !strings.ContainsRune(tok[idx+1:], ':')
}

// padLines は lines を高さ n に切り詰め/空行で埋める。
func padLines(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func shortenPath(p string) string {
	if home, err := homeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
