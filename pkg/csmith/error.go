// Upstream: Error.h / Error.cpp (generation error codes).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Error codes mirror Error.h macros.
const (
	// ErrSuccess is SUCCESS.
	ErrSuccess = 0
	// ErrGeneric is ERROR.
	ErrGeneric = -1
	// ErrExceedMaxDepth is EXCEED_MAX_DEPTH_ERROR.
	ErrExceedMaxDepth = -2
	// ErrFilter is FILTER_ERROR.
	ErrFilter = -3
	// ErrBacktracking is BACKTRACKING_ERROR.
	ErrBacktracking = -4
	// ErrCompatibleCheck is COMPATIBLE_CHECK_ERROR.
	ErrCompatibleCheck = -5
	// ErrInvalidSimpleDelta is INVALID_SIMPLE_DELTA_SEQUENCE.
	ErrInvalidSimpleDelta = -6
)

// genError mirrors Error::r_error_.
var genError int

// GetError mirrors Error::get_error.
func GetError() int { return genError }

// SetError mirrors Error::set_error.
func SetError(code int) { genError = code }

// ClearError mirrors resetting to SUCCESS.
func ClearError() { genError = ErrSuccess }

// HasError is true when get_error() != SUCCESS.
func HasError() bool { return genError != ErrSuccess }
