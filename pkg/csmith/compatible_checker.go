// Upstream: CompatibleChecker.h / CompatibleChecker.cpp / Variable::compatible.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// CompatibleCheckExprVar mirrors CompatibleChecker::compatible_check(Variable*, Expression*).
// CompatibleChecker.cpp:43–53 — when disabled always false; else exp.compatible(v).
// Returns true when assignment should be rejected (COMPATIBLE_CHECK_ERROR).
func CompatibleCheckExprVar(opts Options, v *Variable, exp *Expression) bool {
	if !opts.CompatibleCheck || v == nil || exp == nil {
		return false
	}
	return exp.CompatibleWithVar(v, opts.ExpandStruct)
}

// CompatibleCheckExprs mirrors CompatibleChecker::compatible_check(Expression*, Expression*).
// CompatibleChecker.cpp:58–65.
func CompatibleCheckExprs(opts Options, a, b *Expression) bool {
	if !opts.CompatibleCheck || a == nil || b == nil {
		return false
	}
	return a.CompatibleWithExpr(b, opts.ExpandStruct) || b.CompatibleWithExpr(a, opts.ExpandStruct)
}
