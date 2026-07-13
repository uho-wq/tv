package filter

import (
	"sort"

	"github.com/uho-wq/tv/internal/todotxt"
)

// GroupKey はグルーピングの軸を表す。
type GroupKey string

const (
	GroupByProject  GroupKey = "project"
	GroupByContext  GroupKey = "context"
	GroupByPriority GroupKey = "priority"
	GroupFlat       GroupKey = "flat"
)

// Next は次のグルーピング軸を返す（UI のトグル用）。
func (k GroupKey) Next() GroupKey {
	switch k {
	case GroupFlat:
		return GroupByProject
	case GroupByProject:
		return GroupByContext
	case GroupByContext:
		return GroupByPriority
	default:
		return GroupFlat
	}
}

// SortKey はグループ内のタスクの並び順を表す。
type SortKey string

const (
	SortPriority   SortKey = "priority"   // 未完了→優先度→原文順（デフォルト）
	SortCompletion SortKey = "completion" // 完了日の新しい順
)

// Next は次のソート軸を返す（UI のトグル用）。
func (k SortKey) Next() SortKey {
	if k == SortPriority {
		return SortCompletion
	}
	return SortPriority
}

// Group は見出しと、それに属するタスクのインデックス群を持つ。
type Group struct {
	Title   string
	Indices []int // tasks 内のインデックス
}

const (
	noProject  = "(no project)"
	noContext  = "(no context)"
	noPriority = "(no priority)"
)

// GroupBy は indices で示されるタスクを key の軸でグルーピングする。
// グループの並びとグループ内のタスク順は安定にソートされる。
//
// project / context を複数持つタスクは、該当する各グループに重複して現れる
// （俯瞰しやすくするため）。
func GroupBy(tasks []todotxt.Task, indices []int, key GroupKey, sortKey SortKey) []Group {
	if key == GroupFlat {
		g := Group{Title: "", Indices: append([]int(nil), indices...)}
		sortIndices(tasks, g.Indices, sortKey)
		return []Group{g}
	}

	buckets := map[string][]int{}
	var order []string // 出現順を保ちつつ、後でタイトルソート
	add := func(title string, idx int) {
		if _, ok := buckets[title]; !ok {
			order = append(order, title)
		}
		buckets[title] = append(buckets[title], idx)
	}

	for _, i := range indices {
		t := tasks[i]
		switch key {
		case GroupByProject:
			if len(t.Projects) == 0 {
				add(noProject, i)
			}
			for _, p := range t.Projects {
				add("+"+p, i)
			}
		case GroupByContext:
			if len(t.Contexts) == 0 {
				add(noContext, i)
			}
			for _, c := range t.Contexts {
				add("@"+c, i)
			}
		case GroupByPriority:
			if t.Priority == todotxt.PriorityNone {
				add(noPriority, i)
			} else {
				add(t.Priority.String(), i)
			}
		}
	}

	// グループ見出しをソート。"(no ...)" は末尾に寄せる。
	sort.SliceStable(order, func(a, b int) bool {
		na := isNoGroup(order[a])
		nb := isNoGroup(order[b])
		if na != nb {
			return !na // no-group を後ろへ
		}
		return order[a] < order[b]
	})

	groups := make([]Group, 0, len(order))
	for _, title := range order {
		idxs := buckets[title]
		sortIndices(tasks, idxs, sortKey)
		groups = append(groups, Group{Title: title, Indices: idxs})
	}
	return groups
}

func isNoGroup(title string) bool {
	return title == noProject || title == noContext || title == noPriority
}

// sortIndices はグループ内のタスク順を sortKey に従って整える。
//
// SortPriority: 未完了を先、完了を後。次に優先度（A が上、なしは下）、最後に原文順。
// SortCompletion: 完了タスクを完了日の新しい順で先頭に。日付なしの完了は
// その後ろ、未完了は末尾に原文順で続く。
func sortIndices(tasks []todotxt.Task, idxs []int, sortKey SortKey) {
	if sortKey == SortCompletion {
		sortByCompletion(tasks, idxs)
		return
	}
	sort.SliceStable(idxs, func(a, b int) bool {
		ta, tb := tasks[idxs[a]], tasks[idxs[b]]
		if ta.Completed != tb.Completed {
			return !ta.Completed // 未完了が先
		}
		pa, pb := priorityRank(ta.Priority), priorityRank(tb.Priority)
		if pa != pb {
			return pa < pb
		}
		return idxs[a] < idxs[b] // 原文順
	})
}

func sortByCompletion(tasks []todotxt.Task, idxs []int) {
	sort.SliceStable(idxs, func(a, b int) bool {
		ta, tb := tasks[idxs[a]], tasks[idxs[b]]
		if ta.Completed != tb.Completed {
			return ta.Completed // 完了が先
		}
		da, db := ta.CompletionDate, tb.CompletionDate
		if (da != nil) != (db != nil) {
			return da != nil // 完了日ありが先
		}
		if da != nil && !da.Equal(*db) {
			return da.After(*db) // 新しい完了が上
		}
		return idxs[a] < idxs[b] // 原文順
	})
}

// priorityRank は優先度の並び順。A=0, B=1, ... 優先度なしは最大。
func priorityRank(p todotxt.Priority) int {
	if p == todotxt.PriorityNone {
		return 1 << 30
	}
	return int(p - 'A')
}
