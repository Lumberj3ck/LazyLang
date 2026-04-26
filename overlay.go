package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type overlayModel struct {
	baseUrlInput *textinput.Model
	tokenInput   *textinput.Model
	modelInput   *textinput.Model
	focusIndex   int
	width        int
	height       int
	displayMsg   string
}

func NewOverlay() overlayModel {
	bu := textinput.New()
	bu.Focus()
	bu.Prompt = ""
	bu.Placeholder = "Base Url"
	// bubbles/textinput v0.21.0 truncates placeholders to 1 rune when Width <= 0.
	// Give a sensible default; we'll resize it in SetWidth on window size events.
	bu.Width = 40
	bu.Validate = func(s string) error {
		if len(s) <= 7 || strings.HasPrefix(s, "https://"){
			return nil
		}
		return fmt.Errorf("Invalid schema")
	}

	ti := textinput.New()
	ti.Placeholder = "Token"
	ti.Prompt = ""
	ti.Width = 40

	mi := textinput.New()
	mi.Placeholder = "Model"
	mi.Prompt = ""
	mi.Width = 40
	return overlayModel{
		baseUrlInput: &bu,
		tokenInput:   &ti,
		modelInput:   &mi,
	}
}
func (o overlayModel) Init() tea.Cmd {
	return nil
}

func (o overlayModel) GetInputs() Provider {
	return Provider{
		BaseURL: o.baseUrlInput.Value(),
		Token: o.tokenInput.Value(),
		Model: o.modelInput.Value(),
	}
}

func (o overlayModel) IsValid() bool {
	return o.baseUrlInput.Value() != "" && o.tokenInput.Value() != "" && o.modelInput.Value() != ""
}

func (o *overlayModel) SetMsg(msg string) {
	o.displayMsg = msg
}

func (o overlayModel) Update(msg tea.Msg) (overlayModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch k := msg.String(); k {
		case "tab", "shift+tab", "up", "down":
			s := msg.String()
			inputs := []*textinput.Model{o.baseUrlInput, o.tokenInput, o.modelInput}
			// // Cycle indexes
			if s == "up" || s == "shift+tab" {
				o.focusIndex--
			} else {
				o.focusIndex++
			}
			log.Println(o.focusIndex)

			if o.focusIndex >= len(inputs) {
				o.focusIndex = 0
			} else if o.focusIndex < 0 {
				o.focusIndex = len(inputs) - 1
			}

			for idx, item := range inputs {
				if idx == o.focusIndex {
					cmd = item.Focus()
				} else {
					item.Blur()
				}
			}

			return o, cmd
		}
	}
	um, cmd := o.baseUrlInput.Update(msg)
	cmds = append(cmds, cmd)
	o.baseUrlInput = &um

	tm, cmd := o.tokenInput.Update(msg)
	cmds = append(cmds, cmd)
	o.tokenInput = &tm

	mi, cmd := o.modelInput.Update(msg)
	cmds = append(cmds, cmd)
	o.modelInput = &mi
	return o, tea.Batch(cmds...)
}

func (o *overlayModel) SetWidth(w int) {
	o.width = w
	// Keep the text inputs within the modal box.
	// Border is 1 char on each side, plus horizontal padding of 2 on each side.
	inner := max(1, w-6)
	o.baseUrlInput.Width = inner
	o.tokenInput.Width = inner
	o.modelInput.Width = inner
}

func (o *overlayModel) SetHeight(h int) {
	o.height = h
}

func (o overlayModel) View() string {
	view := fmt.Sprintf("%s\n%s\n%s\n",  o.baseUrlInput.View(), o.tokenInput.View(), o.modelInput.View())

	if o.displayMsg != ""{
		view = fmt.Sprintf("%s\n%s", o.displayMsg, view)
	} else if o.baseUrlInput.Err != nil{
		view = fmt.Sprintf("%s\n%s", o.baseUrlInput.Err.Error(), view)
	}
	modalBox := lipgloss.NewStyle().
		Padding(1, 2).
		Width(o.width).
		// Height(o.height).
		Border(lipgloss.RoundedBorder()).
		Render(view)

	return modalBox
}
