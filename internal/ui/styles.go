package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/uho-wq/tv/internal/todotxt"
)

type styles struct {
	header     lipgloss.Style
	headerMeta lipgloss.Style
	groupTitle lipgloss.Style
	cursor     lipgloss.Style
	completed  lipgloss.Style
	project    lipgloss.Style
	context    lipgloss.Style
	meta       lipgloss.Style
	paneSep    lipgloss.Style
	footer     lipgloss.Style
	status     lipgloss.Style
	statusErr  lipgloss.Style
	priority   map[todotxt.Priority]lipgloss.Style
	priDefault lipgloss.Style
}

func newStyles() styles {
	s := styles{
		header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("63")).
			Padding(0, 1),
		headerMeta: lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		groupTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("213")),
		cursor:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("238")),
		completed: lipgloss.NewStyle().Faint(true).Strikethrough(true),
		project:   lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		context:   lipgloss.NewStyle().Foreground(lipgloss.Color("75")),
		meta:      lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		paneSep:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		footer:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1),
		status:    lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Padding(0, 1),
		statusErr: lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Padding(0, 1),
	}

	// 優先度ごとの色
	bold := lipgloss.NewStyle().Bold(true)
	s.priority = map[todotxt.Priority]lipgloss.Style{
		'A': bold.Foreground(lipgloss.Color("203")), // 赤
		'B': bold.Foreground(lipgloss.Color("215")), // 橙
		'C': bold.Foreground(lipgloss.Color("221")), // 黄
		'D': bold.Foreground(lipgloss.Color("114")), // 緑
	}
	s.priDefault = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	return s
}

// priorityStyle は優先度に対応する色スタイルを返す。
func (s styles) priorityStyle(p todotxt.Priority) lipgloss.Style {
	if st, ok := s.priority[p]; ok {
		return st
	}
	return s.priDefault
}
