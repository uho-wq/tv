// Package ui は Bubble Tea による TUI を実装する。
package ui

import (
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/uho-wq/tv/internal/filter"
	"github.com/uho-wq/tv/internal/store"
)

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeEdit
)

// undoOp は u で取り消せる「直前の操作」の種類。
// アーカイブと削除で復元手順が異なるため、どちらが直近かを保持する。
type undoOp int

const (
	undoNone undoOp = iota
	undoArchive
	undoDelete
)

// paneFocusArea は pane 表示中のフォーカス位置（サイドバー or タスクリスト）。
type paneFocusArea int

const (
	paneSidebar paneFocusArea = iota
	paneList
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowTask
)

// row は画面に表示する 1 行（グループ見出し or タスク）。
type row struct {
	kind    rowKind
	title   string // 見出し用
	count   int    // 見出し用: グループ件数
	taskIdx int    // タスク用: file.Tasks のインデックス
}

// Model はアプリケーションの全状態を保持する。
type Model struct {
	file     *store.File
	criteria filter.Criteria
	groupKey filter.GroupKey
	sortKey  filter.SortKey

	rows   []row // 表示行（見出し + タスク）
	cursor int   // rows 内のインデックス（常にタスク行を指す。タスクが無ければ -1）
	offset int   // 表示開始の rows インデックス（スクロール）

	pane       bool           // 2 ペイン表示（左: グループ, 右: タスク）
	paneFocus  paneFocusArea  // フォーカス中のペイン
	paneGroups []filter.Group // サイドバーのグループ一覧
	paneSel    int            // 選択中グループの paneGroups インデックス
	paneTitle  string         // 選択中グループの見出し（rebuild 後の追従用）
	paneOffset int            // サイドバーの表示開始インデックス（スクロール）

	width  int
	height int

	mode      mode
	input     textinput.Model
	editInput textinput.Model
	editIdx   int // 編集中タスクの file.Tasks インデックス

	status    string
	statusErr bool

	lastOp        undoOp   // 直前の取り消し可能な操作
	lastArchive   []string // undoArchive: 直前のアーカイブ行
	lastDelete    string   // undoDelete: 直前に削除した行の原文
	lastDeleteIdx int      // undoDelete: 削除前の file.Tasks インデックス

	copyText func(string) error // クリップボード書き込み（テストで差し替え可能）

	help bool
	keys keyMap
	st   styles
}

// New は読み込んだファイルから Model を構築する。
func New(f *store.File) Model {
	ti := textinput.New()
	ti.Placeholder = "+project @context (A) keyword"
	ti.Prompt = "filter> "

	ei := textinput.New()
	ei.Prompt = "edit> "

	m := Model{
		file:      f,
		groupKey:  filter.GroupFlat,
		sortKey:   filter.SortPriority,
		keys:      defaultKeys(),
		st:        newStyles(),
		input:     ti,
		editInput: ei,
		cursor:    -1,
		copyText:  clipboard.WriteAll,
	}
	m.rebuild()
	return m
}

// Init は Bubble Tea のエントリ。初期コマンドは不要。
func (m Model) Init() tea.Cmd { return nil }

// rebuild は criteria / groupKey から表示行を再構築する。
//
// カーソルは「タスク行の序数（先頭から何番目のタスクか）」で保持する。
// rows インデックスで保持すると、グルーピング切替で見出し行の数が変わった際に
// 別のタスクを指してしまうため。アーカイブ時はタスクが 1 つ減るので、同じ序数が
// 自然と「次のタスク」を指し、カーソルがその場に留まる。
func (m *Model) rebuild() {
	if m.pane {
		m.rebuildPane()
		return
	}

	ord := m.cursorOrdinal()

	indices := filter.Apply(m.file.Tasks, m.criteria)
	groups := filter.GroupBy(m.file.Tasks, indices, m.groupKey, m.sortKey)

	rows := make([]row, 0, len(indices)+len(groups))
	for _, g := range groups {
		if g.Title != "" { // flat の場合は見出しなし
			rows = append(rows, row{kind: rowHeader, title: g.Title, count: len(g.Indices)})
		}
		for _, idx := range g.Indices {
			rows = append(rows, row{kind: rowTask, taskIdx: idx})
		}
	}
	m.rows = rows
	m.cursor = m.taskRowByOrdinal(ord)
	m.ensureVisible()
}

// rebuildPane は pane 表示用にサイドバーのグループと表示行を再構築する。
//
// グループ選択は「見出し」で追従する。編集や完了トグルでグループの並びが
// 変わっても、同じ見出しのグループを見続けられるようにするため。
// 選択していたグループが消えた場合は位置をクランプし、カーソルは先頭に戻す。
func (m *Model) rebuildPane() {
	ord := m.cursorOrdinal()

	indices := filter.Apply(m.file.Tasks, m.criteria)
	m.paneGroups = filter.PaneGroups(m.file.Tasks, indices, m.sortKey)

	sel := -1
	for i, g := range m.paneGroups {
		if g.Title == m.paneTitle {
			sel = i
			break
		}
	}
	if sel < 0 {
		sel = min(m.paneSel, len(m.paneGroups)-1)
		sel = max(sel, 0)
		ord = 0 // 別グループに切り替わるためカーソルは先頭へ
	}
	m.paneSel = sel
	m.paneTitle = ""
	if len(m.paneGroups) > 0 {
		m.paneTitle = m.paneGroups[sel].Title
	}

	m.rows = m.paneRows(sel)
	m.cursor = m.taskRowByOrdinal(ord)
	m.ensureVisible()
	m.ensurePaneVisible()
}

