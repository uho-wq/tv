// Package filter はタスクの絞り込みとグルーピングのロジックを提供する。
package filter

import (
	"slices"
	"strings"

	"github.com/uho-wq/todotxt-viewer/internal/todotxt"
)

// Criteria は絞り込み条件を表す。
type Criteria struct {
	Projects      []string           // いずれかにマッチ (OR)
	Contexts      []string           // いずれかにマッチ (OR)
	Priorities    []todotxt.Priority // いずれかにマッチ (OR)
	ShowCompleted bool               // 完了タスクを含めるか
	Query         string             // 本文の部分一致（大文字小文字無視）
}

// Empty は条件が未設定（全件表示と同等）か返す。
func (c Criteria) Empty() bool {
	return len(c.Projects) == 0 && len(c.Contexts) == 0 &&
		len(c.Priorities) == 0 && c.Query == ""
}

// Apply は条件に合致するタスクの「インデックス」（tasks 内の位置）を返す。
// 空行は常に除外する。各カテゴリ内は OR、カテゴリ間は AND で評価する。
//
// インデックスを返すのは、呼び出し側（UI）が元の tasks スライスに対して
// アーカイブ等の操作を行えるようにするため。
func Apply(tasks []todotxt.Task, c Criteria) []int {
	out := make([]int, 0, len(tasks))
	q := strings.ToLower(c.Query)
	for i, t := range tasks {
		if t.Blank {
			continue
		}
		if !c.ShowCompleted && t.Completed {
			continue
		}
		if len(c.Projects) > 0 && !slices.ContainsFunc(c.Projects, t.HasProject) {
			continue
		}
		if len(c.Contexts) > 0 && !slices.ContainsFunc(c.Contexts, t.HasContext) {
			continue
		}
		if len(c.Priorities) > 0 && !slices.Contains(c.Priorities, t.Priority) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Raw), q) {
			continue
		}
		out = append(out, i)
	}
	return out
}

// ParseQuery はフィルタ入力文字列を Criteria に変換する。
//
//	"+Proj"  -> Projects
//	"@ctx"   -> Contexts
//	"(A)"    -> Priorities
//	その他    -> Query（スペース結合の部分一致）
func ParseQuery(input string) Criteria {
	var c Criteria
	var queryParts []string
	for tok := range strings.FieldsSeq(input) {
		switch {
		case len(tok) > 1 && tok[0] == '+':
			c.Projects = append(c.Projects, tok[1:])
		case len(tok) > 1 && tok[0] == '@':
			c.Contexts = append(c.Contexts, tok[1:])
		case len(tok) == 3 && tok[0] == '(' && tok[2] == ')' && tok[1] >= 'A' && tok[1] <= 'Z':
			c.Priorities = append(c.Priorities, todotxt.Priority(tok[1]))
		default:
			queryParts = append(queryParts, tok)
		}
	}
	c.Query = strings.Join(queryParts, " ")
	return c
}
