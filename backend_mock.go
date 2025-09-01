//go:build mock

package main

import (
	"github.com/shazow/wifitui/backend"
	mockBackend "github.com/shazow/wifitui/backend/mock"
)

func GetBackend() (backend.Backend, error) {
	return mockBackend.New()
}
