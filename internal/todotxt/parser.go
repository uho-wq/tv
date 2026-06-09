package todotxt

import (
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Parse はファイル内容全体を行単位で Task に変換する。
// 末尾の改行差異を吸収するため、"\r\n" と "\n" の両方を行区切りとして扱う。
// 空行も Task（Blank=true）として保持し、行番号の対応を保つ。
func Parse(content string) []Task {
	// CRLF を一旦 LF に正規化してから分割する（Raw からは改行を除く）。
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	// 末尾の改行による空要素を避けつつ、途中の空行は維持する。
	lines := strings.Split(normalized, "\n")
	// 末尾が改行で終わる場合、Split は末尾に空要素を1つ作る。これは行ではないので落とす。
	if n := len(lines); n > 0 && lines[n-1] == "" && strings.HasSuffix(normalized, "\n") {
		lines = lines[:n-1]
	}

	tasks := make([]Task, 0, len(lines))
	for i, line := range lines {
		tasks = append(tasks, ParseLine(line, i))
	}
	return tasks
}

// ParseLine は 1 行を Task に変換する。
func ParseLine(line string, lineNo int) Task {
	t := Task{Raw: line, LineNo: lineNo, Metadata: map[string]string{}}

	if strings.TrimSpace(line) == "" {
		t.Blank = true
		return t
	}

	fields := strings.Fields(line)
	i := 0

	// 完了マーカー: 行頭が単独の "x"
	if fields[i] == "x" {
		t.Completed = true
		i++
		// 完了タスク: x <完了日> [<作成日>] ...
		if i < len(fields) {
			if d, ok := parseDate(fields[i]); ok {
				t.CompletionDate = &d
				i++
				if i < len(fields) {
					if d2, ok := parseDate(fields[i]); ok {
						t.CreationDate = &d2
						i++
					}
				}
			}
		}
	} else if pr, ok := parsePriority(fields[i]); ok {
		// 未完了タスクの優先度
		t.Priority = pr
		i++
	}

	// 完了タスクで優先度が残っている記法 (x date (A) ...) にも一応対応
	if i < len(fields) {
		if pr, ok := parsePriority(fields[i]); ok && t.Priority == PriorityNone {
			t.Priority = pr
			i++
		}
	}

	// 未完了タスクの作成日（優先度の直後、または行頭）
	if !t.Completed && t.CreationDate == nil && i < len(fields) {
		if d, ok := parseDate(fields[i]); ok {
			t.CreationDate = &d
			i++
		}
	}

	// 残りトークンが本文。project / context / metadata を抽出する。
	rest := fields[i:]
	t.Description = strings.Join(rest, " ")
	for _, tok := range rest {
		switch {
		case len(tok) > 1 && tok[0] == '+':
			t.Projects = append(t.Projects, tok[1:])
		case len(tok) > 1 && tok[0] == '@':
			t.Contexts = append(t.Contexts, tok[1:])
		default:
			if k, v, ok := parseKeyValue(tok); ok {
				t.Metadata[k] = v
			}
		}
	}

	return t
}

// parsePriority は "(A)" 形式を Priority に変換する。
func parsePriority(tok string) (Priority, bool) {
	if len(tok) == 3 && tok[0] == '(' && tok[2] == ')' && tok[1] >= 'A' && tok[1] <= 'Z' {
		return Priority(tok[1]), true
	}
	return PriorityNone, false
}

// parseDate は "YYYY-MM-DD" を time.Time に変換する。
func parseDate(tok string) (time.Time, bool) {
	if len(tok) != 10 {
		return time.Time{}, false
	}
	d, err := time.Parse(dateLayout, tok)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// parseKeyValue は "key:value" 形式を分解する。
// key と value はともに非空白で、コロンはちょうど 1 つ（URL の "http://" などは除外）。
func parseKeyValue(tok string) (key, value string, ok bool) {
	idx := strings.IndexByte(tok, ':')
	if idx <= 0 || idx >= len(tok)-1 {
		return "", "", false
	}
	key = tok[:idx]
	value = tok[idx+1:]
	if strings.ContainsRune(value, ':') {
		return "", "", false
	}
	return key, value, true
}
