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
	inputs     []TaggedInput
	focusIndex int
	width      int
	height     int
	displayMsg string
}

type TaggedInput struct {
	key   string
	model textinput.Model
}

func NewOverlay(withBaseUrl, withToken, withModel bool) overlayModel {
	inputs := make([]TaggedInput, 0, 3)
	if withBaseUrl {
		bu := textinput.New()
		bu.Prompt = ""
		bu.Placeholder = "Base Url"
		// bubbles/textinput v0.21.0 truncates placeholders to 1 rune when Width <= 0.
		// Give a sensible default; we'll resize it in SetWidth on window size events.
		bu.Width = 40
		bu.Validate = func(s string) error {
			if len(s) <= 7 || strings.HasPrefix(s, "https://") {
				return nil
			}
			return fmt.Errorf("Invalid schema")
		}
		inputs = append(inputs, TaggedInput{"baseurl", bu})
	}

	if withToken {
		ti := textinput.New()
		ti.Placeholder = "Token"
		ti.Prompt = ""
		ti.Width = 40
		inputs = append(inputs, TaggedInput{"token", ti})
	}

	if withModel {
		mi := textinput.New()
		mi.Placeholder = "Model"
		mi.Prompt = ""
		mi.Width = 40
		inputs = append(inputs, TaggedInput{"model", mi})
	}
	for i := range inputs {
		inputs[i].model.Focus()
		break
	}
	return overlayModel{
		inputs: inputs,
	}
}
func (o overlayModel) Init() tea.Cmd {
	return nil
}

func (o overlayModel) GetInputs() Provider {
	p := Provider{}

	for _, i := range o.inputs {
		switch i.key {
		case "baseurl":
			p.BaseURL = i.model.Value()
		case "token":
			p.Token = i.model.Value()
		case "model":
			p.Model = i.model.Value()
		}
	}
	return p
}

func (o overlayModel) IsValid() bool {
	for _, i := range o.inputs {
		if i.model.Value() == "" {
			return false
		}
	}
	return true
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
			// // Cycle indexes
			if s == "up" || s == "shift+tab" {
				o.focusIndex--
			} else {
				o.focusIndex++
			}

			if o.focusIndex >= len(o.inputs) {
				o.focusIndex = 0
			} else if o.focusIndex < 0 {
				o.focusIndex = len(o.inputs) - 1
			}
			log.Println(o.focusIndex)

			for idx := range o.inputs {
				if idx == o.focusIndex {
					cmd = o.inputs[idx].model.Focus()
				} else {
					o.inputs[idx].model.Blur()
				}
			}

			return o, cmd
		}
	}

	for i, input := range o.inputs {
		m, cmd := input.model.Update(msg)
		o.inputs[i].model = m
		cmds = append(cmds, cmd)
	}
	return o, tea.Batch(cmds...)
}

func (o *overlayModel) SetWidth(w int) {
	o.width = w
	// Keep the text inputs within the modal box.
	// Border is 1 char on each side, plus horizontal padding of 2 on each side.
	inner := max(1, w-6)
	for _, input := range o.inputs {
		input.model.Width = inner
	}
}

func (o *overlayModel) SetHeight(h int) {
	o.height = h
}

func (o overlayModel) View() string {
	var base strings.Builder
	for _, i := range o.inputs {
		fmt.Fprintf(&base, "%s\n", i.model.View())
	}

	var view strings.Builder
	if o.displayMsg != "" {
		fmt.Fprintf(&view, "%s\n", o.displayMsg)
	} else {
		for _, i := range o.inputs {
			item := i.model
			if item.Err != nil {
				fmt.Fprintf(&view, "%s\n", item.Err.Error())
			}
		}
	}
	fmt.Fprintf(&view, "%s", base.String())
	modalBox := lipgloss.NewStyle().
		Padding(1, 2).
		Width(o.width).
		// Height(o.height).
		Border(lipgloss.RoundedBorder()).
		Render(view.String())

	return modalBox
}
