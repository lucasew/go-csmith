// Upstream: Bookkeeper.h / Bookkeeper.cpp (generation statistics counters + tail dump).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// Bookkeeper counters live on Session.BK (session.go).

// sessBK returns the bookkeeper counters for s, or the ambient session bag.
func sessBK(s *Session) *bookkeeperState {
	if s != nil {
		return &s.BK
	}
	return &currentSession().BK
}

// IncrCounter mirrors incr_counter — grow vector and ++ at pos.
// Bookkeeper.cpp:527–537.
// Counters always live; sticky (no invent soft-skip stats past hole).
// pos < 0 is complete no-op (out-of-range index, not hard IR).
func IncrCounter(counters *[]int, pos int) {
	if counters == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if pos < 0 {
		return
	}
	for len(*counters) <= pos {
		*counters = append(*counters, 0)
	}
	(*counters)[pos]++
}

// CalcTotal mirrors calc_total.
// Bookkeeper.cpp:539–545.
func CalcTotal(counters []int) int {
	total := 0
	for _, c := range counters {
		total += c
	}
	return total
}

// BookkeeperDoFinalization mirrors Bookkeeper::doFinalization.
// Bookkeeper.cpp:116–126 (subset of cleared fields; full reset of all counters).
func BookkeeperDoFinalization() { BookkeeperDoFinalizationSess(nil) }

// BookkeeperDoFinalizationSess clears bookkeeper counters on s (or ambient).
func BookkeeperDoFinalizationSess(s *Session) {
	bk := sessBK(s)
	bk.structDepthCnts = nil
	bk.unionVarCnt = 0
	bk.exprDepthCnts = nil
	bk.blkDepthCnts = nil
	bk.dereferenceLevelCnts = nil
	bk.addressTakenCnt = 0
	bk.writeDereferenceCnts = nil
	bk.readDereferenceCnts = nil
	bk.cmpPtrToNull = 0
	bk.cmpPtrToPtr = 0
	bk.cmpPtrToAddr = 0
	bk.readVolatileCnt = 0
	bk.writeVolatileCnt = 0
	bk.readNonVolatileCnt = 0
	bk.writeNonVolatileCnt = 0
	bk.readVolatileThruPtrCnt = 0
	bk.writeVolatileThruPtrCnt = 0
	bk.pointerAvailForDeref = 0
	bk.volatileAvail = 0
	bk.structsWithBitfields = 0
	bk.varsWithBitfields = nil
	bk.varsWithFullBitfields = nil
	bk.varsWithBitfieldsAddressTakenCnt = 0
	bk.bitfieldsInTotal = 0
	bk.unamedBitfieldsInTotal = 0
	bk.constBitfieldsInTotal = 0
	bk.volatileBitfieldsInTotal = 0
	bk.lhsBitfieldsStructsVarsCnt = 0
	bk.rhsBitfieldsStructsVarsCnt = 0
	bk.lhsBitfieldCnt = 0
	bk.rhsBitfieldCnt = 0
	bk.forwardJumpCnt = 0
	bk.backwardJumpCnt = 0
	bk.useNewVarCnt = 0
	bk.useOldVarCnt = 0
	bk.oobCnt = 0
	bk.relyOnIntSize = false
	bk.relyOnPtrSize = false
}

func formattedOutput(b *strings.Builder, msg string, num int) {
	// Bookkeeper.cpp always has live ostream + message; sticky no invent silent
	// skip of stats lines (undercount) past missing builder shell
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if msg == "" {
		sessNoteError(nil, ErrGeneric)
		return
	}
	b.WriteString("XXX ")
	b.WriteString(msg)
	b.WriteString(fmt.Sprintf("%d\n", num))
}

func formattedOutputf(b *strings.Builder, msg string, num float64) {
	// Bookkeeper.cpp always has live ostream + message; sticky no invent silent
	// skip of stats lines past missing builder shell
	if b == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if msg == "" {
		sessNoteError(nil, ErrGeneric)
		return
	}
	b.WriteString("XXX ")
	b.WriteString(msg)
	// Bookkeeper uses default precision; ~3 digits like upstream out.precision(3)
	b.WriteString(fmt.Sprintf("%g\n", num))
}

// RecordAddressTaken mirrors Bookkeeper::record_address_taken.
// Bookkeeper.cpp:324–334.
func RecordAddressTaken(v *Variable) { RecordAddressTakenSess(nil, v) }

// RecordAddressTakenSess is RecordAddressTaken on an explicit session bag.
func RecordAddressTakenSess(s *Session, v *Variable) {
	// Bookkeeper.cpp:325–326 — assert(var); assert(var->type) sticky
	// (no invent skip broken IR as zero address-taken stats / soft re-pick)
	if v == nil || v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	v.IsAddrTaken = true
	sessBK(s).addressTakenCnt++
	if v.Type.HasBitfields() {
		// residual ERROR sticky — no invent soft-count past HasBitfields hole
		if sessHasError(s) {
			return
		}
		sessBK(s).varsWithBitfieldsAddressTakenCnt++
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent soft-skip bitfield count past HasBitfields residual false
		return
	}
}

// RecordVolatileAccess mirrors Bookkeeper::record_volatile_access.
// Bookkeeper.cpp:386–412.
func RecordVolatileAccess(v *Variable, derefLevel int, write bool) {
	RecordVolatileAccessSess(nil, v, derefLevel, write)
}

