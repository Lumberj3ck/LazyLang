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
	finishedSetup
)

type wizardModel struct {
	exitErr       error
	c             *Config
	showOverlay   bool
	overlay       overlayModel
	stage         Stage
	currRow       int
	providersList []ProviderAvailable
	width         int
	height        int
}

var dim = lipgloss.NewStyle().Faint(true)

var providersKey = map[string]string{
	"openai": "OpenAI",
	"groq":   "Groq",
	"ollama": "Ollama",
}

type SttEntity struct {
	key  string
	name string
}

type ProviderAvailable struct {
	available bool
	key       string
}

var localStt = "local"
var providerStt = "provider"
var sttsKey = []SttEntity{
	{localStt, "Local Whisper model"},
	{providerStt, "Provider STT model"},
}

func NewWizard(c *Config) wizardModel {
	providers := make([]ProviderAvailable, 0, len(providersKey))
	for key := range providersKey {
		log.Printf("%q", c)
		_, available := c.Providers[key]
		providers = append(providers, ProviderAvailable{available, key})
	}

	return wizardModel{
		c:             c,
		overlay:       NewOverlay(true, true, true),
		showOverlay:   false,
		stage:         adjustProvider,
		currRow:       0,
		providersList: providers,
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
			var rowsNum int

			switch m.stage {
			case adjustProvider:
				rowsNum = len(m.providersList)
			case adjustSTT:
				rowsNum = len(sttsKey)
			}
			m.currRow = min(rowsNum-1, m.currRow+1)
		case "esc":
			m.showOverlay = false
		case "k", "up":
			if m.showOverlay {
				break
			}
			m.currRow = max(0, m.currRow-1)
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.showOverlay {
				if !m.overlay.IsValid() {
					s := lipgloss.NewStyle().Foreground(lipgloss.Color("#961c92"))
					m.overlay.SetMsg(s.Render("Please fill all the required fields"))
					break
				}
				switch m.stage {
				case adjustProvider:
					m.showOverlay = false
					provider := m.providersList[m.currRow]
					np := m.overlay.GetInputs()
					m.c.Providers[provider.key] = np
					m.c.CompletionProvider = provider.key

					log.Println(m.c)
					m.stage = adjustSTT
					m.currRow = 0
				case adjustSTT:
					m.showOverlay = false
					np := m.overlay.GetInputs()
					t := sttsKey[m.currRow]
					s := STTBackend{
						Type:  STTType(t.key),
						Model: np.Model,
						URL:   np.BaseURL,
						Token: np.Token,
					}
					m.c.STTBackend = s
					if err := SaveConfig(*m.c); err != nil {
						m.exitErr = err
						return m, tea.Quit
					}
					m.stage = finishedSetup
				}
			} else {
				m.showOverlay = true
				m.overlay = NewOverlay(true, true, true)
				if m.stage == adjustSTT {
					t := sttsKey[m.currRow]
					if t.key == localStt {
						m.overlay = NewOverlay(false, false, true)
					}
				}
			}
		}
	}

	if m.showOverlay {
		m.overlay, cmd = m.overlay.Update(msg)
	}
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

func overlayAt(bg, fg string, x, y int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for i, line := range fgLines {
		by := y + i
		if by < 0 || by >= len(bgLines) {
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
	case adjustSTT:
		for i, stt := range sttsKey {
			check := " "
			if i == m.currRow {
				check = "x"
			}
			fmt.Fprintf(&view, "[%s] %s\n", check, stt.name)
		}
	case adjustProvider:
		for idx, provider := range m.providersList {
			name := providersKey[provider.key]
			checkSign := " "
			if provider.available {
				checkSign = "✓"
			}
			if m.currRow == idx {
				fmt.Fprintf(&view, "> %s %s\n", name, checkSign)
			} else {
				fmt.Fprintf(&view, "  %s %s\n", name, checkSign)
			}
		}
	case finishedSetup:
		fmt.Fprintf(&view, "Setup is done; reload app")
	}
	base := view.String()
	if m.showOverlay {
		// Overlay math assumes a fixed-size background canvas.
		if m.width > 0 && m.height > 0 {
			base = lipgloss.NewStyle().Width(m.width).Height(m.height).Render(base)
		}

		overlayBox := m.overlay.View()
		x := m.width/2 - lipgloss.Width(overlayBox)/2
		y := m.height/2 - lipgloss.Height(overlayBox)/2
		faintBase := dim.Render(base)
		return overlayAt(faintBase, overlayBox, x, y)
	}
	return base
}
