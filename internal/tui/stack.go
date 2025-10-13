package tui

import tea "github.com/charmbracelet/bubbletea"

// ComponentStack is a stack of components.
type ComponentStack struct {
	components []Component
}

// NewComponentStack creates a new component stack.
func NewComponentStack(initial ...Component) *ComponentStack {
	return &ComponentStack{
		components: initial,
	}
}

// Push adds a component to the top of the stack.
func (s *ComponentStack) Push(c Component) {
	s.components = append(s.components, c)
}

// Pop removes the top component if there is more than one component on the
// stack.
func (s *ComponentStack) Pop() {
	if len(s.components) > 1 {
		s.components = s.components[:len(s.components)-1]
	}
}

// Top returns the top component on the stack.
func (s *ComponentStack) Top() Component {
	if len(s.components) == 0 {
		return nil
	}
	return s.components[len(s.components)-1]
}

// IsConsumingInput returns true if any component on the stack is consuming input.
func (s *ComponentStack) IsConsumingInput() bool {
	for _, c := range s.components {
		if c.IsConsumingInput() {
			return true
		}
	}
	return false
}

// Update updates the top component on the stack.
func (s *ComponentStack) Update(msg tea.Msg) tea.Cmd {
	if len(s.components) == 0 {
		return nil
	}
	top := s.components[len(s.components)-1]
	newComp, cmd := top.Update(msg)
	if newComp != top {
		s.Push(newComp)
	}
	return cmd
}

// View returns the view of the top component on the stack.
func (s *ComponentStack) View() string {
	if len(s.components) == 0 {
		return ""
	}
	return s.components[len(s.components)-1].View()
}