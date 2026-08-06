// Package store は todo.txt の読み込み・原子的な書き込み・archive.txt への
// アーカイブ移動・外部変更検知を担う。
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/uho-wq/tv/internal/todotxt"
)

// File は読み込んだ todo.txt とそのタスク群を表す。
type File struct {
	Path    string
	Tasks   []todotxt.Task
	mtime   time.Time
	hadEOL  bool // 元ファイルが末尾改行で終わっていたか
	useCRLF bool // 元ファイルが CRLF を使っていたか
}

// Load はファイルを読み込み、パースして File を返す。
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	f := &File{
		Path:    path,
		Tasks:   todotxt.Parse(content),
		hadEOL:  strings.HasSuffix(content, "\n"),
		useCRLF: strings.Contains(content, "\r\n"),
	}
	if info, err := os.Stat(path); err == nil {
		f.mtime = info.ModTime()
	}
	return f, nil
}

// render は現在の Tasks をファイル内容（バイト列）に整形する。
// 元ファイルの改行コードと末尾改行の有無を踏襲する。
func (f *File) render() string {
	eol := "\n"
	if f.useCRLF {
		eol = "\r\n"
	}
	lines := make([]string, len(f.Tasks))
	for i, t := range f.Tasks {
		lines[i] = t.String()
	}
	out := strings.Join(lines, eol)
	if f.hadEOL && len(f.Tasks) > 0 {
		out += eol
	}
	return out
}

// Save は現在の Tasks を原子的に書き戻す（一時ファイル + rename）。
// 書き込み成功後は内部の mtime を更新し、自身の書き込みを外部変更と誤検知しないようにする。
func (f *File) Save() error {
	if err := atomicWrite(f.Path, []byte(f.render())); err != nil {
		return err
	}
	f.refreshMtime()
	return nil
}

// ExternallyModified は前回読み込み/保存以降にファイルが外部で変更されたか返す。
func (f *File) ExternallyModified() (bool, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return false, err
	}
	return info.ModTime().After(f.mtime), nil
}

// refreshMtime は保存後に内部の mtime を更新する。
func (f *File) refreshMtime() {
	if info, err := os.Stat(f.Path); err == nil {
		f.mtime = info.ModTime()
	}
}

// ArchivePath は todo.txt と同じディレクトリの archive.txt のパスを返す。
func (f *File) ArchivePath() string {
	return filepath.Join(filepath.Dir(f.Path), "archive.txt")
}

// Archive は indices で指定したタスクを archive.txt に「行そのまま」追記し、
// todo.txt から取り除いて保存する。アーカイブした行（追記順）を返す。
//
// indices は Tasks のインデックス。複数指定可。空行は対象外として無視する。
func (f *File) Archive(indices ...int) ([]string, error) {
	// 重複を除き、降順に並べて削除時のインデックスずれを防ぐ。
	seen := map[int]bool{}
	var sorted []int
	for _, i := range indices {
		if i < 0 || i >= len(f.Tasks) || f.Tasks[i].Blank || seen[i] {
			continue
		}
		seen[i] = true
		sorted = append(sorted, i)
	}
	if len(sorted) == 0 {
		return nil, nil
	}
	// 追記順は元の並び（昇順）にする。
	asc := append([]int(nil), sorted...)
	slices.Sort(asc)

	archived := make([]string, 0, len(asc))
	for _, i := range asc {
		archived = append(archived, f.Tasks[i].Raw)
	}

	// 先に archive.txt へ追記する（失敗したら todo.txt は変更しない）。
	if err := appendLines(f.ArchivePath(), archived, f.useCRLF); err != nil {
		return nil, err
	}

	// todo.txt から削除（降順で削除してインデックスずれを回避）。
	descIdx := append([]int(nil), asc...)
	slices.Reverse(descIdx)
	for _, i := range descIdx {
		f.Tasks = append(f.Tasks[:i], f.Tasks[i+1:]...)
	}

	if err := f.Save(); err != nil {
		return nil, err
	}
	return archived, nil
}

