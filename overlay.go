package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overlayModel struct{
	baseUrlInput     textinput.Model
	tokenInput 		 textinput.Model
	width     int
	height    int
}

func NewOverlay() overlayModel {
	bu := textinput.New()
	bu.Focus()
	bu.Prompt = ""
	bu.Placeholder = "Base Url"
	// bubbles/textinput v0.21.0 truncates placeholders to 1 rune when Width <= 0.
	// Give a sensible default; we'll resize it in SetWidth on window size events.
	bu.Width = 40

	ti := textinput.New()
	ti.Placeholder = "Token"
	ti.Prompt = ""
	ti.Width = 40
	return overlayModel{
		baseUrlInput: bu,
		tokenInput:   ti,
	}
}
func (o overlayModel) Init() tea.Cmd {
	return nil
}

func (o overlayModel) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "tab", "shift+tab", "enter", "up", "down":
			if o.baseUrlInput.Focused(){
				o.tokenInput.Focus()
				o.baseUrlInput.Blur()
			} else {
				o.baseUrlInput.Focus()
				o.tokenInput.Blur()
			}
			 
		}
	}

	o.baseUrlInput, cmd = o.baseUrlInput.Update(msg)
	cmds = append(cmds, cmd)

	o.tokenInput, cmd = o.tokenInput.Update(msg)
	cmds = append(cmds, cmd)
	return o, tea.Batch(cmds...)
}

func (o *overlayModel) SetWidth(w int) {
	o.width = w
	// Keep the text inputs within the modal box.
	// Border is 1 char on each side, plus horizontal padding of 2 on each side.
	inner := max(1, w-6)
	o.baseUrlInput.Width = inner
	o.tokenInput.Width = inner
}

func (o *overlayModel) SetHeight(h int) {
	o.height = h
}

func (o overlayModel) View() string {
	view := fmt.Sprintf("%s\n%s", o.baseUrlInput.View(), o.tokenInput.View())
	modalBox := lipgloss.NewStyle().
		Padding(1, 2).
		Width(o.width).
		// Height(o.height).
		Border(lipgloss.RoundedBorder()).
		Render(view)
	
	return modalBox
}
