package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	agentdetect "github.com/agentguard/agentguard/internal/agent_detect"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C5CFF"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B96A3"))
	checkedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3DDC97"))
	initDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5B6776"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#4AD6FF")).Bold(true)
	headerBox    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#232A33")).Padding(0, 1)
)

type initTUIModel struct {
	detections []*agentdetect.Detection
	selected   []bool
	cursor     int
	confirmed  bool
	aborted    bool
}

func newInitTUI(detections []*agentdetect.Detection) initTUIModel {
	sel := make([]bool, len(detections))
	for i := range sel {
		sel[i] = true
	}
	return initTUIModel{detections: detections, selected: sel}
}

func (m initTUIModel) Init() tea.Cmd { return nil }

func (m initTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+c", "q", "esc":
			m.aborted = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.detections)-1 {
				m.cursor++
			}
		case " ", "x":
			if len(m.selected) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "a":
			for i := range m.selected {
				m.selected[i] = true
			}
		case "n":
			for i := range m.selected {
				m.selected[i] = false
			}
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m initTUIModel) View() string {
	var b strings.Builder
	b.WriteString(headerBox.Render(titleStyle.Render("AgentGuard init")) + "\n")
	b.WriteString(helpStyle.Render("Choose which agents to route through AgentGuard.") + "\n\n")
	for i, d := range m.detections {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}
		mark := "[ ]"
		if m.selected[i] {
			mark = checkedStyle.Render("[✓]")
		}
		line := fmt.Sprintf("%s%s  %-12s  %s  %s",
			cursor, mark, d.DisplayName,
			initDimStyle.Render(d.ConfigPath),
			initDimStyle.Render(fmt.Sprintf("(%d server%s)", len(d.Servers), pluralS(len(d.Servers)))),
		)
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · space toggle · a all · n none · enter confirm · esc abort"))
	return b.String() + "\n"
}

// runInitTUI runs the Bubble Tea checklist and returns the list of detections
// the user selected. If the user aborts, returns (nil, true).
func runInitTUI(detections []*agentdetect.Detection) (kept []*agentdetect.Detection, aborted bool, err error) {
	if len(detections) == 0 {
		return nil, false, nil
	}
	p := tea.NewProgram(newInitTUI(detections))
	final, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	m := final.(initTUIModel)
	if m.aborted {
		return nil, true, nil
	}
	for i, d := range m.detections {
		if m.selected[i] {
			kept = append(kept, d)
		}
	}
	return kept, false, nil
}
