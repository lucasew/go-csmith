// Upstream: CompatibleChecker.h / CompatibleChecker.cpp / Variable::compatible.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// currentSession().CompatibleCheck mirrors CompatibleChecker::compatible_check_ static.
// Enabled by EnableCompatibleCheck (DFS resolve path) or opts.CompatibleCheck.

// EnableCompatibleCheck mirrors CompatibleChecker::enable_compatible_check.
// CompatibleChecker.cpp:68–70; CGOptions.cpp:416–417 (resolve_exhaustive_options).
// Non-Sess EnableCompatibleCheck deleted — pass run bag or testAmbientSession explicitly.

// EnableCompatibleCheckSess enables the checker on an explicit session bag.
func EnableCompatibleCheckSess(s *Session) {
	sessOrAmbient(s).CompatibleCheck = true
}

// ResetCompatibleCheck clears the process static (tests / finalization).
// Non-Sess ResetCompatibleCheck deleted — pass run bag or testAmbientSession explicitly.

// ResetCompatibleCheckSess clears the checker flag on an explicit session bag.
func ResetCompatibleCheckSess(s *Session) {
	sessOrAmbient(s).CompatibleCheck = false
}

// compatibleCheckOn is true when CompatibleChecker::compatible_check_ is live.
// CompatibleChecker.cpp:50/63 — gated by static set only via enable_compatible_check().
// That enable is called from CGOptions::resolve_exhaustive_options only when
// dfs_exhaustive (CGOptions.cpp:410–417). CLI --compatible-check alone does NOT
// enable the checker in random mode (golden NDEBUG binary matches).
// Session.CompatibleCheck mirrors the C++ static (EnableCompatibleCheckSess).
// Non-Sess compatibleCheckOn deleted — pass run bag or testAmbientSession explicitly.

// compatibleCheckOnSess is compatibleCheckOn with an explicit session bag.
func compatibleCheckOnSess(s *Session, opts Options) bool {
	_ = opts // CLI flag alone is not the gate; see comment above
	if s == nil {
		return false
	}
	return s.CompatibleCheck
}

// CompatibleCheckExprVar mirrors CompatibleChecker::compatible_check(Variable*, Expression*).
// CompatibleChecker.cpp:43–53 — when disabled always false.
// Returns true when assignment should be rejected (COMPATIBLE_CHECK_ERROR).
// Non-Sess CompatibleCheckExprVar deleted — pass run bag or testAmbientSession explicitly.

// CompatibleCheckExprVarSess is CompatibleCheckExprVar on bag s.
func CompatibleCheckExprVarSess(s *Session, opts Options, v *Variable, exp *Expression) bool {
	if !compatibleCheckOnSess(s, opts) {
		return false
	}
	// CompatibleChecker.cpp:46–52 — assert(v); assert(exp); assert(0);
	// NDEBUG (golden bodyparity): assert elided → return exp->compatible(v).
	// incomplete IR rejects sticky (no invent "compatible OK" / soft re-pick)
	if v == nil || exp == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	return exp.CompatibleWithVarSess(s, v, opts.ExpandStruct)
}

// CompatibleCheckExprs mirrors CompatibleChecker::compatible_check(Expression*, Expression*).
// CompatibleChecker.cpp:58–65 — when disabled false; else bidirectional compatible.
// Non-Sess CompatibleCheckExprs deleted — pass run bag or testAmbientSession explicitly.

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
	if a.CompatibleWithExprSess(s, b, opts.ExpandStruct) {
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
	if b.CompatibleWithExprSess(s, a, opts.ExpandStruct) {
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
