package wifi

import "errors"

// ErrIncorrectPassphrase is returned when a connection fails due to an
// incorrect passphrase.
var ErrIncorrectPassphrase = errors.New("incorrect passphrase")

// ErrWirelessDisabled is returned when the wireless radio is disabled.
var ErrWirelessDisabled = errors.New("wireless disabled")

// ErrNotFound is returned when a network is not found.
var ErrNotFound = errors.New("not found")

// ErrNotAvailable is returned when a backend is not available.
var ErrNotAvailable = errors.New("not available")

// ErrOperationFailed is returned when an operation fails.
var ErrOperationFailed = errors.New("operation failed")

// ErrNotSupported is returned when a feature is not supported.
var ErrNotSupported = errors.New("not supported")
