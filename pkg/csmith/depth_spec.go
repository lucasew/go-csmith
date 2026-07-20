// Upstream: DepthSpec.h / DepthSpec.cpp (depth_guard + minimal_depth tables).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Depth guard results (DepthSpec.h).
const (
	// GoodDepth is GOOD_DEPTH.
	GoodDepth = 0
	// BadDepth is BAD_DEPTH.
	BadDepth = -1
	// AtomicDepthIncr mirrors ATOMIC_DEPTH_INCR used in param-list depth.
	AtomicDepthIncr = 1
)

// Depth type name keys used by DepthGuardByType (subset of dType enum).
const (
	DtFunction                     = "dtFunction"
	DtFirstFunction                = "dtFirstFunction"
	DtBlock                        = "dtBlock"
	DtStatement                    = "dtStatement"
	DtStatementAssign              = "dtStatementAssign"
	DtStatementFor                 = "dtStatementFor"
	DtStatementIf                  = "dtStatementIf"
	DtStatementExpr                = "dtStatementExpr"
	DtStatementReturn              = "dtStatementReturn"
	DtExpression                   = "dtExpression"
	DtExpressionRandomParam        = "dtExpressionRandomParam"
	DtExpressionVariable           = "dtExpressionVariable"
	DtExpressionFuncall            = "dtExpressionFuncall"
	DtLhs                          = "dtLhs"
	DtReturnType                   = "dtReturnType"
	DtFunctionGenerateBody         = "dtFunctionGenerateBody"
	DtGenerateParamList            = "dtGenerateParamList"
	DtTypeChooseSimple             = "dtTypeChooseSimple"
	DtRandomTypeFromType           = "dtRandomTypeFromType"
	DtSelectVariable               = "dtSelectVariable"
	DtSelectLValue                 = "dtSelectLValue"
	DtConstant                     = "dtConstant"
	DtLoopControl                  = "dtLoopControl"
	DtVariableSelection            = "dtVariableSelection"
	DtInitVariable                 = "dtInitVariable"
	DtSafeOpFlags                  = "dtSafeOpFlags"
	DtFunctionInvocationRandom       = "dtFunctionInvocationRandom"
	DtFunctionInvocationRandomUnary  = "dtFunctionInvocationRandomUnary"
	DtFunctionInvocationRandomBinary = "dtFunctionInvocationRandomBinary"
	DtFunctionInvocationBinary       = "dtFunctionInvocationBinary"
	DtFunctionInvocationUnary        = "dtFunctionInvocationUnary"
	DtGenerateNewVariable            = "dtGenerateNewVariable"
	DtSelectGlobal                 = "dtSelectGlobal"
	DtGenerateNewGlobal            = "dtGenerateNewGlobal"
	DtSelectParentLocal      = "dtSelectParentLocal"
	DtGenerateNewParentLocal = "dtGenerateNewParentLocal"
	DtSelectDerefPointer     = "dtSelectDerefPointer"
	DtInitPointerValue       = "dtInitPointerValue"
)

