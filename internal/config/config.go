// Package config は対象 todo.txt ファイルのパス解決を担う。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath は使用する todo.txt のパスを以下の優先順位で決定する。
//
//  1. CLI 引数 (cliArg, 空文字なら無視)
//  2. 環境変数 TODOTXT_FILE
//  3. 環境変数 TODO_FILE (todo.txt-cli 互換)
//  4. 環境変数 TODO_DIR + "/todo.txt" (todo.txt-cli 互換)
//  5. デフォルト ~/todo.txt
//
// 先頭の "~" はホームディレクトリに展開する。
// CLI 引数が与えられた場合は存在チェックをして、見つからなければエラーを返す。
// それ以外の候補は「存在する最初のもの」を返し、どれも無ければデフォルトパスを
// （存在しなくても）返す。呼び出し側で存在確認・エラー表示を行う。
func ResolvePath(cliArg string) (string, error) {
	if cliArg != "" {
		p, err := expand(cliArg)
		if err != nil {
			return "", err
		}
		if !fileExists(p) {
			return "", fmt.Errorf("file not found: %s", p)
		}
		return p, nil
	}

	for _, cand := range candidates() {
		p, err := expand(cand)
		if err != nil || p == "" {
			continue
		}
		if fileExists(p) {
			return p, nil
		}
	}

	// どの候補も存在しない場合はデフォルトを返す（存在チェックは呼び出し側）。
	return expand(defaultPath())
}

func candidates() []string {
	var cands []string
	if v := os.Getenv("TODOTXT_FILE"); v != "" {
		cands = append(cands, v)
	}
	if v := os.Getenv("TODO_FILE"); v != "" {
		cands = append(cands, v)
	}
	if v := os.Getenv("TODO_DIR"); v != "" {
		cands = append(cands, filepath.Join(v, "todo.txt"))
	}
	cands = append(cands, defaultPath())
	return cands
}

func defaultPath() string {
	return "~/todo.txt"
}

// expand は先頭の "~" をホームディレクトリに展開する。
func expand(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
