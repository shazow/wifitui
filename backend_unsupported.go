//go:build !linux && !darwin

package main

import "fmt"

// NewBackend returns an error for unsupported operating systems.
func NewBackend() (Backend, error) {
	return nil, fmt.Errorf("unsupported operating system")
}