// RecordVolatileAccessSess is RecordVolatileAccess on an explicit session bag.
func RecordVolatileAccessSess(s *Session, v *Variable, derefLevel int, write bool) {
	// Bookkeeper.cpp:388 — assert(var) sticky
	// (no invent skip nil as zero vol-access stats)
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if write {
		RecordBitfieldsWritesSess(s, v)
	} else {
		RecordBitfieldsReadsSess(s, v)
	}
	bk := sessBK(s)
	for i := 0; i <= derefLevel; i++ {
		vol := v.IsVolatileAfterDeref(i)
		// residual ERROR sticky — no invent soft-continue peel stats past IsVolatileAfterDeref hole
		if sessHasError(s) {
			return
		}
		if write {
			if vol {
				if i > 0 {
					bk.writeVolatileThruPtrCnt++
				}
				bk.writeVolatileCnt++
			} else {
				bk.writeNonVolatileCnt++
			}
		} else {
			if vol {
				if i > 0 {
					bk.readVolatileThruPtrCnt++
				}
				bk.readVolatileCnt++
			} else {
				bk.readNonVolatileCnt++
			}
		}
	}
}

// RecordBitfieldsReads mirrors Bookkeeper::record_bitfields_reads.
// Bookkeeper.cpp:336–345.
// RecordBitfieldsReads counts bitfield reads.
// Variable + Type always live; sticky (no invent soft-skip bitfield read stats past hole).
func RecordBitfieldsReads(v *Variable) { RecordBitfieldsReadsSess(nil, v) }

// RecordBitfieldsReadsSess is RecordBitfieldsReads on an explicit session bag.
func RecordBitfieldsReadsSess(s *Session, v *Variable) {
	if v == nil || v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if v.Type.HasBitfields() {
		// residual ERROR sticky — no invent soft-count past HasBitfields hole
		if sessHasError(s) {
			return
		}
		sessBK(s).rhsBitfieldsStructsVarsCnt++
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent soft-skip bitfield count past HasBitfields residual false
		return
	}
	if v.IsBitfield {
		sessBK(s).rhsBitfieldCnt++
	}
}

// RecordBitfieldsWrites mirrors Bookkeeper::record_bitfields_writes.
// Bookkeeper.cpp:347–356.
// RecordBitfieldsWrites counts bitfield writes.
// Variable + Type always live; sticky (no invent soft-skip bitfield write stats past hole).
func RecordBitfieldsWrites(v *Variable) { RecordBitfieldsWritesSess(nil, v) }

// RecordBitfieldsWritesSess is RecordBitfieldsWrites on an explicit session bag.
func RecordBitfieldsWritesSess(s *Session, v *Variable) {
	if v == nil || v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if v.Type.HasBitfields() {
		// residual ERROR sticky — no invent soft-count past HasBitfields hole
		if sessHasError(s) {
			return
		}
		sessBK(s).lhsBitfieldsStructsVarsCnt++
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent soft-skip bitfield count past HasBitfields residual false
		return
	}
	if v.IsBitfield {
		sessBK(s).lhsBitfieldCnt++
	}
}

// RecordPointerComparisons mirrors Bookkeeper::record_pointer_comparisons.
// Bookkeeper.cpp:361–382 — skip function terms; pointer types; null/ptr/addr counts.
// RecordPointerComparisons counts pointer comparison kinds.
// Expression operands always live; sticky (no invent soft-skip cmp stats past hole).
func RecordPointerComparisons(lhs, rhs *Expression) {
	RecordPointerComparisonsSess(nil, lhs, rhs)
}

// RecordPointerComparisonsSess is RecordPointerComparisons on an explicit session bag.
func RecordPointerComparisonsSess(s *Session, lhs, rhs *Expression) {
	if lhs == nil || rhs == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	// Bookkeeper.cpp:363 — both non-function
	if lhs.Term == TermFunction || rhs.Term == TermFunction {
		return
	}
	lt, rt := lhs.GetType(), rhs.GetType()
	// residual ERROR sticky — no invent soft-skip cmp stats past GetType residual hole
	if sessHasError(s) {
		return
	}
	// Bookkeeper.cpp:364–365 — assert both pointer; fail closed non-sticky skip
	// (non-pointer cmp is soft non-event, not sticky factory poison)
	if lt == nil || rt == nil {
		return
	}
	lPtr := lt.IsPointerLike()
	// residual ERROR sticky — no invent soft-skip cmp stats past IsPointerLike residual
	if sessHasError(s) {
		return
	}
	rPtr := rt.IsPointerLike()
	// residual ERROR sticky — no invent soft-skip cmp stats past RHS IsPointerLike residual
	if sessHasError(s) {
		return
	}
	if !lPtr || !rPtr {
		return
	}
	// var vs constant → null compare
	if (lhs.Term == TermVariable && rhs.Term == TermConstant) ||
		(rhs.Term == TermVariable && lhs.Term == TermConstant) {
		sessBK(s).cmpPtrToNull++
		return
	}
	if lhs.Term == TermVariable && rhs.Term == TermVariable {
		// incomplete type IR sticky — no invent ptr-vs-ptr counts via invented level 0
		li, lok := lhs.IndirectLevelComplete()
		ri, rok := rhs.IndirectLevelComplete()
		if !lok || !rok {
			sessNoteError(s, ErrGeneric)
			return
		}
		if li == ri {
			sessBK(s).cmpPtrToPtr++
		} else {
			sessBK(s).cmpPtrToAddr++
		}
	}
}

