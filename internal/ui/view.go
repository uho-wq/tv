package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

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
		"group:" + groupLabel(m.groupKey),
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
	if m.status != "" {
		st := m.st.status
		if m.statusErr {
			st = m.st.statusErr
		}
		return clip.Render(st.Render(m.status))
	}

	keybar := "j/k 移動 · x 完了 · a アーカイブ · f フィルタ · tab グループ · c 完了表示 · u 取消 · r 再読込 · e 編集 · ? ヘルプ · q 終了"
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
		"  a             完了済みタスクを archive.txt へ一括移動",
		"  u             直前のアーカイブを取り消し",
		"  f / /         フィルタ入力 (+proj @ctx (A) keyword)",
		"  esc           フィルタ解除",
		"  tab           グルーピング切替 (project→context→priority→flat)",
		"  c             完了タスクの表示/非表示",
		"  r             ファイル再読込",
		"  e             $EDITOR でファイルを開く (終了後に再読込)",
		"  ?             このヘルプを閉じる",
		"  q / Ctrl+C    終了",
		"",
		"  完了 (x) … todo.txt 内で完了マークを付ける/外す。",
		"  アーカイブ (a) … 完了済みの行を原文のまま archive.txt へ",
		"      移動し、todo.txt からは削除する。",
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
