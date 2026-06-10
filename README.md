# tv

[todo.txt フォーマット](https://github.com/todotxt/todo.txt)で書かれたタスクを閲覧する TUI ツール。
Go + [Bubble Tea](https://github.com/charmbracelet/bubbletea) 製。

## インストール

```sh
go install github.com/uho-wq/tv@latest
```


## 使い方

```sh
tv [path]
```

`path` を省略した場合、次の優先順位で todo.txt を探します。

1. CLI 引数
2. 環境変数 `TODOTXT_FILE`
3. 環境変数 `TODO_FILE` (todo.txt-cli 互換)
4. 環境変数 `TODO_DIR` + `/todo.txt`
5. `~/todo.txt`

## キーバインド

| キー | 動作 |
|------|------|
| `↑`/`k`, `↓`/`j` | カーソル移動 |
| `g` / `G` | 先頭 / 末尾へ |
| `x` / `Space` | 完了トグル（`x` + 完了日を付与/除去） |
| `a` | 完了済みを `archive.txt` へ一括移動 |
| `u` | 直前のアーカイブを取り消し |
| `f` / `/` | フィルタ入力 (`+proj @ctx (A) keyword`) |
| `esc` | フィルタ解除 |
| `tab` | グルーピング切替 (project → context → priority → flat) |
| `c` | 完了タスクの表示/非表示 |
| `r` | ファイル再読込 |
| `?` | ヘルプ |
| `q` / `Ctrl+C` | 終了 |