// RecordVarsWithBitfields mirrors Bookkeeper::record_vars_with_bitfields.
// Bookkeeper.cpp:464–474 — assert(type); base aggregate with bitfields.
func RecordVarsWithBitfields(t *Type) { RecordVarsWithBitfieldsSess(nil, t) }

// RecordVarsWithBitfieldsSess is RecordVarsWithBitfields on an explicit session bag.
func RecordVarsWithBitfieldsSess(s *Session, t *Type) {
	// Bookkeeper.cpp:465 assert(type) sticky on nil; !has_bitfields is normal no-op
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !t.HasBitfields() {
		// residual ERROR sticky — no invent soft-skip count past HasBitfields residual false
		if sessHasError(s) {
			return
		}
		return
	}
	// residual ERROR sticky — no invent soft-count past HasBitfields residual true path
	if sessHasError(s) {
		return
	}
	level := t.IndirectLevel()
	// residual ERROR sticky — no invent soft-count past IndirectLevel residual hole
	if sessHasError(s) {
		return
	}
	bk := sessBK(s)
	IncrCounter(&bk.varsWithBitfields, level)
}

// RecordTypeWithBitfields mirrors Bookkeeper::record_type_with_bitfields.
// Bookkeeper.cpp:476–499 — only when has_bitfields; count bitfield members.
// RecordTypeWithBitfields counts bitfield members on aggregate types.
// Type always live; sticky (no invent soft-skip bitfield stats past hole).
// Non-aggregate is complete no-op.
func RecordTypeWithBitfields(t *Type) {
	RecordTypeWithBitfieldsSess(nil, t)
}

// RecordTypeWithBitfieldsSess is RecordTypeWithBitfields on an explicit session bag.
func RecordTypeWithBitfieldsSess(s *Session, t *Type) {
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	if !t.IsAggregate() {
		// residual ERROR sticky — no invent soft-skip count past IsAggregate residual false
		if sessHasError(s) {
			return
		}
		return
	}
	// residual ERROR sticky — no invent soft-continue count past IsAggregate residual true
	if sessHasError(s) {
		return
	}
	// Bookkeeper.cpp:480 — if (!typ->has_bitfields()) return (via outer if)
	if !t.HasBitfields() {
		// residual ERROR sticky — no invent soft-skip count past HasBitfields residual false
		if sessHasError(s) {
			return
		}
		return
	}
	// residual ERROR sticky — no invent soft-count past HasBitfields residual true path
	if sessHasError(s) {
		return
	}
	// Bookkeeper.cpp:482–483 — assert(len == fields.size()); Fields carry BitWidth
	// incomplete field IR (nil Type) sticky stop — no invent zero counts past gaps
	sessBK(s).structsWithBitfields++
	for _, f := range t.Fields {
		// StructField Type always live for bitfield stats; nil hole sticky stop
		if f.Type == nil {
			sessNoteError(s, ErrGeneric)
			return
		}
		// Bookkeeper.cpp:485 — if (!is_bitfield(i)) continue
		if f.BitWidth < 0 {
			continue
		}
		// Bookkeeper.cpp:488–489 — bitfields_in_total++; zero width → unamed
		sessBK(s).bitfieldsInTotal++
		if f.BitWidth == 0 {
			sessBK(s).unamedBitfieldsInTotal++
		}
		// Bookkeeper.cpp:491–495 — qfers_[i] const/volatile
		if f.Qfer.IsConst() {
			// residual ERROR sticky — no invent soft-count past IsConst residual hole
			if sessHasError(s) {
				return
			}
			sessBK(s).constBitfieldsInTotal++
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue stats past IsConst residual false
			return
		}
		if f.Qfer.IsVolatile() {
			// residual ERROR sticky — no invent soft-count past IsVolatile residual hole
			if sessHasError(s) {
				return
			}
			sessBK(s).volatileBitfieldsInTotal++
		} else if sessHasError(s) {
			// residual ERROR sticky — no invent soft-continue stats past IsVolatile residual false
			return
		}
	}
}

// RecordVarCreated mirrors use_new_var path in VariableSelector::SelectVariable.
// VariableSelector.cpp:1230–1236.
// RecordVarCreated mirrors use_new_var bookkeeping on create.
// Variable + Type always live; sticky (no invent soft-skip create stats past hole).
func RecordVarCreated(v *Variable) { RecordVarCreatedSess(nil, v) }

// RecordVarCreatedSess is RecordVarCreated on an explicit session bag.
func RecordVarCreatedSess(s *Session, v *Variable) {
	if v == nil || v.Type == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	sessBK(s).useNewVarCnt++
	RecordVarsWithBitfieldsSess(s, v.Type)
	// residual ERROR sticky — no invent soft-continue create stats past bitfields residual
	if sessHasError(s) {
		return
	}
	d := v.Type.StructDepth()
	// residual ERROR sticky — no invent soft-count depth past StructDepth residual
	if sessHasError(s) {
		return
	}
	bk := sessBK(s)
	IncrCounter(&bk.structDepthCnts, d)
	if v.Type.IsUnion() {
		// residual ERROR sticky — no invent soft-count union past IsUnion residual hole
		if sessHasError(s) {
			return
		}
		sessBK(s).unionVarCnt++
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent soft-continue past IsUnion residual false
		return
	}
}

