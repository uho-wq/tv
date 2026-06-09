package todotxt

import "testing"

func TestComplete(t *testing.T) {
	today := mustDate(t, "2026-06-09")

	tests := []struct {
		name string
		line string
		opts CompleteOptions
		want string
	}{
		{
			name: "priority and creation date",
			line: "(A) 2026-06-01 買い物 +Home",
			want: "x 2026-06-09 2026-06-01 買い物 +Home",
		},
		{
			name: "no priority no date",
			line: "牛乳を買う",
			want: "x 2026-06-09 牛乳を買う",
		},
		{
			name: "preserve priority as kv",
			line: "(B) review +Work",
			opts: CompleteOptions{PreservePriorityAsKV: true},
			want: "x 2026-06-09 review +Work pri:B",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ParseLine(tt.line, 0)
			task.Complete(today, tt.opts)
			if !task.Completed {
				t.Errorf("not marked completed")
			}
			if task.Raw != tt.want {
				t.Errorf("Raw = %q, want %q", task.Raw, tt.want)
			}
		})
	}
}

func TestUncomplete(t *testing.T) {
	// pri: 退避ありなら優先度が復元される（往復）
	task := ParseLine("(A) 2026-06-01 買い物 +Home", 0)
	today := mustDate(t, "2026-06-09")
	task.Complete(today, CompleteOptions{PreservePriorityAsKV: true})
	if task.Raw != "x 2026-06-09 2026-06-01 買い物 +Home pri:A" {
		t.Fatalf("after complete: %q", task.Raw)
	}
	task.Uncomplete()
	if task.Completed {
		t.Errorf("still completed")
	}
	if task.Raw != "(A) 2026-06-01 買い物 +Home" {
		t.Errorf("Raw = %q, want round-trip to original", task.Raw)
	}
}

func TestUncompleteNoPriority(t *testing.T) {
	task := ParseLine("x 2026-06-08 古紙を出す", 0)
	task.Uncomplete()
	if task.Completed {
		t.Errorf("still completed")
	}
	if task.Raw != "古紙を出す" {
		t.Errorf("Raw = %q, want %q", task.Raw, "古紙を出す")
	}
}

func TestCompleteIdempotent(t *testing.T) {
	task := ParseLine("x 2026-06-08 done", 0)
	before := task.Raw
	task.Complete(mustDate(t, "2026-06-09"), CompleteOptions{})
	if task.Raw != before {
		t.Errorf("completing a completed task changed it: %q", task.Raw)
	}
}
