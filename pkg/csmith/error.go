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
func GetError() int { return GetErrorSess(testAmbientSession) }

// GetErrorSess returns GenError on an explicit session bag.
func GetErrorSess(s *Session) int { return sessOrAmbient(s).GenError }

// SetError mirrors Error::set_error.
func SetError(code int) { SetErrorSess(testAmbientSession, code) }

// SetErrorSess sets GenError on an explicit session bag.
func SetErrorSess(s *Session, code int) { sessOrAmbient(s).GenError = code }

// ClearError mirrors resetting to SUCCESS.
func ClearError() { ClearErrorSess(testAmbientSession) }

// ClearErrorSess clears GenError on an explicit session bag.
func ClearErrorSess(s *Session) { sessOrAmbient(s).GenError = ErrSuccess }

// HasError is true when get_error() != SUCCESS.
func HasError() bool { return HasErrorSess(testAmbientSession) }

// HasErrorSess reports sticky error on an explicit session bag.
func HasErrorSess(s *Session) bool { return sessOrAmbient(s).GenError != ErrSuccess }

// sessNoteError writes GenError on s when non-nil; nil s writes the quarantined
// unit-test ambient bag for residual hang-prevention on Filter loops (unit tests
// without an explicit bag). Prefer explicit s so pure Generate never touches ambient.
func sessNoteError(s *Session, code int) {
	if s != nil {
		s.GenError = code
		return
	}
	testAmbientSession.GenError = code
}

// sessHasError reports sticky error on s when non-nil, else unit-test ambient.
// Prefer explicit s for pure-session ERROR_GUARD; ambient nil path is unit-test only.
func sessHasError(s *Session) bool {
	if s != nil {
		return s.GenError != ErrSuccess
	}
	return testAmbientSession.GenError != ErrSuccess
}

// sessClearError clears sticky error on s when non-nil; nil s clears ambient.
func sessClearError(s *Session) {
	if s != nil {
		s.GenError = ErrSuccess
		return
	}
	testAmbientSession.GenError = ErrSuccess
}

// sessErrorCode returns sticky code on s when non-nil; nil s reads ambient.
func sessErrorCode(s *Session) int {
	if s != nil {
		return s.GenError
	}
	return testAmbientSession.GenError
}
