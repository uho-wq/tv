package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up              key.Binding
	Down            key.Binding
	Top             key.Binding
	Bottom          key.Binding
	Complete        key.Binding
	Archive         key.Binding
	Undo            key.Binding
	Group           key.Binding
	ToggleCompleted key.Binding
	Filter          key.Binding
	ClearFilter     key.Binding
	Reload          key.Binding
	Edit            key.Binding
	Help            key.Binding
	Quit            key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "上"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "下"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "先頭"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "末尾"),
		),
		Complete: key.NewBinding(
			key.WithKeys("x", " "),
			key.WithHelp("x", "完了トグル"),
		),
		Archive: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "完了をアーカイブ"),
		),
		Undo: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "取消"),
		),
		Group: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "グループ切替"),
		),
		ToggleCompleted: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "完了表示"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f", "/"),
			key.WithHelp("f", "フィルタ"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "フィルタ解除"),
		),
		Reload: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "再読込"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "エディタで開く"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "ヘルプ"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "終了"),
		),
	}
}