// RecordVarReused mirrors use_old_var_cnt++.
// VariableSelector.cpp:1237–1238.
func RecordVarReused() { RecordVarReusedSess(nil) }

// RecordVarReusedSess records on an explicit session bag.
func RecordVarReusedSess(s *Session) {
	sessBK(s).useOldVarCnt++
}

// RecordForwardJump mirrors Bookkeeper::forward_jump_cnt++.
func RecordForwardJump() { RecordForwardJumpSess(nil) }

// RecordForwardJumpSess records on an explicit session bag.
func RecordForwardJumpSess(s *Session) { sessBK(s).forwardJumpCnt++ }

// RecordBackwardJump mirrors Bookkeeper::backward_jump_cnt++.
func RecordBackwardJump() { RecordBackwardJumpSess(nil) }

// RecordBackwardJumpSess records on an explicit session bag.
func RecordBackwardJumpSess(s *Session) { sessBK(s).backwardJumpCnt++ }

// RecordPointerAvailForDeref mirrors Bookkeeper::pointer_avail_for_dereference++.
// VariableSelector.cpp:416–419.
func RecordPointerAvailForDeref() { RecordPointerAvailForDerefSess(nil) }

// RecordPointerAvailForDerefSess records on an explicit session bag.
func RecordPointerAvailForDerefSess(s *Session) { sessBK(s).pointerAvailForDeref++ }

// RecordVolatileAvail mirrors Bookkeeper::volatile_avail++.
// VariableSelector.cpp:311 — has_eligible_volatile_var hit.
func RecordVolatileAvail() { RecordVolatileAvailSess(nil) }

// RecordVolatileAvailSess records on an explicit session bag.
func RecordVolatileAvailSess(s *Session) { sessBK(s).volatileAvail++ }

// VolatileAvailCount returns volatile_avail (tests / statistics).
func VolatileAvailCount() int { return sessBK(nil).volatileAvail }

// RecordOOB mirrors Bookkeeper::oob_cnt++ when array_oob_prob fires.
// StatementFor.cpp:157–158 — make_random_array_control.
func RecordOOB() { RecordOOBSess(nil) }

// RecordOOBSess records on an explicit session bag.
func RecordOOBSess(s *Session) { sessBK(s).oobCnt++ }

// OOBCount returns oob_cnt (tests / statistics).
func OOBCount() int { return sessBK(nil).oobCnt }

// ExpressionComplexity mirrors Expression::get_complexity.
// ExpressionVariable/Constant: 0; ExpressionFuncall.cpp:131–143 — user call +1
// plus sum of arg complexities; assign/comma nest.
// Incomplete IR fails closed sticky as -1 (no invent leaf depth 0 / soft re-pick
// stats past partial nest counts).
func ExpressionComplexity(e *Expression) int {
	return ExpressionComplexitySess(nil, e)
}

func ExpressionComplexitySess(s *Session, e *Expression) int {
	if e == nil {
		sessNoteError(s, ErrGeneric)
		return -1
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Type* + Value; incomplete shell sticky → -1
		// (no invent leaf complexity 0 for Type-nil / empty-value shell)
		if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
			sessNoteError(s, ErrGeneric)
			return -1
		}
		return 0
	case TermVariable:
		// ExpressionVariable always has live Variable*; incomplete sticky → -1
		// Type-nil non-special sticky (no invent leaf complexity 0 past type hole)
		if e.Var == nil {
			sessNoteError(s, ErrGeneric)
			return -1
		}
		if e.Var.Type == nil && !IsSpecialPtr(e.Var) {
			sessNoteError(s, ErrGeneric)
			return -1
		}
		return 0
	case TermFunction:
		// ExpressionFuncall::get_complexity — live invoke only
		// no soft invent complexity 0/1 for nil Invoke IR sticky
		if e.Invoke == nil {
			sessNoteError(s, ErrGeneric)
			return -1
		}
		comp := 0
		if e.Invoke.IsStd {
			// std unary/binary: no +1 for call itself
		} else {
			// user-defined path: Function* always live; incomplete sticky
			// (no invent complexity 0 shell past missing User as non-call)
			if e.Invoke.User == nil {
				sessNoteError(s, ErrGeneric)
				return -1
			}
			comp++ // function call itself
		}
		for _, a := range e.Invoke.Args {
			// param_value[i] always live after ERROR_GUARD; nil hole → fail closed sticky
			if a == nil {
				sessNoteError(s, ErrGeneric)
				return -1
			}
			c := ExpressionComplexitySess(s, a)
			if c < 0 {
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return -1
			}
			comp += c
		}
		return comp
	case TermAssignment:
		// incomplete Assign IR sticky — no invent complexity 1 shell
		if e.Assign == nil || e.Assign.Expr == nil {
			sessNoteError(s, ErrGeneric)
			return -1
		}
		c := ExpressionComplexitySess(s, e.Assign.Expr)
		if c < 0 {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return -1
		}
		return 1 + c
	case TermCommaExpr:
		// both sides always live; incomplete sticky → -1
		if e.CommaLHS == nil || e.CommaRHS == nil {
			sessNoteError(s, ErrGeneric)
			return -1
		}
		cl := ExpressionComplexitySess(s, e.CommaLHS)
		cr := ExpressionComplexitySess(s, e.CommaRHS)
		if cl < 0 || cr < 0 {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return -1
		}
		return 1 + cl + cr
	default:
		// unknown term kind — incomplete IR sticky
		sessNoteError(s, ErrGeneric)
		return -1
	}
}

