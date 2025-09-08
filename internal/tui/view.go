package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shazow/wifitui/wifi"
)

//- Messages for stack navigation ----------------------------------------------

//- Messages for stack navigation ----------------------------------------------

// PushMsg is a message to push a new view onto the stack.
type PushMsg struct{ Model tea.Model }

// PopMsg is a message to pop a view from the stack.
type PopMsg struct{}

//- Messages for global state --------------------------------------------------

// SetStatusMsg is a message to set the status message on the root model.
type SetStatusMsg string

// SetLoadingMsg is a message to control the loading spinner on the root model.
type SetLoadingMsg struct {
	Loading bool
	Message string
}

// ShowErrorMsg is a message to show the error view.
type ShowErrorMsg struct{ Err error }

//- The stack model ------------------------------------------------------------

// Stack is a tea.Model that manages a stack of other tea.Models.
type Stack struct {
	views         []tea.Model
	backend       wifi.Backend
	spinner       spinner.Model
	loading       bool
	statusMessage string
	width, height int
}

// NewStack creates a new stack with an initial view.
func NewStack(backend wifi.Backend, initialView tea.Model) *Stack {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(CurrentTheme.Primary)

	return &Stack{
		backend: backend,
		views:   []tea.Model{initialView},
		spinner: s,
	}
}

// Init initializes the model at the top of the stack.
func (s *Stack) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, s.spinner.Tick)
	if s.Top() != nil {
		cmds = append(cmds, s.Top().Init())
	}
	return tea.Batch(cmds...)
}

// Update handles messages for the stack.
func (s *Stack) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// Handle stack-specific messages
	case PopMsg:
		s.Pop()
		if s.Top() == nil {
			return s, tea.Quit
		}
		return s, nil
	case PushMsg:
		s.Push(msg.Model)
		return s, s.Top().Init()

	// Handle global state messages
	case SetStatusMsg:
		s.statusMessage = string(msg)
		s.loading = false
	case SetLoadingMsg:
		s.loading = msg.Loading
		s.statusMessage = msg.Message
	case ShowErrorMsg:
		s.Push(NewErrorModel(msg.Err))
	case secretsLoadedMsg:
		s.Push(NewEditModel(s.backend, msg.Conn, msg.Password))
		return s, s.Top().Init()

	// Handle window size messages
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		// Propagate the window size message to all views on the stack
		for i, view := range s.views {
			updatedView, cmd := view.Update(msg)
			s.views[i] = updatedView
			cmds = append(cmds, cmd)
		}
		return s, tea.Batch(cmds...)
	}

	// Delegate all other messages to the top view
	if s.Top() != nil {
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = s.Top().Update(msg)
		s.views[len(s.views)-1] = model
		cmds = append(cmds, cmd)
	}

	// Always update the spinner
	var spinCmd tea.Cmd
	s.spinner, spinCmd = s.spinner.Update(msg)
	cmds = append(cmds, spinCmd)

	return s, tea.Batch(cmds...)
}

// View renders the view at the top of the stack.
func (s *Stack) View() string {
	var view strings.Builder
	if s.Top() != nil {
		view.WriteString(s.Top().View())
	}

	// Render global status/loading bar
	if s.loading {
		view.WriteString(fmt.Sprintf("\n\n%s %s", s.spinner.View(), lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(s.statusMessage)))
	} else if s.statusMessage != "" {
		view.WriteString(fmt.Sprintf("\n\n%s", lipgloss.NewStyle().Foreground(CurrentTheme.Primary).Render(s.statusMessage)))
	}

	return view.String()
}

// Push adds a view to the top of the stack.
func (s *Stack) Push(v tea.Model) {
	s.views = append(s.views, v)
}

// Pop removes and returns the view from the top of the stack.
func (s *Stack) Pop() tea.Model {
	if len(s.views) == 0 {
		return nil
	}
	v := s.views[len(s.views)-1]
	s.views = s.views[:len(s.views)-1]
	return v
}

// Top returns the view at the top of the stack without removing it.
func (s *Stack) Top() tea.Model {
	if len(s.views) == 0 {
		return nil
	}
	return s.views[len(s.views)-1]
}
