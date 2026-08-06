package store

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	content := "(A) task one +Proj @ctx\nx 2026-06-08 done task\nplain task\n"
	writeFile(t, path, content)

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != content {
		t.Errorf("round trip mismatch:\n got: %q\nwant: %q", got, content)
	}
}

func TestDeleteAndInsert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "a\nb\nc\n")

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := f.Delete(1)
	if err != nil {
		t.Fatal(err)
	}
	if raw != "b" {
		t.Errorf("deleted raw = %q, want %q", raw, "b")
	}
	if got := readFile(t, path); got != "a\nc\n" {
		t.Errorf("after delete = %q, want %q", got, "a\nc\n")
	}

	// 元の位置へ戻す。
	if err := f.Insert(1, raw); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "a\nb\nc\n" {
		t.Errorf("after insert = %q, want %q", got, "a\nb\nc\n")
	}
	// 再パースされ、構造化フィールドが復元されていること。
	if f.Tasks[1].Raw != "b" {
		t.Errorf("restored task = %+v", f.Tasks[1])
	}
}

func TestDeleteOutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "a\n")

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Delete(5); err == nil {
		t.Errorf("expected error for out-of-range index")
	}
	if got := readFile(t, path); got != "a\n" {
		t.Errorf("file changed: %q", got)
	}
}

func TestInsertBeyondEndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "a\n")

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Insert(99, "z"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "a\nz\n" {
		t.Errorf("after insert = %q, want %q", got, "a\nz\n")
	}
}

func TestArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	content := "(A) task one +Proj @ctx\ntask two\nx 2026-06-08 done task\n"
	writeFile(t, path, content)

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	// task two (index 1) をアーカイブ
	archived, err := f.Archive(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 || archived[0] != "task two" {
		t.Fatalf("archived = %v, want [task two]", archived)
	}

	// todo.txt から task two が消えている
	wantTodo := "(A) task one +Proj @ctx\nx 2026-06-08 done task\n"
	if got := readFile(t, path); got != wantTodo {
		t.Errorf("todo.txt = %q, want %q", got, wantTodo)
	}

	// archive.txt に行そのまま追記されている
	archivePath := filepath.Join(dir, "archive.txt")
	if got := readFile(t, archivePath); got != "task two\n" {
		t.Errorf("archive.txt = %q, want %q", got, "task two\n")
	}
}

func TestArchiveMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "a\nb\nc\nd\n")

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// b(1) と d(3) をアーカイブ。追記順は昇順（b, d）。
	archived, err := f.Archive(3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 2 || archived[0] != "b" || archived[1] != "d" {
		t.Fatalf("archived = %v, want [b d]", archived)
	}
	if got := readFile(t, path); got != "a\nc\n" {
		t.Errorf("todo.txt = %q, want %q", got, "a\nc\n")
	}
	if got := readFile(t, filepath.Join(dir, "archive.txt")); got != "b\nd\n" {
		t.Errorf("archive.txt = %q, want %q", got, "b\nd\n")
	}
}

func TestUndoArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	writeFile(t, path, "a\nb\nc\n")

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := f.Archive(1) // b
	if err != nil {
		t.Fatal(err)
	}
	if err := f.UndoArchive(archived); err != nil {
		t.Fatal(err)
	}
	// b が末尾に戻る
	if got := readFile(t, path); got != "a\nc\nb\n" {
		t.Errorf("todo.txt = %q, want %q", got, "a\nc\nb\n")
	}
	// archive.txt は空に戻る
	if got := readFile(t, filepath.Join(dir, "archive.txt")); got != "" {
		t.Errorf("archive.txt = %q, want empty", got)
	}
}

func TestArchiveAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "todo.txt")
	archivePath := filepath.Join(dir, "archive.txt")
	writeFile(t, path, "new task\n")
	writeFile(t, archivePath, "old archived\n")

	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Archive(0); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, archivePath); got != "old archived\nnew task\n" {
		t.Errorf("archive.txt = %q, want %q", got, "old archived\nnew task\n")
	}
}