// collectStmtExprs appends expressions like Statement::get_exprs + get_blocks.
// Bookkeeper.cpp:209–221 / Statement.cpp get_exprs virtuals.
// Returns false on incomplete IR sticky (no invent partial expr list / soft re-pick past holes).}

func collectStmtExprs(st *Stmt, out *[]*Expression) bool {
	if st == nil || out == nil {
		if out != nil {
			sessNoteError(nil, ErrGeneric)
		}
		return false
	}
	// StatementFor::get_exprs — push test only (init/incr are separate
	// StatementAssigns not walked via get_exprs). Kind-gated (no invent
	// Loop-on-wrong-kind). Incomplete Loop IR fails closed sticky.
	// StatementArrayOp.h:65–68 — if (init_value) only; NOT For test.
	// Fair: ArrayOp make_random_array_init uses numeric LoopControl (no TestExpr).
	switch st.Kind {
	case StmtFor:
		if st.Loop == nil || st.Loop.TestExpr == nil {
			sessNoteError(nil, ErrGeneric)
			return false
		}
		*out = append(*out, st.Loop.TestExpr)
	case StmtArrayOp:
		// optional init_value (body path has none; Go may nest RHS under Then)
		if st.Expr != nil {
			*out = append(*out, st.Expr)
		}
	case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
		// C++ get_exprs always yields live Expression* for these kinds
		// incomplete nil Expr fails closed sticky (no invent empty get_exprs success)
		if st.Expr == nil {
			sessNoteError(nil, ErrGeneric)
			return false
		}
		*out = append(*out, st.Expr)
	default:
		if st.Expr != nil {
			*out = append(*out, st.Expr)
		}
	}
	// get_blocks → recurse (Block* always live; nil hole fails closed sticky)
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			sessNoteError(nil, ErrGeneric)
			return false
		}
		for i := range b.Stmts {
			if !collectStmtExprs(&b.Stmts[i], out) {
				return false
			}
		}
	}
	return true
}

// StatExprDepths mirrors Bookkeeper::stat_expr_depths over non-builtin funcs.
// Bookkeeper.cpp:224–230.
// Function* always live; incomplete Funcs list sticky clears counts.
// Builtins without body skip. Incomplete expressions / stmt IR sticky clear counts —
// no invent counting broken IR as leaf depth 0 / soft re-pick past holes.
func StatExprDepths(funcs []*Function) {
	StatExprDepthsSess(nil, funcs)
}

// StatExprDepthsSess is StatExprDepths writing expr depth counters on bag s.
func StatExprDepthsSess(s *Session, funcs []*Function) {
	sessBK(s).exprDepthCnts = nil
	if !FunctionsComplete(funcs) {
		sessNoteError(s, ErrGeneric)
		return
	}
	for _, f := range funcs {
		if f.IsBuiltin {
			continue
		}
		// user func body always live after build; nil sticky clear
		if f.Body == nil {
			sessBK(s).exprDepthCnts = nil
			sessNoteError(s, ErrGeneric)
			return
		}
		for i := range f.Body.Stmts {
			var exprs []*Expression
			if !collectStmtExprs(&f.Body.Stmts[i], &exprs) {
				// collectStmtExprs may already sticky
				sessBK(s).exprDepthCnts = nil
				if !sessHasError(s) {
					sessNoteError(s, ErrGeneric)
				}
				return
			}
			for _, e := range exprs {
				c := ExpressionComplexitySess(s, e)
				if c < 0 {
					// ExpressionComplexity may already sticky
					sessBK(s).exprDepthCnts = nil
					if !sessHasError(s) {
						sessNoteError(s, ErrGeneric)
					}
					return
				}
				bk := sessBK(s)
				IncrCounter(&bk.exprDepthCnts, c)
			}
		}
	}
}

// StatBlkDepths mirrors Bookkeeper::stat_blk_depths.
// Bookkeeper.cpp:128–152 — non-block stmts counted at get_blk_depth()-1.
// Incomplete Funcs / Block* holes fail closed sticky zero counts (no invent partial depths).
func StatBlkDepths(funcs []*Function) int {
	return StatBlkDepthsSess(nil, funcs)
}

