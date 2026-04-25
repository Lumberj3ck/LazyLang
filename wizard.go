package main

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type Stage int

const (
	adjustProvider Stage = iota
	adjustSTT
)

type wizardModel struct {
	c                  *Config
	showOverlay        bool
	overlay 		   overlayModel
	stage              Stage
	currRow            int
	availableProviders []string
	width int
	height int
}

var dim = lipgloss.NewStyle().Faint(true)

func NewWizard(c *Config) wizardModel {
	return wizardModel{
		c: c, 
		overlay: NewOverlay(),
		showOverlay: false, 
		stage: adjustProvider,
		currRow: 0, 
		availableProviders: []string{"OpenAI", "Groq", "Ollama", "Custom openai style endpoint"},
	}
}

func (m wizardModel) Init() tea.Cmd {
	return nil
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

		m.overlay.SetHeight(m.height / 2)
		m.overlay.SetWidth(m.width / 2)
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "j", "down":
			if m.showOverlay {
				break
			}
			log.Println(m.currRow)
			if m.stage == adjustProvider{
				m.currRow = min(len(m.availableProviders)-1, m.currRow+1)
			}
		case "k", "up":
			if m.showOverlay {
				break
			}
			m.currRow = max(0, m.currRow-1)
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+p":
			m.showOverlay = !m.showOverlay
		}
	}
	
	m.overlay, cmd = m.overlay.Update(msg)
	// case tea.WindowSizeMsg:
	// if !m.ready {
	// m.ready = true
	// } else {
	// m.viewport.Width = viewportWidth
	// m.viewport.Height = viewportHeight
	// }
	// }
	// Keep the existing model state; returning a zero-value model
	// would drop the config pointer and later dereference nil in View().
	return m, cmd
}

func overlayAt(bg, fg string, x, y int) string{
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, line := range fgLines{
		by := y + i
		if by < 0 || by >= len(bgLines){
			continue
		}

		bgLine := bgLines[by]
		end := x + ansi.StringWidth(line)
		left := ansi.Cut(bgLine, 0, x)
		right := ansi.Cut(bgLine, end, ansi.StringWidth(bgLine))
		bgLines[by] = left + line + right
	}
	return strings.Join(bgLines, "\n")
}

func (m wizardModel) View() string {
	// Select Completion Provider
	//     	Openai
	//     	Groq
	//     	Ollama (default localhost:11434)
	//		Please provide openai style Completion url
	// 		Skip (here check if completion provider is already present) if not exit
	//   	And everytime we don't have anything, we just try to redirect to the Setup Wizard
	//
	// Select STT Provider
	// 		Openai
	// 		Groq
	// 		Run local
	// 		Skip (Same check If we already have STT, otherwise exit and on new start redirect here)
	// Select TTS
	var view strings.Builder
	switch m.stage {
	case adjustProvider:
		if m.currRow >= len(m.availableProviders) || m.currRow < 0 {
			m.currRow = 0
		}

		for idx, provider := range m.availableProviders {
			if m.currRow == idx {
				fmt.Fprintf(&view, "> %s\n", provider)
			} else {
				fmt.Fprintf(&view, "  %s\n", provider)
			}
		}
	}
	base := view.String()
	if m.showOverlay{
		// Overlay math assumes a fixed-size background canvas.
		if m.width > 0 && m.height > 0 {
			base = lipgloss.NewStyle().Width(m.width).Height(m.height).Render(base)
		}

		overlayBox := m.overlay.View()
		x := m.width / 2 - lipgloss.Width(overlayBox) / 2
		y := m.height / 2 - lipgloss.Height(overlayBox) / 2
		faintBase := dim.Render(base)
		return overlayAt(faintBase, overlayBox, x, y)
	}
	return base
}
