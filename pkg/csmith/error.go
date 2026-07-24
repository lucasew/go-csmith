// Upstream: Error.h / Error.cpp (generation error codes).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Error codes mirror Error.h macros.
const (
	ErrSuccess              = 0
	ErrGeneric              = -1
	ErrExceedMaxDepth       = -2
	ErrFilter               = -3
	ErrBacktracking         = -4
	ErrCompatibleCheck      = -5
	ErrInvalidSimpleDelta   = -6
)

// GetError mirrors Error::get_error (session-local).
func GetError() int { return currentSession().GenError }

// SetError mirrors Error::set_error.
func SetError(code int) { currentSession().GenError = code }

// ClearError mirrors resetting to SUCCESS.
func ClearError() { currentSession().GenError = ErrSuccess }

// HasError is true when get_error() != SUCCESS.
func HasError() bool { return currentSession().GenError != ErrSuccess }