// StatBlkDepthsSess is StatBlkDepths writing block depth counters on bag s.
func StatBlkDepthsSess(s *Session, funcs []*Function) int {
	sessBK(s).blkDepthCnts = nil
	cnt := 0
	if !FunctionsComplete(funcs) {
		sessNoteError(s, ErrGeneric)
		return 0
	}
	incomplete := false
	// Bookkeeper.cpp:128–140 — stat_blk_depths_for_stmt
	var walk func(st *Stmt, parent *Block)
	walk = func(st *Stmt, parent *Block) {
		if st == nil || incomplete {
			return
		}
		// eType != eBlock: our Stmt kinds are never pure Block statements
		// incr_counter(blk_depth_cnts, s->get_blk_depth() - 1)
		depth := GetBlkDepth(parent)
		if depth > 0 {
			depth--
		}
		bk := sessBK(s)
		IncrCounter(&bk.blkDepthCnts, depth)
		cnt++
		// get_blocks → recurse into Then/Else stmts with that block as parent
		// Block* always live; nil hole sticky clear counts
		for _, blk := range GetBlocksStmt(st) {
			if blk == nil {
				incomplete = true
				sessBK(s).blkDepthCnts = nil
				cnt = 0
				sessNoteError(s, ErrGeneric)
				return
			}
			for i := range blk.Stmts {
				walk(&blk.Stmts[i], blk)
			}
		}
	}
	for _, f := range funcs {
		// pre-validated FunctionsComplete
		if f.IsBuiltin {
			continue
		}
		// user func body always live after build; nil sticky
		if f.Body == nil {
			sessBK(s).blkDepthCnts = nil
			sessNoteError(s, ErrGeneric)
			return 0
		}
		// body is a Block; count its statements with parent=body
		for i := range f.Body.Stmts {
			walk(&f.Body.Stmts[i], f.Body)
			if incomplete {
				return 0
			}
		}
	}
	return cnt
}

// OutputStatistics mirrors Bookkeeper::output_statistics.
// Bookkeeper.cpp:167–192.
func OutputStatistics(funcs []*Function, opts Options) string {
	return OutputStatisticsSess(nil, funcs, opts)
}

// OutputStatisticsSess is OutputStatistics reading counters on bag s.
func OutputStatisticsSess(s *Session, funcs []*Function, opts Options) string {
	var b strings.Builder
	outputStructUnionStatisticsSess(s, &b, opts)
	// residual ERROR sticky — no invent soft-continue later stats past residual hole
	if sessHasError(s) {
		return ""
	}
	b.WriteString("\n")
	outputExprStatisticsSess(s, &b, funcs)
	// residual ERROR sticky — no invent soft-continue later stats past StatExpr residual
	if sessHasError(s) {
		return ""
	}
	b.WriteString("\n")
	outputPointerStatistics(&b, s)
	// residual ERROR sticky — no invent soft-continue later stats past pointer residual
	if sessHasError(s) {
		return ""
	}
	b.WriteString("\n")
	outputVolatileAccessStatisticsSess(s, &b)
	if sessHasError(s) {
		return ""
	}
	b.WriteString("\n")
	outputJumpStatisticsSess(s, &b)
	if sessHasError(s) {
		return ""
	}
	b.WriteString("\n")
	outputStmtsStatisticsSess(s, &b, funcs)
	// residual ERROR sticky — no invent soft-continue later stats past StatBlk residual
	if sessHasError(s) {
		return ""
	}
	b.WriteString("\n")
	outputVarFreshnessSess(s, &b)
	if sessHasError(s) {
		return ""
	}
	if sessBK(s).relyOnIntSize {
		b.WriteString("FYI: the random generator makes assumptions about the integer size. See platform.info for more details.\n")
	}
	if sessBK(s).relyOnPtrSize {
		b.WriteString("FYI: the random generator makes assumptions about the pointer size. See platform.info for more details.\n")
	}
	formattedOutput(&b, "total OOB instances added: ", sessBK(s).oobCnt)
	return b.String()
}

func outputStructUnionStatistics(b *strings.Builder, opts Options) {
	outputStructUnionStatisticsSess(nil, b, opts)
}

func outputStructUnionStatisticsSess(s *Session, b *strings.Builder, opts Options) {
	maxD := len(sessBK(s).structDepthCnts) - 1
	if maxD < 0 {
		maxD = 0
	}
	// empty vector → size()-1 wraps in C++; we emit 0
	if len(sessBK(s).structDepthCnts) == 0 {
		formattedOutput(b, "max struct depth: ", -1) // size_t(-1) style when empty? C++ size 0 → (0-1)=max
		// upstream: (struct_depth_cnts.size() - 1) as int when empty is -1
	} else {
		formattedOutput(b, "max struct depth: ", maxD)
	}
	b.WriteString("breakdown:\n")
	for i, c := range sessBK(s).structDepthCnts {
		b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, c))
	}
	formattedOutput(b, "total union variables: ", sessBK(s).unionVarCnt)
	outputBitfieldsSess(s, b, opts)
}

func outputBitfields(b *strings.Builder, opts Options) {
	outputBitfieldsSess(nil, b, opts)
}

