package filter

import (
	"testing"

	"github.com/uho-wq/tv/internal/todotxt"
)

func TestSortCheckRealData(t *testing.T) {
	lines := []string{
		"x 2026-06-09 @today +server みっつさんPRレビュー",
		"x 2026-06-11 @weekly +etl 課金ログにオンボ経路追加",
		"x 2026-06-10 @today +others 勉強会資料",
		"x 2026-06-11 @today +others SE4勉強会",
		"x 2026-06-09 @today +onbo 野村くん、井上くんに連絡",
	}
	tasks := make([]todotxt.Task, len(lines))
	for i, l := range lines {
		tasks[i] = todotxt.ParseLine(l, i)
		t.Logf("task %d: completed=%v date=%v", i, tasks[i].Completed, tasks[i].CompletionDate)
	}
	indices := Apply(tasks, Criteria{ShowCompleted: true})
	groups := GroupBy(tasks, indices, GroupFlat, SortCompletion)
	t.Logf("sorted: %v", groups[0].Indices)
	want := []int{1, 3, 2, 0, 4}
	if !eqInts(groups[0].Indices, want) {
		t.Errorf("got %v, want %v", groups[0].Indices, want)
	}
}
