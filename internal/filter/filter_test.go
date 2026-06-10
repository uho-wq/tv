package filter

import (
	"testing"

	"github.com/uho-wq/tv/internal/todotxt"
)

func parse(lines ...string) []todotxt.Task {
	tasks := make([]todotxt.Task, len(lines))
	for i, l := range lines {
		tasks[i] = todotxt.ParseLine(l, i)
	}
	return tasks
}

func TestApply(t *testing.T) {
	tasks := parse(
		"(A) write report +Work @office",
		"buy milk +Home @errand",
		"x 2026-06-08 done thing +Work",
		"call client +Work @phone",
		"   ", // blank
	)

	tests := []struct {
		name string
		c    Criteria
		want []int
	}{
		{"all incomplete", Criteria{}, []int{0, 1, 3}},
		{"with completed", Criteria{ShowCompleted: true}, []int{0, 1, 2, 3}},
		{"project Work", Criteria{Projects: []string{"Work"}}, []int{0, 3}},
		{"context office", Criteria{Contexts: []string{"office"}}, []int{0}},
		{"priority A", Criteria{Priorities: []todotxt.Priority{'A'}}, []int{0}},
		{"query milk", Criteria{Query: "milk"}, []int{1}},
		{"and project+context", Criteria{Projects: []string{"Work"}, Contexts: []string{"phone"}}, []int{3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply(tasks, tt.c)
			if !eqInts(got, tt.want) {
				t.Errorf("Apply = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseQuery(t *testing.T) {
	c := ParseQuery("+Work @phone (A) buy some milk")
	if len(c.Projects) != 1 || c.Projects[0] != "Work" {
		t.Errorf("Projects = %v", c.Projects)
	}
	if len(c.Contexts) != 1 || c.Contexts[0] != "phone" {
		t.Errorf("Contexts = %v", c.Contexts)
	}
	if len(c.Priorities) != 1 || c.Priorities[0] != 'A' {
		t.Errorf("Priorities = %v", c.Priorities)
	}
	if c.Query != "buy some milk" {
		t.Errorf("Query = %q", c.Query)
	}
}

func TestGroupByProject(t *testing.T) {
	tasks := parse(
		"(A) a +Work",
		"b +Home",
		"c +Work +Home", // 両方に出る
		"d",             // no project
	)
	indices := Apply(tasks, Criteria{})
	groups := GroupBy(tasks, indices, GroupByProject)

	// +Home, +Work が先（アルファベット順）、(no project) が最後
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3: %+v", len(groups), groups)
	}
	if groups[0].Title != "+Home" || groups[1].Title != "+Work" || groups[2].Title != "(no project)" {
		t.Errorf("group titles = %q, %q, %q", groups[0].Title, groups[1].Title, groups[2].Title)
	}
	// +Work には a(0) と c(2)
	if !eqInts(groups[1].Indices, []int{0, 2}) {
		t.Errorf("+Work indices = %v, want [0 2]", groups[1].Indices)
	}
}

func TestGroupFlatSort(t *testing.T) {
	tasks := parse(
		"plain task",
		"(B) medium",
		"(A) high",
	)
	indices := Apply(tasks, Criteria{})
	groups := GroupBy(tasks, indices, GroupFlat)
	if len(groups) != 1 {
		t.Fatalf("flat should be 1 group, got %d", len(groups))
	}
	// 優先度順: A(2), B(1), なし(0)
	if !eqInts(groups[0].Indices, []int{2, 1, 0}) {
		t.Errorf("flat sorted = %v, want [2 1 0]", groups[0].Indices)
	}
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
