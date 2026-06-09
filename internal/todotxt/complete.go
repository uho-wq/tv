package todotxt

import (
	"strings"
	"time"
)

// CompleteOptions は完了マーク付与時の挙動を制御する。
type CompleteOptions struct {
	// PreservePriorityAsKV を true にすると、完了時に優先度を削除する代わりに
	// "pri:A" のメタデータへ退避する（Uncomplete で優先度を復元できる）。
	// 既定 (false) は todo.txt-cli と同様に優先度を削除する。
	PreservePriorityAsKV bool
}

// Complete は未完了タスクに完了マークを付ける。
//
//	(A) 2026-06-01 買い物 +Home  →  x 2026-06-09 2026-06-01 買い物 +Home
//
// 完了日 (today) を行頭の "x" の直後に置き、作成日があればその後ろに残す。
// 既定では優先度を削除する（opts で pri: への退避も可能）。
// 変換後は原文 (Raw) を再生成し、全フィールドを整合させる。
func (t *Task) Complete(today time.Time, opts CompleteOptions) {
	if t.Blank || t.Completed {
		return
	}

	body := t.Description
	if opts.PreservePriorityAsKV && t.Priority != PriorityNone {
		body = appendToken(body, "pri:"+string(byte(t.Priority)))
	}

	parts := []string{"x", today.Format(dateLayout)}
	if t.CreationDate != nil {
		parts = append(parts, t.CreationDate.Format(dateLayout))
	}
	line := strings.Join(parts, " ")
	if body != "" {
		line += " " + body
	}

	*t = ParseLine(line, t.LineNo)
}

// Uncomplete は完了タスクを未完了に戻す。
// 完了マークと完了日を取り除き、"pri:" に退避された優先度があれば復元する。
func (t *Task) Uncomplete() {
	if t.Blank || !t.Completed {
		return
	}

	body := t.Description
	pr := PriorityNone
	if v, ok := t.Metadata["pri"]; ok && len(v) == 1 && v[0] >= 'A' && v[0] <= 'Z' {
		pr = Priority(v[0])
		body = removeToken(body, "pri:"+v)
	}

	var parts []string
	if pr != PriorityNone {
		parts = append(parts, pr.String())
	}
	if t.CreationDate != nil {
		parts = append(parts, t.CreationDate.Format(dateLayout))
	}
	if body != "" {
		parts = append(parts, body)
	}

	*t = ParseLine(strings.Join(parts, " "), t.LineNo)
}

func appendToken(body, tok string) string {
	if strings.TrimSpace(body) == "" {
		return tok
	}
	return body + " " + tok
}

func removeToken(body, tok string) string {
	fields := strings.Fields(body)
	out := fields[:0]
	for _, f := range fields {
		if f != tok {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}
