package tui

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

// Pop removes and returns the top component.
func (s *ComponentStack) Pop() (Component, bool) {
	if s.IsEmpty() {
		return nil, false
	}
	index := len(s.components) - 1
	component := s.components[index]
	s.components = s.components[:index]
	return component, true
}

// Top returns the top component without removing it.
// It will panic if the stack is empty. The TUI logic should guarantee that
// the stack is never empty.
func (s *ComponentStack) Top() Component {
	return s.components[len(s.components)-1]
}

// IsEmpty returns true if the stack is empty.
func (s *ComponentStack) IsEmpty() bool {
	return len(s.components) == 0
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

// Len returns the number of components on the stack.
func (s *ComponentStack) Len() int {
	return len(s.components)
}