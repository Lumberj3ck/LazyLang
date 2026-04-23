package main

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Stage int

const (
	adjustProvider Stage = iota
	adjustSTT
)

type wizardModel struct {
	c                  *Config
	stage              Stage
	currRow            int
	availableProviders []string
}

func initialWizard(c *Config) wizardModel {
	log.Println(c)
	return wizardModel{c, adjustProvider, 0, []string{"OpenAI", "Groq", "Ollama", "Custom openai style endpoint"}}
}

func (m wizardModel) Init() tea.Cmd {
	return nil
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "j", "down":
			log.Println(m.currRow)
			if m.stage == adjustProvider{
				m.currRow = min(len(m.availableProviders)-1, m.currRow+1)
			}
		case "k", "up":
			m.currRow = max(0, m.currRow-1)
		case "ctrl+c", "q":
			return m, tea.Quit
		}
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
	return m, nil
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
		if m.currRow > len(m.availableProviders) || m.currRow < 0 {
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
	return view.String()
}
