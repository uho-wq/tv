// Package todotxt は todo.txt フォーマットのパースとモデルを提供する。
//
// 設計上の重要方針: 本ツールはタスク行を編集せず「閲覧」と「アーカイブ移動」しか
// 行わない。アーカイブは行を変換せず原文 (Raw) のまま archive.txt へ移すため、
// パースで得る構造化フィールドは表示・フィルタ・グルーピング専用であり、
// ファイルへの書き戻しは常に Raw を用いる（ラウンドトリップ保証）。
package todotxt

import (
	"slices"
	"time"
)

// Priority はタスクの優先度を表す。'A'..'Z' の大文字、または優先度なし。
type Priority byte

// PriorityNone は優先度が設定されていないことを表す。
const PriorityNone Priority = 0

// String は "(A)" 形式の文字列を返す。優先度なしの場合は空文字。
func (p Priority) String() string {
	if p == PriorityNone {
		return ""
	}
	return "(" + string(byte(p)) + ")"
}

// Task は todo.txt の 1 行を表す。
//
// Raw は常にパース元の原文を保持する（行末の改行は含まない）。
// それ以外のフィールドは表示・フィルタ・グルーピング用に抽出した値であり、
// ファイルへ書き戻すときは Raw をそのまま使う。
type Task struct {
	Raw            string            // パース元の生テキスト（改行なし）
	Blank          bool              // 空行（空白のみの行を含む）
	Completed      bool              // 行頭が "x " で始まる
	Priority       Priority          // 優先度（未完了タスクの (A) など）
	CompletionDate *time.Time        // 完了日（完了タスクのみ）
	CreationDate   *time.Time        // 作成日
	Description    string            // 優先度・完了マーカー・日付を除いた本文
	Projects       []string          // "+" を除いた project 名
	Contexts       []string          // "@" を除いた context 名
	Metadata       map[string]string // key:value メタデータ
	LineNo         int               // 元ファイルでの行番号（0 始まり）
}

// HasProject は task が指定 project を持つか返す。
func (t Task) HasProject(name string) bool {
	return slices.Contains(t.Projects, name)
}

// HasContext は task が指定 context を持つか返す。
func (t Task) HasContext(name string) bool {
	return slices.Contains(t.Contexts, name)
}