func outputBitfieldsSess(s *Session, b *strings.Builder, opts Options) {
	if !opts.Bitfields {
		return
	}
	b.WriteString("\n")
	formattedOutput(b, "non-zero bitfields defined in structs: ", sessBK(s).bitfieldsInTotal)
	formattedOutput(b, "zero bitfields defined in structs: ", sessBK(s).unamedBitfieldsInTotal)
	formattedOutput(b, "const bitfields defined in structs: ", sessBK(s).constBitfieldsInTotal)
	formattedOutput(b, "volatile bitfields defined in structs: ", sessBK(s).volatileBitfieldsInTotal)
	formattedOutput(b, "structs with bitfields in the program: ", CalcTotal(sessBK(s).varsWithBitfields))
	b.WriteString("breakdown:\n")
	for i, c := range sessBK(s).varsWithBitfields {
		b.WriteString(fmt.Sprintf("   indirect level: %d, occurrence: %d\n", i, c))
	}
	formattedOutput(b, "times a bitfields struct's address is taken: ", sessBK(s).varsWithBitfieldsAddressTakenCnt)
	formattedOutput(b, "times a bitfields struct is read: ", sessBK(s).rhsBitfieldsStructsVarsCnt)
	formattedOutput(b, "times a bitfields struct is write: ", sessBK(s).lhsBitfieldsStructsVarsCnt)
	formattedOutput(b, "times a bitfield is read: ", sessBK(s).rhsBitfieldCnt)
	formattedOutput(b, "times a bitfield is write: ", sessBK(s).lhsBitfieldCnt)
}

func outputExprStatistics(b *strings.Builder, funcs []*Function) {
	outputExprStatisticsSess(nil, b, funcs)
}

func outputExprStatisticsSess(s *Session, b *strings.Builder, funcs []*Function) {
	StatExprDepthsSess(s, funcs)
	maxD := len(sessBK(s).exprDepthCnts) - 1
	if len(sessBK(s).exprDepthCnts) == 0 {
		formattedOutput(b, "max expression depth: ", -1)
	} else {
		formattedOutput(b, "max expression depth: ", maxD)
	}
	b.WriteString("breakdown:\n")
	for i, c := range sessBK(s).exprDepthCnts {
		if c != 0 {
			b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, c))
		}
	}
}

func outputPointerStatistics(b *strings.Builder, s *Session) {
	// Bookkeeper.cpp:245–318 — all_ptrs / all_aliases when present
	sess := sessOrAmbient(s)
	bk := sessBK(s)
	ptrs := sess.AllPtrs
	formattedOutput(b, "total number of pointers: ", len(ptrs))
	if len(ptrs) > 0 {
		b.WriteString("\n")
	}
	formattedOutput(b, "times a variable address is taken: ", bk.addressTakenCnt)
	formattedOutput(b, "times a pointer is dereferenced on RHS: ", CalcTotal(bk.readDereferenceCnts))
	b.WriteString("breakdown:\n")
	for i := 1; i < len(bk.readDereferenceCnts); i++ {
		b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, bk.readDereferenceCnts[i]))
	}
	formattedOutput(b, "times a pointer is dereferenced on LHS: ", CalcTotal(bk.writeDereferenceCnts))
	b.WriteString("breakdown:\n")
	for i := 1; i < len(bk.writeDereferenceCnts); i++ {
		b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, bk.writeDereferenceCnts[i]))
	}
	formattedOutput(b, "times a pointer is compared with null: ", bk.cmpPtrToNull)
	formattedOutput(b, "times a pointer is compared with address of another variable: ", bk.cmpPtrToAddr)
	formattedOutput(b, "times a pointer is compared with another pointer: ", bk.cmpPtrToPtr)
	formattedOutput(b, "times a pointer is qualified to be dereferenced: ", bk.pointerAvailForDeref)
	if len(bk.dereferenceLevelCnts) > 0 {
		b.WriteString("\n")
		formattedOutput(b, "max dereference level: ", len(bk.dereferenceLevelCnts)-1)
		b.WriteString("breakdown:\n")
		for i, c := range bk.dereferenceLevelCnts {
			b.WriteString(fmt.Sprintf("   level: %d, occurrence: %d\n", i, c))
		}
	}
	if len(ptrs) > 0 {
		totalAlias := 0
		hasNull := 0
		ptPtr, ptScalar, ptStruct := 0, 0, 0
		for i, p := range ptrs {
			// Variable* always live in ptrs bookkeeping; nil hole sticky fail closed
			// (no invent skip partial alias/pointee stats)
			if p == nil || p.Type == nil {
				totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
				sessNoteError(s, ErrGeneric)
				break
			}
			// Bookkeeper.cpp:260 — assert(t->eType == ePointer); skip non-pointer aggregates
			ptrLike := p.Type.IsPointerLike()
			// residual ERROR sticky — no invent soft-skip stats past IsPointerLike residual
			if sessHasError(s) {
				totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
				break
			}
			if !ptrLike {
				continue
			}
			aliases := sess.AllAliases
			if i < len(aliases) {
				totalAlias += len(aliases[i])
				if IsVariableInSet(aliases[i], NullPtr) {
					hasNull++
				}
			}
			if p.Type.IndirectLevel() > 1 {
				// residual ERROR sticky — no invent soft-count past IndirectLevel residual hole
				if sessHasError(s) {
					totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
					break
				}
				ptPtr++
			} else if sessHasError(s) {
				// residual ERROR sticky — no invent soft-continue stats past IndirectLevel residual false
				totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
				break
			} else if pt := p.Type.PtrType(); pt != nil {
				// residual ERROR sticky — no invent soft-count past PtrType residual hole
				if sessHasError(s) {
					totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
					break
				}
				if pt.IsSimple() {
					// residual ERROR sticky — no invent soft-count past IsSimple residual hole
					if sessHasError(s) {
						totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
						break
					}
					ptScalar++
				} else if sessHasError(s) {
					// residual ERROR sticky — no invent soft-continue past IsSimple residual false
					totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
					break
				} else if pt.IsStruct() {
					// residual ERROR sticky — no invent soft-count past IsStruct residual hole
					if sessHasError(s) {
						totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
						break
					}
					ptStruct++
				} else if sessHasError(s) {
					// residual ERROR sticky — no invent soft-continue past IsStruct residual false
					totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
					break
				}
			} else if sessHasError(s) {
				// residual ERROR sticky — no invent soft-continue past PtrType residual nil
				totalAlias, hasNull, ptPtr, ptScalar, ptStruct = 0, 0, 0, 0, 0
				break
			}
		}
		formattedOutput(b, "number of pointers point to pointers: ", ptPtr)
		formattedOutput(b, "number of pointers point to scalars: ", ptScalar)
		formattedOutput(b, "number of pointers point to structs: ", ptStruct)
		formattedOutputf(b, "percent of pointers has null in alias set: ", float64(hasNull)*100.0/float64(len(ptrs)))
		formattedOutputf(b, "average alias set size: ", float64(totalAlias)/float64(len(ptrs)))
	}
}