// MinimalDepth returns DepthSpec::*minimal_depth for a dType name.
// DepthSpec.cpp atomic and composed depths (flag ignored except documented cases).
// Used when dfs_exhaustive; random mode guards ignore the value.
func MinimalDepth(dType string, flag int) int {
	switch dType {
	case DtConstant:
		return 0
	case DtVariableSelection, DtTypeChooseSimple, "dtTypeNonVoidSimple",
		"dtTypeChooseRandom", "dtChooseRandomPointerType", DtInitVariable:
		return 1
	case DtLoopControl:
		return 3
	case DtLhs:
		return 1
	case DtSelectDerefPointer:
		return 0
	case DtExpressionVariable, DtSelectLValue, DtSelectVariable:
		// flag == MAX_VAR_SCOPE → +1
		base := 1
		if flag == int(MaxVarScope) {
			return base + 1
		}
		return base
	case DtExpression, DtExpressionRandomParam:
		// flag == MAX_TERM_TYPES → +1
		base := 0 // constant
		if flag == int(MaxTermTypes) {
			return base + 1
		}
		return base
	case DtStatementReturn:
		return 1
	case DtStatement:
		// dtStatementReturn + 1; MAX_STATEMENT_TYPE flag → +1 more
		base := 1 + 1 // return + 1
		if flag == int(MaxStatementType) {
			return base + 1
		}
		return base
	case DtBlock, DtFunctionGenerateBody:
		return MinimalDepth(DtStatement, 0) + 1
	case DtStatementAssign, DtStatementFor, DtStatementIf, DtStatementExpr:
		return MinimalDepth(DtStatement, 0)
	case DtReturnType, "dtRandomTypeFromType":
		return 1
	case DtGenerateParamList:
		return AtomicDepthIncr + 1
	case DtFunction:
		return MinimalDepth(DtGenerateParamList, 0) + MinimalDepth(DtFunctionGenerateBody, 0)
	case DtFirstFunction:
		return MinimalDepth(DtReturnType, 0) + MinimalDepth(DtFunctionGenerateBody, 0)
	case DtSafeOpFlags:
		// DepthSpec.cpp — sOpBinary → 2; sOpUnary (and assign) → 3
		// SafeOpKind: sOpUnary=0, sOpBinary=1, sOpAssign=2
		if flag == int(SafeOpBinary) {
			return 2
		}
		return 3
	case DtInitPointerValue:
		// DepthSpec: init pointer may recurse into create local/global
		return 1
	case DtGenerateNewGlobal:
		return 1 + 1 // 1 + init
	case DtSelectGlobal:
		return 1 + 1
	case DtSelectParentLocal:
		return 1 + 1
	case DtGenerateNewParentLocal:
		return 2 + 1
	case DtGenerateNewVariable:
		// min(parentLocal, global) + 1
		a := MinimalDepth(DtGenerateNewParentLocal, 0)
		b := MinimalDepth(DtGenerateNewGlobal, 0)
		if a <= b {
			return a + 1
		}
		return b + 1
	case DtExpressionFuncall, DtFunctionInvocationRandom,
		DtFunctionInvocationRandomUnary, DtFunctionInvocationRandomBinary,
		DtFunctionInvocationBinary, DtFunctionInvocationUnary:
		return 1
	default:
		// DepthSpec.cpp:381–382 assert(0) for unknown dType — sticky no invent depth 1
		SetError(ErrGeneric)
		return -1
	}
}

// knownDepthType reports whether dType is a handled DepthSpec case.
func knownDepthType(dType string) bool {
	d := MinimalDepth(dType, 0)
	// residual ERROR sticky — no invent known-true past MinimalDepth residual hole
	// (unknown dType SetError + -1; residual must not invent known via soft >=0)
	if HasError() {
		return false
	}
	return d >= 0
}

// DepthGuardByDepth mirrors DepthSpec::depth_guard_by_depth.
// DepthSpec.cpp:330–335 — always GOOD_DEPTH when !dfs_exhaustive (random mode).
func DepthGuardByDepth(opts Options, depthNeeded int) int {
	if !opts.DFSExhaustive {
		return GoodDepth
	}
	// DFS: would call eager_backtracking(depthNeeded); no DFS engine → GOOD
	_ = depthNeeded
	return GoodDepth
}

// DepthGuardByType mirrors DepthSpec::depth_guard_by_type.
// DepthSpec.cpp:337+ — always GOOD when !dfs_exhaustive; else backtracking(minimal).
func DepthGuardByType(opts Options, dType string) int {
	return DepthGuardByTypeFlag(opts, dType, 0)
}

// DepthGuardByTypeFlag is depth_guard_by_type with extra_flag for MAX_* cases.
func DepthGuardByTypeFlag(opts Options, dType string, flag int) int {
	if !opts.DFSExhaustive {
		return GoodDepth
	}
	// DepthSpec.cpp:381–382 — unknown dType assert(0) → BAD_DEPTH sticky fail closed
	if MinimalDepth(dType, flag) < 0 {
		// MinimalDepth already SetError on unknown; ensure sticky if that path skipped
		if !HasError() {
			SetError(ErrGeneric)
		}
		return BadDepth
	}
	// DFS backtracking not implemented; known types report GOOD (no false BAD)
	return GoodDepth
}
