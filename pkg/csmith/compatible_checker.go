// Upstream: CompatibleChecker.h / CompatibleChecker.cpp / Variable::compatible.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// compatibleCheckProcess mirrors CompatibleChecker::compatible_check_ static.
// Enabled by EnableCompatibleCheck (DFS resolve path) or opts.CompatibleCheck.
var compatibleCheckProcess bool

// EnableCompatibleCheck mirrors CompatibleChecker::enable_compatible_check.
// CompatibleChecker.cpp:68–70; CGOptions.cpp:416–417 (resolve_exhaustive_options).
func EnableCompatibleCheck() {
	compatibleCheckProcess = true
}

// ResetCompatibleCheck clears the process static (tests / finalization).
func ResetCompatibleCheck() {
	compatibleCheckProcess = false
}

// compatibleCheckOn is true when process static or option requests the checker.
func compatibleCheckOn(opts Options) bool {
	return compatibleCheckProcess || opts.CompatibleCheck
}

// CompatibleCheckExprVar mirrors CompatibleChecker::compatible_check(Variable*, Expression*).
// CompatibleChecker.cpp:43–53 — when disabled always false.
// Returns true when assignment should be rejected (COMPATIBLE_CHECK_ERROR).
func CompatibleCheckExprVar(opts Options, v *Variable, exp *Expression) bool {
	if !compatibleCheckOn(opts) {
		return false
	}
	// CompatibleChecker.cpp:46–49 — assert(v); assert(exp); assert(0)
	// This overload always aborts when enabled (dead code after assert).
	// Fail closed: reject assignment rather than invent exp.compatible(v).
	// incomplete IR rejects sticky (no invent "compatible OK" / soft re-pick)
	if v == nil || exp == nil {
		SetError(ErrGeneric)
		return true
	}
	// complete path always reject (C++ assert(0)); callers SetError(ErrCompatibleCheck)
	return true
}

// CompatibleCheckExprs mirrors CompatibleChecker::compatible_check(Expression*, Expression*).
// CompatibleChecker.cpp:58–65 — when disabled false; else bidirectional compatible.
func CompatibleCheckExprs(opts Options, a, b *Expression) bool {
	// CompatibleChecker.cpp:60–61 — assert(exp1); assert(exp2)
	if !compatibleCheckOn(opts) {
		return false
	}
	// incomplete Expression* fails closed sticky as reject (no invent non-error)
	if a == nil || b == nil {
		SetError(ErrGeneric)
		return true
	}
	if a.CompatibleWithExpr(b, opts.ExpandStruct) {
		// residual ERROR sticky — no invent reject-true past CompatibleWithExpr residual hole
		if HasError() {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue reverse check past residual false
	if HasError() {
		return true
	}
	if b.CompatibleWithExpr(a, opts.ExpandStruct) {
		// residual ERROR sticky — no invent reject-true past reverse CompatibleWithExpr residual
		if HasError() {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue no-reject past reverse residual false
	if HasError() {
		return true
	}
	return false
}
