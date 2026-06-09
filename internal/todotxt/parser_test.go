package todotxt

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return d
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want Task
	}{
		{
			name: "priority creation date project context",
			line: "(A) 2026-06-01 Call Mom +Family @phone",
			want: Task{
				Priority:    'A',
				Description: "Call Mom +Family @phone",
				Projects:    []string{"Family"},
				Contexts:    []string{"phone"},
			},
		},
		{
			name: "completed with completion and creation date",
			line: "x 2011-03-02 2011-03-01 Review Tim's pull request +Project @context",
			want: Task{
				Completed:   true,
				Description: "Review Tim's pull request +Project @context",
				Projects:    []string{"Project"},
				Contexts:    []string{"context"},
			},
		},
		{
			name: "completed with completion date only",
			line: "x 2026-06-08 古紙を出す",
			want: Task{
				Completed:   true,
				Description: "古紙を出す",
			},
		},
		{
			name: "metadata due",
			line: "Pay bills due:2026-06-30 @home",
			want: Task{
				Description: "Pay bills due:2026-06-30 @home",
				Contexts:    []string{"home"},
				Metadata:    map[string]string{"due": "2026-06-30"},
			},
		},
		{
			name: "plain description no metadata",
			line: "just a task",
			want: Task{Description: "just a task"},
		},
		{
			name: "url is not key value",
			line: "read https://example.com later",
			want: Task{Description: "read https://example.com later"},
		},
		{
			name: "blank line",
			line: "   ",
			want: Task{Blank: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLine(tt.line, 0)

			if got.Completed != tt.want.Completed {
				t.Errorf("Completed = %v, want %v", got.Completed, tt.want.Completed)
			}
			if got.Priority != tt.want.Priority {
				t.Errorf("Priority = %q, want %q", got.Priority, tt.want.Priority)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Blank != tt.want.Blank {
				t.Errorf("Blank = %v, want %v", got.Blank, tt.want.Blank)
			}
			if !eqStrings(got.Projects, tt.want.Projects) {
				t.Errorf("Projects = %v, want %v", got.Projects, tt.want.Projects)
			}
			if !eqStrings(got.Contexts, tt.want.Contexts) {
				t.Errorf("Contexts = %v, want %v", got.Contexts, tt.want.Contexts)
			}
			for k, v := range tt.want.Metadata {
				if got.Metadata[k] != v {
					t.Errorf("Metadata[%q] = %q, want %q", k, got.Metadata[k], v)
				}
			}
			// Raw は常に原文を保持する
			if got.Raw != tt.line {
				t.Errorf("Raw = %q, want %q", got.Raw, tt.line)
			}
		})
	}
}

func TestParseDates(t *testing.T) {
	got := ParseLine("x 2011-03-02 2011-03-01 task", 0)
	if got.CompletionDate == nil || !got.CompletionDate.Equal(mustDate(t, "2011-03-02")) {
		t.Errorf("CompletionDate = %v, want 2011-03-02", got.CompletionDate)
	}
	if got.CreationDate == nil || !got.CreationDate.Equal(mustDate(t, "2011-03-01")) {
		t.Errorf("CreationDate = %v, want 2011-03-01", got.CreationDate)
	}

	got2 := ParseLine("(B) 2026-06-01 task", 0)
	if got2.CreationDate == nil || !got2.CreationDate.Equal(mustDate(t, "2026-06-01")) {
		t.Errorf("CreationDate = %v, want 2026-06-01", got2.CreationDate)
	}
}

// TestRoundTrip はパース→文字列化で原文が完全に保たれることを確認する。
func TestRoundTrip(t *testing.T) {
	content := "(A) 2026-06-01 Call Mom +Family @phone\n" +
		"x 2026-06-08 古紙を出す\n" +
		"\n" +
		"Pay bills due:2026-06-30 @home\n" +
		"  weird   spacing   line  \n"

	tasks := Parse(content)
	var rebuilt string
	for i, tk := range tasks {
		if i > 0 {
			rebuilt += "\n"
		}
		rebuilt += tk.String()
	}
	rebuilt += "\n"

	if rebuilt != content {
		t.Errorf("round trip mismatch:\n got: %q\nwant: %q", rebuilt, content)
	}
}

func eqStrings(a, b []string) bool {
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
