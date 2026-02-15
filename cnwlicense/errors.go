package cnwlicense

import (
	"errors"
	"fmt"
)

// Sentinel errors for license validation failures.
var (
	ErrLicenseNotFound = errors.New("license not found")
	ErrLicenseInactive = errors.New("license is not active")
	ErrLicenseExpired  = errors.New("license expired")
	ErrActivationLimit = errors.New("activation limit reached")
)

// Sentinel errors for offline license verification.
var (
	ErrSignatureInvalid   = errors.New("signature verification failed")
	ErrPublicKeyInvalid   = errors.New("invalid public key")
	ErrLicenseFileInvalid = errors.New("invalid license file format")
)

// Sentinel errors for hardware limit enforcement.
var (
	ErrCPULimitExceeded  = errors.New("CPU limit exceeded")
	ErrNodeLimitExceeded = errors.New("node limit exceeded")
)

// ServerError represents an error response from the CNW License Server.
// The server returns errors in the format: {"error": {"code": "...", "message": "..."}}.
type ServerError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("server error %d: [%s] %s", e.StatusCode, e.Code, e.Message)
}

// mapServerError converts a ServerError to a well-known sentinel error if possible.
// The returned error wraps both the sentinel error and the original ServerError
// so callers can use errors.Is() for sentinel checks and errors.As() for details.
func mapServerError(se *ServerError) error {
	var sentinel error
	switch se.Code {
	case "NOT_FOUND":
		sentinel = ErrLicenseNotFound
	case "FORBIDDEN":
		if se.Message == "license expired" {
			sentinel = ErrLicenseExpired
		} else {
			sentinel = ErrLicenseInactive
		}
	case "ACTIVATION_LIMIT":
		sentinel = ErrActivationLimit
	default:
		return se
	}
	return &mappedError{sentinel: sentinel, server: se}
}

// mappedError wraps a sentinel error with the original ServerError details.
type mappedError struct {
	sentinel error
	server   *ServerError
}

func (e *mappedError) Error() string {
	return e.sentinel.Error()
}

func (e *mappedError) Is(target error) bool {
	return target == e.sentinel
}

func (e *mappedError) As(target interface{}) bool {
	if t, ok := target.(**ServerError); ok {
		*t = e.server
		return true
	}
	return false
}

func (e *mappedError) Unwrap() error {
	return e.sentinel
}