func outputVolatileAccessStatistics(b *strings.Builder) {
	outputVolatileAccessStatisticsSess(nil, b)
}

func outputVolatileAccessStatisticsSess(s *Session, b *strings.Builder) {
	formattedOutput(b, "times a non-volatile is read: ", sessBK(s).readNonVolatileCnt)
	formattedOutput(b, "times a non-volatile is write: ", sessBK(s).writeNonVolatileCnt)
	formattedOutput(b, "times a volatile is read: ", sessBK(s).readVolatileCnt)
	formattedOutput(b, "   times read thru a pointer: ", sessBK(s).readVolatileThruPtrCnt)
	formattedOutput(b, "times a volatile is write: ", sessBK(s).writeVolatileCnt)
	formattedOutput(b, "   times written thru a pointer: ", sessBK(s).writeVolatileThruPtrCnt)
	total := sessBK(s).readNonVolatileCnt + sessBK(s).writeNonVolatileCnt + sessBK(s).readVolatileCnt + sessBK(s).writeVolatileCnt
	percentage := 0.0
	if total > 0 {
		percentage = float64(sessBK(s).readNonVolatileCnt+sessBK(s).writeNonVolatileCnt) * 100.0 / float64(total)
	}
	formattedOutputf(b, "times a volatile is available for access: ", float64(sessBK(s).volatileAvail))
	formattedOutputf(b, "percentage of non-volatile access: ", percentage)
}

func outputJumpStatistics(b *strings.Builder) {
	outputJumpStatisticsSess(nil, b)
}

func outputJumpStatisticsSess(s *Session, b *strings.Builder) {
	formattedOutput(b, "forward jumps: ", sessBK(s).forwardJumpCnt)
	formattedOutput(b, "backward jumps: ", sessBK(s).backwardJumpCnt)
}

func outputStmtsStatistics(b *strings.Builder, funcs []*Function) {
	outputStmtsStatisticsSess(nil, b, funcs)
}

func outputStmtsStatisticsSess(s *Session, b *strings.Builder, funcs []*Function) {
	stmtCnt := StatBlkDepthsSess(s, funcs)
	formattedOutput(b, "stmts: ", stmtCnt)
	maxD := len(sessBK(s).blkDepthCnts) - 1
	if len(sessBK(s).blkDepthCnts) == 0 {
		formattedOutput(b, "max block depth: ", -1)
	} else {
		formattedOutput(b, "max block depth: ", maxD)
	}
	b.WriteString("breakdown:\n")
	for i, c := range sessBK(s).blkDepthCnts {
		if c != 0 {
			b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, c))
		}
	}
}

func outputVarFreshness(b *strings.Builder) {
	outputVarFreshnessSess(nil, b)
}

func outputVarFreshnessSess(s *Session, b *strings.Builder) {
	total := sessBK(s).useNewVarCnt + sessBK(s).useOldVarCnt
	fresh, exist := 0.0, 0.0
	if total > 0 {
		fresh = float64(sessBK(s).useNewVarCnt) * 100.0 / float64(total)
		exist = float64(sessBK(s).useOldVarCnt) * 100.0 / float64(total)
	}
	formattedOutputf(b, "percentage a fresh-made variable is used: ", fresh)
	formattedOutputf(b, "percentage an existing variable is used: ", exist)
}

// OutputTail mirrors OutputMgr::OutputTail — statistics comment after main.
// OutputMgr.cpp:223–233.
func OutputTail(funcs []*Function, opts Options) string {
	return OutputTailSess(nil, funcs, opts)
}

// OutputTailSess is OutputTail reading statistics counters on bag s.
func OutputTailSess(s *Session, funcs []*Function, opts Options) string {
	if opts.Concise {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n/************************ statistics *************************\n")
	stats := OutputStatisticsSess(s, funcs, opts)
	// residual ERROR sticky — no invent soft-continue stats shell past residual hole
	if sessHasError(s) {
		return ""
	}
	b.WriteString(stats)
	b.WriteString("********************* end of statistics **********************/\n")
	return b.String()
}