// paneRows は paneGroups[sel] のタスク行を作る（pane 表示に見出し行は無い）。
func (m Model) paneRows(sel int) []row {
	if sel < 0 || sel >= len(m.paneGroups) {
		return nil
	}
	g := m.paneGroups[sel]
	rows := make([]row, 0, len(g.Indices))
	for _, idx := range g.Indices {
		rows = append(rows, row{kind: rowTask, taskIdx: idx})
	}
	return rows
}

// paneSidebarActive は pane 表示中でサイドバーにフォーカスがあるか返す。
func (m Model) paneSidebarActive() bool {
	return m.pane && m.paneFocus == paneSidebar
}

// paneMove はサイドバーのグループ選択を dir(+1/-1) へ動かす。
func (m *Model) paneMove(dir int) {
	sel := m.paneSel + dir
	if sel < 0 || sel >= len(m.paneGroups) {
		return
	}
	m.paneSelect(sel)
}

// paneSelect は sel 番目のグループを選択し、タスクリストを差し替える。
// カーソルは新しいグループの先頭タスクに移る。
func (m *Model) paneSelect(sel int) {
	if sel < 0 || sel >= len(m.paneGroups) || sel == m.paneSel {
		return
	}
	m.paneSel = sel
	m.paneTitle = m.paneGroups[sel].Title
	m.rows = m.paneRows(sel)
	m.cursor = m.firstTaskRow()
	m.offset = 0
	m.ensurePaneVisible()
}

// ensurePaneVisible はサイドバーの選択が表示範囲に入るよう paneOffset を調整する。
func (m *Model) ensurePaneVisible() {
	bh := m.bodyHeight()
	if m.paneSel < m.paneOffset {
		m.paneOffset = m.paneSel
	}
	if m.paneSel >= m.paneOffset+bh {
		m.paneOffset = m.paneSel - bh + 1
	}
	if m.paneOffset < 0 {
		m.paneOffset = 0
	}
}

// firstTaskRow は最初のタスク行の rows インデックスを返す（無ければ -1）。
func (m Model) firstTaskRow() int {
	for i, r := range m.rows {
		if r.kind == rowTask {
			return i
		}
	}
	return -1
}

// cursorOrdinal はカーソルが指すタスクが「先頭から何番目のタスク行か」を返す
// （0 始まり）。カーソルが無効なら -1。
func (m Model) cursorOrdinal() int {
	if m.cursor < 0 {
		return -1
	}
	ord := 0
	for i := 0; i < m.cursor && i < len(m.rows); i++ {
		if m.rows[i].kind == rowTask {
			ord++
		}
	}
	return ord
}

// taskRowByOrdinal は ord 番目のタスク行の rows インデックスを返す。
// ord が範囲を超える場合は最後のタスク行に丸める。タスクが無ければ -1。
func (m Model) taskRowByOrdinal(ord int) int {
	if ord < 0 {
		return m.firstTaskRow()
	}
	count, last := 0, -1
	for i, r := range m.rows {
		if r.kind == rowTask {
			last = i
			if count == ord {
				return i
			}
			count++
		}
	}
	return last
}

// moveCursor は方向 dir(+1/-1) に最も近いタスク行へカーソルを動かす。
func (m *Model) moveCursor(dir int) {
	if m.cursor < 0 {
		return
	}
	for i := m.cursor + dir; i >= 0 && i < len(m.rows); i += dir {
		if m.rows[i].kind == rowTask {
			m.cursor = i
			m.ensureVisible()
			return
		}
	}
}

// cursorToEdge は先頭(dir<0) または末尾(dir>0) のタスク行へ移動する。
func (m *Model) cursorToEdge(dir int) {
	if dir < 0 {
		m.cursor = m.firstTaskRow()
	} else {
		for i := len(m.rows) - 1; i >= 0; i-- {
			if m.rows[i].kind == rowTask {
				m.cursor = i
				break
			}
		}
	}
	m.ensureVisible()
}

// cursorToTask は file.Tasks インデックスが idx のタスク行へカーソルを移す。
// 表示中に見つかれば true を返す（フィルタ等で非表示なら false でカーソルは動かさない）。
func (m *Model) cursorToTask(idx int) bool {
	for i, r := range m.rows {
		if r.kind == rowTask && r.taskIdx == idx {
			m.cursor = i
			m.ensureVisible()
			return true
		}
	}
	return false
}

// selectedTaskIdx はカーソル位置のタスクの file.Tasks インデックスを返す（無ければ -1）。
func (m Model) selectedTaskIdx() int {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return -1
	}
	r := m.rows[m.cursor]
	if r.kind != rowTask {
		return -1
	}
	return r.taskIdx
}

// bodyHeight は本体（リスト）に使える行数を返す。
func (m Model) bodyHeight() int {
	h := m.height - 2 // header 1 行 + footer 1 行
	if h < 1 {
		return 1
	}
	return h
}

// ensureVisible はカーソルが表示範囲に入るよう offset を調整する。
func (m *Model) ensureVisible() {
	if m.cursor < 0 {
		m.offset = 0
		return
	}
	bh := m.bodyHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	// カーソルが表示先頭のタスク行で、直上がグループ見出しなら見出しも表示に含める。
	if m.cursor == m.offset && m.cursor > 0 && m.rows[m.cursor-1].kind == rowHeader {
		m.offset = m.cursor - 1
	}
	if m.cursor >= m.offset+bh {
		m.offset = m.cursor - bh + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// taskCount は現在表示中のタスク数（重複表示は除いた実数）を返す。
func (m Model) taskCount() int {
	return len(filter.Apply(m.file.Tasks, m.criteria))
}