// Delete は idx のタスクを todo.txt から取り除いて保存し、削除した原文を返す。
// archive.txt には残さないため、取り消しは Insert で行う（呼び出し側が原文を保持する）。
// 保存に失敗した場合は Tasks を元に戻し、ファイルとメモリの状態を一致させる。
func (f *File) Delete(idx int) (string, error) {
	if idx < 0 || idx >= len(f.Tasks) {
		return "", fmt.Errorf("インデックスが範囲外です: %d", idx)
	}
	removed := f.Tasks[idx]
	f.Tasks = slices.Delete(f.Tasks, idx, idx+1)
	if err := f.Save(); err != nil {
		f.Tasks = slices.Insert(f.Tasks, idx, removed)
		return "", err
	}
	return removed.Raw, nil
}

// Insert は raw をパースして idx の位置に挿入し、保存する（Delete の取り消し用）。
// idx が範囲外なら末尾に追加する。
func (f *File) Insert(idx int, raw string) error {
	if idx < 0 || idx > len(f.Tasks) {
		idx = len(f.Tasks)
	}
	f.Tasks = slices.Insert(f.Tasks, idx, todotxt.ParseLine(raw, idx))
	if err := f.Save(); err != nil {
		f.Tasks = slices.Delete(f.Tasks, idx, idx+1)
		return err
	}
	return nil
}

// UndoArchive は直前の Archive を取り消す。archive.txt の末尾から count 行を
// 取り除き、それらの行を todo.txt の末尾に戻して保存する。
// archive.txt の末尾が想定（want）と一致しない場合はエラーを返す（外部変更検知）。
func (f *File) UndoArchive(want []string) error {
	if len(want) == 0 {
		return nil
	}
	if err := removeTrailingLines(f.ArchivePath(), want); err != nil {
		return err
	}
	for _, line := range want {
		f.Tasks = append(f.Tasks, todotxt.ParseLine(line, len(f.Tasks)))
	}
	return f.Save()
}

// --- ファイル I/O ヘルパ ---

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tv-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功後は存在しないので no-op

	// 元ファイルのパーミッションに合わせる。
	if info, err := os.Stat(path); err == nil {
		_ = tmp.Chmod(info.Mode().Perm())
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// appendLines は lines を path に追記する。ファイルが無ければ作成する。
func appendLines(path string, lines []string, useCRLF bool) error {
	eol := "\n"
	if useCRLF {
		eol = "\r\n"
	}
	// 既存ファイルが末尾改行で終わっていない場合に備え、先頭に改行を補う。
	prefix := ""
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		prefix = eol
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	var b strings.Builder
	b.WriteString(prefix)
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString(eol)
	}
	_, err = f.WriteString(b.String())
	return err
}

// removeTrailingLines は path の末尾 len(want) 行が want と一致することを確認し、
// 一致すればそれらを取り除いて書き戻す。不一致ならエラー（外部変更の疑い）。
func removeTrailingLines(path string, want []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	hadEOL := strings.HasSuffix(content, "\n")
	trimmed := strings.TrimSuffix(content, "\n")

	var lines []string
	if trimmed != "" {
		lines = strings.Split(trimmed, "\n")
	}
	if len(lines) < len(want) {
		return fmt.Errorf("archive.txt has fewer lines than expected; refusing to undo")
	}
	tail := lines[len(lines)-len(want):]
	for i := range want {
		if tail[i] != want[i] {
			return fmt.Errorf("archive.txt tail does not match; it may have been modified externally")
		}
	}
	remaining := lines[:len(lines)-len(want)]

	out := strings.Join(remaining, "\n")
	if hadEOL && len(remaining) > 0 {
		out += "\n"
	}
	return atomicWrite(path, []byte(out))
}
