package wifi

import "errors"

var (
	ErrNotSupported   = errors.New("not supported")
	ErrNotFound       = errors.New("not found")
	ErrNotAvailable   = errors.New("not available")
	ErrOperationFailed = errors.New("operation failed")
	ErrWirelessDisabled = errors.New("wireless is disabled")
)
