// Upstream: Error.h / Error.cpp (generation error codes).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Error codes mirror Error.h macros.
const (
	ErrSuccess            = 0
	ErrGeneric            = -1
	ErrExceedMaxDepth     = -2
	ErrFilter             = -3
	ErrBacktracking       = -4
	ErrCompatibleCheck    = -5
	ErrInvalidSimpleDelta = -6
)

// GetError mirrors Error::get_error (session-local).
func GetError() int { return GetErrorSess(nil) }

// GetErrorSess returns GenError on an explicit session bag.
func GetErrorSess(s *Session) int { return sessOrAmbient(s).GenError }

// SetError mirrors Error::set_error.
func SetError(code int) { SetErrorSess(nil, code) }

// SetErrorSess sets GenError on an explicit session bag.
func SetErrorSess(s *Session, code int) { sessOrAmbient(s).GenError = code }

// ClearError mirrors resetting to SUCCESS.
func ClearError() { ClearErrorSess(nil) }

// ClearErrorSess clears GenError on an explicit session bag.
func ClearErrorSess(s *Session) { sessOrAmbient(s).GenError = ErrSuccess }

// HasError is true when get_error() != SUCCESS.
func HasError() bool { return HasErrorSess(nil) }

// HasErrorSess reports sticky error on an explicit session bag.
func HasErrorSess(s *Session) bool { return sessOrAmbient(s).GenError != ErrSuccess }

// sessNoteError writes GenError on s when non-nil; nil s targets the Process*
// ambient bag (defaultSession / activeSession). Explicit bags no longer dual-sync
// ambient — pure-session sticky stays on the bag (Generate still activates s so
// residual Process* readers that share the active bag continue to see it).
func sessNoteError(s *Session, code int) {
	if s != nil {
		s.GenError = code
		return
	}
	sessOrAmbient(nil).GenError = code
}

// sessHasError reports sticky error on s when non-nil, else ambient.
func sessHasError(s *Session) bool {
	if s != nil {
		return s.GenError != ErrSuccess
	}
	return sessOrAmbient(nil).GenError != ErrSuccess
}

// sessClearError clears sticky error on s when non-nil, else ambient.
func sessClearError(s *Session) {
	if s != nil {
		s.GenError = ErrSuccess
		return
	}
	sessOrAmbient(nil).GenError = ErrSuccess
}

// sessErrorCode returns sticky code on s when non-nil, else ambient.
func sessErrorCode(s *Session) int {
	if s != nil {
		return s.GenError
	}
	return sessOrAmbient(nil).GenError
}
