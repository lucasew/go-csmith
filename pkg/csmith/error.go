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
// tests pass an explicit *Session (see ambient_test.go).
func GetErrorSess(s *Session) int { return sessOrAmbient(s).GenError }

// SetErrorSess sets GenError on an explicit session bag.
func SetErrorSess(s *Session, code int) { sessOrAmbient(s).GenError = code }

// ClearErrorSess clears GenError on an explicit session bag.
func ClearErrorSess(s *Session) { sessOrAmbient(s).GenError = ErrSuccess }

// HasErrorSess reports sticky error on an explicit session bag.
func HasErrorSess(s *Session) bool { return sessOrAmbient(s).GenError != ErrSuccess }

// sessNoteError writes GenError on an explicit bag. Nil s panics — residual
// sticky must not dual-fill any package ambient (unit tests pass explicit bags).
func sessNoteError(s *Session, code int) {
	if s == nil {
		panic("residual sessNoteError(nil)")
	}
	s.GenError = code
}

// noteErrFM notes sticky on fm.Sess. Nil fm: no sticky (fail-closed only — no ambient dual-fill).
func noteErrFM(fm *FactMgr, code int) {
	if fm == nil {
		// No bag: fail-closed returns only — do not dual-fill ambient.
		return
	}
	if fm.Sess == nil {
		panic("noteErrFM: Sess unset (use NewFactMgrSess)")
	}
	sessNoteError(fm.Sess, code)
}

// noteErrCG notes sticky on c.Sess. Nil c: no sticky (fail-closed only — no ambient dual-fill).
func noteErrCG(c *CGContext, code int) {
	if c == nil {
		// No bag: fail-closed returns only — do not dual-fill ambient.
		return
	}
	if c.Sess == nil {
		panic("noteErrCG: Sess unset (use WithSession / set Sess)")
	}
	sessNoteError(c.Sess, code)
}

// noteErrVS notes sticky on vs.Sess. Nil vs: no sticky (fail-closed only — no ambient dual-fill).
func noteErrVS(vs *VariableSelector, code int) {
	if vs == nil {
		// No bag: fail-closed returns only — do not dual-fill ambient.
		return
	}
	if vs.Sess == nil {
		panic("noteErrVS: Sess unset (use NewVariableSelector or set VS.Sess)")
	}
	sessNoteError(vs.Sess, code)
}

// noteErrG notes sticky on g.Sess. Nil g: no sticky (fail-closed only — no ambient dual-fill).
func noteErrG(g *ProgramGenerator, code int) {
	if g == nil {
		// No bag: fail-closed returns only — do not dual-fill ambient.
		return
	}
	if g.Sess == nil {
		panic("noteErrG: Sess unset (use NewProgramGenerator or set g.Sess)")
	}
	sessNoteError(g.Sess, code)
}

// noteErrEnv notes sticky on env.Sess. Nil env: no sticky (fail-closed only — no ambient dual-fill).
func noteErrEnv(env *TypeEnv, code int) {
	if env == nil {
		// No bag: fail-closed returns only — do not dual-fill ambient.
		return
	}
	if env.Sess == nil {
		panic("noteErrEnv: Sess unset (set TypeEnv.Sess)")
	}
	sessNoteError(env.Sess, code)
}

// hasErrFM reports sticky on fm.Sess. Nil fm: no sticky.
func hasErrFM(fm *FactMgr) bool {
	if fm == nil {
		return false // no bag → no sticky
	}
	if fm.Sess == nil {
		panic("hasErrFM: Sess unset (use NewFactMgrSess)")
	}
	return sessHasError(fm.Sess)
}

// hasErrCG reports sticky on c.Sess. Nil c: no sticky.
func hasErrCG(c *CGContext) bool {
	if c == nil {
		return false // no bag → no sticky
	}
	if c.Sess == nil {
		panic("hasErrCG: Sess unset (use WithSession / set Sess)")
	}
	return sessHasError(c.Sess)
}

// hasErrVS reports sticky on vs.Sess. Nil vs: no sticky.
func hasErrVS(vs *VariableSelector) bool {
	if vs == nil {
		return false // no bag → no sticky
	}
	if vs.Sess == nil {
		panic("hasErrVS: Sess unset (use NewVariableSelector or set VS.Sess)")
	}
	return sessHasError(vs.Sess)
}

// hasErrG reports sticky on g.Sess. Nil g: no sticky.
func hasErrG(g *ProgramGenerator) bool {
	if g == nil {
		return false // no bag → no sticky
	}
	if g.Sess == nil {
		panic("hasErrG: Sess unset (use NewProgramGenerator or set g.Sess)")
	}
	return sessHasError(g.Sess)
}

// hasErrEnv reports sticky on env.Sess. Nil env: no sticky.
func hasErrEnv(env *TypeEnv) bool {
	if env == nil {
		return false // no bag → no sticky
	}
	if env.Sess == nil {
		panic("hasErrEnv: Sess unset (set TypeEnv.Sess)")
	}
	return sessHasError(env.Sess)
}

// sessFromCG returns c.Sess. Nil owner → throwaway NewSession (no package ambient).
// Unset Sess on live c still panics via cgSess.
func sessFromCG(c *CGContext) *Session {
	if c == nil {
		// Library residual without owner: throwaway bag (no package ambient).
		return NewSession(Defaults())
	}
	return cgSess(c)
}

// sessFromFM returns fm.Sess. Nil owner → throwaway NewSession (no package ambient).
func sessFromFM(fm *FactMgr) *Session {
	if fm == nil {
		// Library residual without owner: throwaway bag (no package ambient).
		return NewSession(Defaults())
	}
	return fmSess(fm)
}

// sessFromVS returns vs.Sess. Nil owner → throwaway NewSession (no package ambient).
func sessFromVS(vs *VariableSelector) *Session {
	if vs == nil {
		// Library residual without owner: throwaway bag (no package ambient).
		return NewSession(Defaults())
	}
	return vsSess(vs)
}

// sessFromEnv returns env.Sess. Nil owner → throwaway NewSession (no package ambient).
func sessFromEnv(env *TypeEnv) *Session {
	if env == nil {
		// Library residual without owner: throwaway bag (no package ambient).
		return NewSession(Defaults())
	}
	return envSess(env)
}

// sessFromG returns g.Sess. Nil owner → throwaway NewSession (no package ambient).
func sessFromG(g *ProgramGenerator) *Session {
	if g == nil {
		// Library residual without owner: throwaway bag (no package ambient).
		return NewSession(Defaults())
	}
	return gSess(g)
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
