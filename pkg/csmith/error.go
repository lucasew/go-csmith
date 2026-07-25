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

// GetErrorSess returns GenError on an explicit session bag.
// Non-Sess GetError/SetError/ClearError/HasError bridges are deleted — unit
// tests pass testAmbientSession explicitly.
func GetErrorSess(s *Session) int { return sessOrAmbient(s).GenError }

// SetErrorSess sets GenError on an explicit session bag.
func SetErrorSess(s *Session, code int) { sessOrAmbient(s).GenError = code }

// ClearErrorSess clears GenError on an explicit session bag.
func ClearErrorSess(s *Session) { sessOrAmbient(s).GenError = ErrSuccess }

// HasErrorSess reports sticky error on an explicit session bag.
func HasErrorSess(s *Session) bool { return sessOrAmbient(s).GenError != ErrSuccess }

// sessNoteError writes GenError on an explicit bag. Nil s panics — residual
// sticky must not dual-fill testAmbientSession (unit tests use *Sess helpers
// or pass testAmbientSession / vsSess/cgSess/envSess/fmSess).
func sessNoteError(s *Session, code int) {
	if s == nil {
		panic("residual sessNoteError(nil)")
	}
	s.GenError = code
}

// sessHasError reports sticky error on an explicit bag. Nil s panics.
func sessHasError(s *Session) bool {
	if s == nil {
		panic("residual sessHasError(nil)")
	}
	return s.GenError != ErrSuccess
}

// sessClearError clears sticky error on an explicit bag. Nil s panics.
func sessClearError(s *Session) {
	if s == nil {
		panic("residual sessClearError(nil)")
	}
	s.GenError = ErrSuccess
}

// sessErrorCode returns sticky code on an explicit bag. Nil s panics.
func sessErrorCode(s *Session) int {
	if s == nil {
		panic("residual sessErrorCode(nil)")
	}
	return s.GenError
}
