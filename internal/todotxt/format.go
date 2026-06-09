package todotxt

// String は Task を todo.txt の 1 行として返す。
//
// 本ツールはタスク行を編集しないため、常に原文 (Raw) を返す。
// これにより、アーカイブ対象以外の行はファイルへ書き戻しても 1 バイトも変化しない。
func (t Task) String() string {
	return t.Raw
}
