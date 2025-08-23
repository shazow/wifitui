//go:build linux

package main

// NewBackend determines which backend to use on Linux.
// It first tries NetworkManager, then falls back to iwd.
func NewBackend() (Backend, error) {
	backend, err := NewDBusBackend()
	if err == nil {
		return backend, nil
	}
	// If DBus backend failed, try the iwd backend
	return NewIwdBackend()
}
