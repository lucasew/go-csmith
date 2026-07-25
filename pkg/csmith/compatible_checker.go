// Upstream: CompatibleChecker.h / CompatibleChecker.cpp / Variable::compatible.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// currentSession().CompatibleCheck mirrors CompatibleChecker::compatible_check_ static.
// Enabled by EnableCompatibleCheck (DFS resolve path) or opts.CompatibleCheck.

// EnableCompatibleCheck mirrors CompatibleChecker::enable_compatible_check.
// CompatibleChecker.cpp:68–70; CGOptions.cpp:416–417 (resolve_exhaustive_options).
func EnableCompatibleCheck() {
	EnableCompatibleCheckSess(testAmbientSession)
}

// EnableCompatibleCheckSess enables the checker on an explicit session bag.
func EnableCompatibleCheckSess(s *Session) {
	sessOrAmbient(s).CompatibleCheck = true
}

// ResetCompatibleCheck clears the process static (tests / finalization).
func ResetCompatibleCheck() {
	ResetCompatibleCheckSess(testAmbientSession)
}

// ResetCompatibleCheckSess clears the checker flag on an explicit session bag.
func ResetCompatibleCheckSess(s *Session) {
	sessOrAmbient(s).CompatibleCheck = false
}

// compatibleCheckOn is true when option or session static requests the checker.
func compatibleCheckOn(opts Options) bool {
	return compatibleCheckOnSess(testAmbientSession, opts)
}

// compatibleCheckOnSess is compatibleCheckOn with an explicit session bag.
func compatibleCheckOnSess(s *Session, opts Options) bool {
	if opts.CompatibleCheck {
		return true
	}
	return sessOrAmbient(s).CompatibleCheck
}

// CompatibleCheckExprVar mirrors CompatibleChecker::compatible_check(Variable*, Expression*).
// CompatibleChecker.cpp:43–53 — when disabled always false.
// Returns true when assignment should be rejected (COMPATIBLE_CHECK_ERROR).
func CompatibleCheckExprVar(opts Options, v *Variable, exp *Expression) bool {
	return CompatibleCheckExprVarSess(testAmbientSession, opts, v, exp)
}

// CompatibleCheckExprVarSess is CompatibleCheckExprVar on bag s.
func CompatibleCheckExprVarSess(s *Session, opts Options, v *Variable, exp *Expression) bool {
	if !compatibleCheckOnSess(s, opts) {
		return false
	}
	// CompatibleChecker.cpp:46–49 — assert(v); assert(exp); assert(0)
	// This overload always aborts when enabled (dead code after assert).
	// Fail closed: reject assignment rather than invent exp.compatible(v).
	// incomplete IR rejects sticky (no invent "compatible OK" / soft re-pick)
	if v == nil || exp == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	// complete path always reject (C++ assert(0)); callers sessNoteError(..., ErrCompatibleCheck)
	return true
}

// CompatibleCheckExprs mirrors CompatibleChecker::compatible_check(Expression*, Expression*).
// CompatibleChecker.cpp:58–65 — when disabled false; else bidirectional compatible.
func CompatibleCheckExprs(opts Options, a, b *Expression) bool {
	return CompatibleCheckExprsSess(testAmbientSession, opts, a, b)
}

// CompatibleCheckExprsSess is CompatibleCheckExprs on bag s.
func CompatibleCheckExprsSess(s *Session, opts Options, a, b *Expression) bool {
	// CompatibleChecker.cpp:60–61 — assert(exp1); assert(exp2)
	if !compatibleCheckOnSess(s, opts) {
		return false
	}
	// incomplete Expression* fails closed sticky as reject (no invent non-error)
	if a == nil || b == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if a.CompatibleWithExpr(b, opts.ExpandStruct) {
		// residual ERROR sticky — no invent reject-true past CompatibleWithExpr residual hole
		if sessHasError(s) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue reverse check past residual false
	if sessHasError(s) {
		return true
	}
	if b.CompatibleWithExpr(a, opts.ExpandStruct) {
		// residual ERROR sticky — no invent reject-true past reverse CompatibleWithExpr residual
		if sessHasError(s) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue no-reject past reverse residual false
	if sessHasError(s) {
		return true
	}
	return false
}
