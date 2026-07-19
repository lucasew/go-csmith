// Upstream: Bookkeeper.h / Bookkeeper.cpp (generation statistics counters + tail dump).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// Bookkeeper package counters mirror Bookkeeper static fields.
// Bookkeeper.cpp:57–94.
var (
	structDepthCnts []int
	unionVarCnt     int

	exprDepthCnts []int
	blkDepthCnts  []int

	dereferenceLevelCnts []int
	addressTakenCnt      int
	writeDereferenceCnts []int
	readDereferenceCnts  []int

	cmpPtrToNull int
	cmpPtrToPtr  int
	cmpPtrToAddr int

	readVolatileCnt         int
	writeVolatileCnt        int
	readNonVolatileCnt      int
	writeNonVolatileCnt     int
	readVolatileThruPtrCnt  int
	writeVolatileThruPtrCnt int
	pointerAvailForDeref    int
	volatileAvail           int

	structsWithBitfields            int
	varsWithBitfields               []int
	varsWithFullBitfields           []int
	varsWithBitfieldsAddressTakenCnt int
	bitfieldsInTotal                int
	unamedBitfieldsInTotal          int
	constBitfieldsInTotal           int
	volatileBitfieldsInTotal        int
	lhsBitfieldsStructsVarsCnt      int
	rhsBitfieldsStructsVarsCnt      int
	lhsBitfieldCnt                  int
	rhsBitfieldCnt                  int

	forwardJumpCnt  int
	backwardJumpCnt int
	useNewVarCnt    int
	useOldVarCnt    int
	oobCnt          int

	relyOnIntSize bool
	relyOnPtrSize bool
)

// IncrCounter mirrors incr_counter — grow vector and ++ at pos.
// Bookkeeper.cpp:527–537.
func IncrCounter(counters *[]int, pos int) {
	if counters == nil || pos < 0 {
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
func BookkeeperDoFinalization() {
	structDepthCnts = nil
	unionVarCnt = 0
	exprDepthCnts = nil
	blkDepthCnts = nil
	dereferenceLevelCnts = nil
	addressTakenCnt = 0
	writeDereferenceCnts = nil
	readDereferenceCnts = nil
	cmpPtrToNull = 0
	cmpPtrToPtr = 0
	cmpPtrToAddr = 0
	readVolatileCnt = 0
	writeVolatileCnt = 0
	readNonVolatileCnt = 0
	writeNonVolatileCnt = 0
	readVolatileThruPtrCnt = 0
	writeVolatileThruPtrCnt = 0
	pointerAvailForDeref = 0
	volatileAvail = 0
	structsWithBitfields = 0
	varsWithBitfields = nil
	varsWithFullBitfields = nil
	varsWithBitfieldsAddressTakenCnt = 0
	bitfieldsInTotal = 0
	unamedBitfieldsInTotal = 0
	constBitfieldsInTotal = 0
	volatileBitfieldsInTotal = 0
	lhsBitfieldsStructsVarsCnt = 0
	rhsBitfieldsStructsVarsCnt = 0
	lhsBitfieldCnt = 0
	rhsBitfieldCnt = 0
	forwardJumpCnt = 0
	backwardJumpCnt = 0
	useNewVarCnt = 0
	useOldVarCnt = 0
	oobCnt = 0
	relyOnIntSize = false
	relyOnPtrSize = false
}

func formattedOutput(b *strings.Builder, msg string, num int) {
	// Bookkeeper.cpp always has live message string; sticky no invent "XXX %d" shell
	if b == nil {
		return
	}
	if msg == "" {
		SetError(ErrGeneric)
		return
	}
	b.WriteString("XXX ")
	b.WriteString(msg)
	b.WriteString(fmt.Sprintf("%d\n", num))
}

func formattedOutputf(b *strings.Builder, msg string, num float64) {
	if b == nil {
		return
	}
	if msg == "" {
		SetError(ErrGeneric)
		return
	}
	b.WriteString("XXX ")
	b.WriteString(msg)
	// Bookkeeper uses default precision; ~3 digits like upstream out.precision(3)
	b.WriteString(fmt.Sprintf("%g\n", num))
}

// RecordAddressTaken mirrors Bookkeeper::record_address_taken.
// Bookkeeper.cpp:324–334.
func RecordAddressTaken(v *Variable) {
	// Bookkeeper.cpp:325–326 — assert(var); assert(var->type) sticky
	// (no invent skip broken IR as zero address-taken stats / soft re-pick)
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return
	}
	v.IsAddrTaken = true
	addressTakenCnt++
	if v.Type.HasBitfields() {
		varsWithBitfieldsAddressTakenCnt++
	}
}

// RecordVolatileAccess mirrors Bookkeeper::record_volatile_access.
// Bookkeeper.cpp:386–412.
func RecordVolatileAccess(v *Variable, derefLevel int, write bool) {
	// Bookkeeper.cpp:388 — assert(var) sticky
	// (no invent skip nil as zero vol-access stats)
	if v == nil {
		SetError(ErrGeneric)
		return
	}
	if write {
		RecordBitfieldsWrites(v)
	} else {
		RecordBitfieldsReads(v)
	}
	for i := 0; i <= derefLevel; i++ {
		vol := v.IsVolatileAfterDeref(i)
		if write {
			if vol {
				if i > 0 {
					writeVolatileThruPtrCnt++
				}
				writeVolatileCnt++
			} else {
				writeNonVolatileCnt++
			}
		} else {
			if vol {
				if i > 0 {
					readVolatileThruPtrCnt++
				}
				readVolatileCnt++
			} else {
				readNonVolatileCnt++
			}
		}
	}
}

// RecordBitfieldsReads mirrors Bookkeeper::record_bitfields_reads.
// Bookkeeper.cpp:336–345.
// RecordBitfieldsReads counts bitfield reads.
// Variable + Type always live; sticky (no invent soft-skip bitfield read stats past hole).
func RecordBitfieldsReads(v *Variable) {
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return
	}
	if v.Type.HasBitfields() {
		rhsBitfieldsStructsVarsCnt++
	}
	if v.IsBitfield {
		rhsBitfieldCnt++
	}
}

// RecordBitfieldsWrites mirrors Bookkeeper::record_bitfields_writes.
// Bookkeeper.cpp:347–356.
// RecordBitfieldsWrites counts bitfield writes.
// Variable + Type always live; sticky (no invent soft-skip bitfield write stats past hole).
func RecordBitfieldsWrites(v *Variable) {
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return
	}
	if v.Type.HasBitfields() {
		lhsBitfieldsStructsVarsCnt++
	}
	if v.IsBitfield {
		lhsBitfieldCnt++
	}
}

// RecordPointerComparisons mirrors Bookkeeper::record_pointer_comparisons.
// Bookkeeper.cpp:361–382 — skip function terms; pointer types; null/ptr/addr counts.
// RecordPointerComparisons counts pointer comparison kinds.
// Expression operands always live; sticky (no invent soft-skip cmp stats past hole).
func RecordPointerComparisons(lhs, rhs *Expression) {
	if lhs == nil || rhs == nil {
		SetError(ErrGeneric)
		return
	}
	// Bookkeeper.cpp:363 — both non-function
	if lhs.Term == TermFunction || rhs.Term == TermFunction {
		return
	}
	lt, rt := lhs.GetType(), rhs.GetType()
	// Bookkeeper.cpp:364–365 — assert both pointer; fail closed non-sticky skip
	// (non-pointer cmp is soft non-event, not sticky factory poison)
	if lt == nil || rt == nil || !lt.IsPointerLike() || !rt.IsPointerLike() {
		return
	}
	// var vs constant → null compare
	if (lhs.Term == TermVariable && rhs.Term == TermConstant) ||
		(rhs.Term == TermVariable && lhs.Term == TermConstant) {
		cmpPtrToNull++
		return
	}
	if lhs.Term == TermVariable && rhs.Term == TermVariable {
		// incomplete type IR sticky — no invent ptr-vs-ptr counts via invented level 0
		li, lok := lhs.IndirectLevelComplete()
		ri, rok := rhs.IndirectLevelComplete()
		if !lok || !rok {
			SetError(ErrGeneric)
			return
		}
		if li == ri {
			cmpPtrToPtr++
		} else {
			cmpPtrToAddr++
		}
	}
}

// RecordVarsWithBitfields mirrors Bookkeeper::record_vars_with_bitfields.
// Bookkeeper.cpp:464–474 — assert(type); base aggregate with bitfields.
func RecordVarsWithBitfields(t *Type) {
	// Bookkeeper.cpp:465 assert(type) sticky on nil; !has_bitfields is normal no-op
	if t == nil {
		SetError(ErrGeneric)
		return
	}
	if !t.HasBitfields() {
		return
	}
	level := t.IndirectLevel()
	IncrCounter(&varsWithBitfields, level)
}

// RecordTypeWithBitfields mirrors Bookkeeper::record_type_with_bitfields.
// Bookkeeper.cpp:476–499 — only when has_bitfields; count bitfield members.
// RecordTypeWithBitfields counts bitfield members on aggregate types.
// Type always live; sticky (no invent soft-skip bitfield stats past hole).
// Non-aggregate is complete no-op.
func RecordTypeWithBitfields(t *Type) {
	if t == nil {
		SetError(ErrGeneric)
		return
	}
	if !t.IsAggregate() {
		return
	}
	// Bookkeeper.cpp:480 — if (!typ->has_bitfields()) return (via outer if)
	if !t.HasBitfields() {
		return
	}
	// Bookkeeper.cpp:482–483 — assert(len == fields.size()); Fields carry BitWidth
	// incomplete field IR (nil Type) sticky stop — no invent zero counts past gaps
	structsWithBitfields++
	for _, f := range t.Fields {
		// StructField Type always live for bitfield stats; nil hole sticky stop
		if f.Type == nil {
			SetError(ErrGeneric)
			return
		}
		// Bookkeeper.cpp:485 — if (!is_bitfield(i)) continue
		if f.BitWidth < 0 {
			continue
		}
		// Bookkeeper.cpp:488–489 — bitfields_in_total++; zero width → unamed
		bitfieldsInTotal++
		if f.BitWidth == 0 {
			unamedBitfieldsInTotal++
		}
		// Bookkeeper.cpp:491–495 — qfers_[i] const/volatile
		if f.Qfer.IsConst() {
			constBitfieldsInTotal++
		}
		if f.Qfer.IsVolatile() {
			volatileBitfieldsInTotal++
		}
	}
}

// RecordVarCreated mirrors use_new_var path in VariableSelector::SelectVariable.
// VariableSelector.cpp:1230–1236.
// RecordVarCreated mirrors use_new_var bookkeeping on create.
// Variable + Type always live; sticky (no invent soft-skip create stats past hole).
func RecordVarCreated(v *Variable) {
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return
	}
	useNewVarCnt++
	RecordVarsWithBitfields(v.Type)
	IncrCounter(&structDepthCnts, v.Type.StructDepth())
	if v.Type.IsUnion() {
		unionVarCnt++
	}
}

// RecordVarReused mirrors use_old_var_cnt++.
// VariableSelector.cpp:1237–1238.
func RecordVarReused() {
	useOldVarCnt++
}

// RecordForwardJump mirrors Bookkeeper::forward_jump_cnt++.
func RecordForwardJump() { forwardJumpCnt++ }

// RecordBackwardJump mirrors Bookkeeper::backward_jump_cnt++.
func RecordBackwardJump() { backwardJumpCnt++ }

// RecordPointerAvailForDeref mirrors Bookkeeper::pointer_avail_for_dereference++.
// VariableSelector.cpp:416–419.
func RecordPointerAvailForDeref() { pointerAvailForDeref++ }

// RecordVolatileAvail mirrors Bookkeeper::volatile_avail++.
// VariableSelector.cpp:311 — has_eligible_volatile_var hit.
func RecordVolatileAvail() { volatileAvail++ }

// VolatileAvailCount returns volatile_avail (tests / statistics).
func VolatileAvailCount() int { return volatileAvail }

// RecordOOB mirrors Bookkeeper::oob_cnt++ when array_oob_prob fires.
// StatementFor.cpp:157–158 — make_random_array_control.
func RecordOOB() { oobCnt++ }

// OOBCount returns oob_cnt (tests / statistics).
func OOBCount() int { return oobCnt }

// ExpressionComplexity mirrors Expression::get_complexity.
// ExpressionVariable/Constant: 0; ExpressionFuncall.cpp:131–143 — user call +1
// plus sum of arg complexities; assign/comma nest.
// Incomplete IR fails closed sticky as -1 (no invent leaf depth 0 / soft re-pick
// stats past partial nest counts).
func ExpressionComplexity(e *Expression) int {
	if e == nil {
		SetError(ErrGeneric)
		return -1
	}
	switch e.Term {
	case TermConstant:
		// Constant always has live Value; incomplete shell sticky → -1
		if e.Con == nil || e.Con.Value == "" {
			SetError(ErrGeneric)
			return -1
		}
		return 0
	case TermVariable:
		// ExpressionVariable always has live Variable*; incomplete sticky → -1
		if e.Var == nil {
			SetError(ErrGeneric)
			return -1
		}
		return 0
	case TermFunction:
		// ExpressionFuncall::get_complexity — live invoke only
		// no soft invent complexity 0/1 for nil Invoke IR sticky
		if e.Invoke == nil {
			SetError(ErrGeneric)
			return -1
		}
		comp := 0
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			comp++ // function call itself
		}
		for _, a := range e.Invoke.Args {
			// param_value[i] always live after ERROR_GUARD; nil hole → fail closed sticky
			if a == nil {
				SetError(ErrGeneric)
				return -1
			}
			c := ExpressionComplexity(a)
			if c < 0 {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return -1
			}
			comp += c
		}
		return comp
	case TermAssignment:
		// incomplete Assign IR sticky — no invent complexity 1 shell
		if e.Assign == nil || e.Assign.Expr == nil {
			SetError(ErrGeneric)
			return -1
		}
		c := ExpressionComplexity(e.Assign.Expr)
		if c < 0 {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return -1
		}
		return 1 + c
	case TermCommaExpr:
		// both sides always live; incomplete sticky → -1
		if e.CommaLHS == nil || e.CommaRHS == nil {
			SetError(ErrGeneric)
			return -1
		}
		cl := ExpressionComplexity(e.CommaLHS)
		cr := ExpressionComplexity(e.CommaRHS)
		if cl < 0 || cr < 0 {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return -1
		}
		return 1 + cl + cr
	default:
		// unknown term kind — incomplete IR sticky
		SetError(ErrGeneric)
		return -1
	}
}

// collectStmtExprs appends expressions like Statement::get_exprs + get_blocks.
// Bookkeeper.cpp:209–221 / Statement.cpp get_exprs virtuals.
// Returns false on incomplete IR sticky (no invent partial expr list / soft re-pick past holes).
func collectStmtExprs(st *Stmt, out *[]*Expression) bool {
	if st == nil || out == nil {
		if out != nil {
			SetError(ErrGeneric)
		}
		return false
	}
	// StatementFor/ArrayOp::get_exprs — push test only (init/incr are separate
	// StatementAssigns not walked via get_exprs). Kind-gated (no invent
	// Loop-on-wrong-kind). Incomplete Loop IR fails closed sticky.
	if st.Kind == StmtFor || st.Kind == StmtArrayOp {
		if st.Loop == nil || st.Loop.TestExpr == nil {
			SetError(ErrGeneric)
			return false
		}
		*out = append(*out, st.Loop.TestExpr)
	} else {
		switch st.Kind {
		case StmtAssign, StmtInvoke, StmtReturn, StmtIfElse, StmtBreak, StmtContinue, StmtGoto:
			// C++ get_exprs always yields live Expression* for these kinds
			// incomplete nil Expr fails closed sticky (no invent empty get_exprs success)
			if st.Expr == nil {
				SetError(ErrGeneric)
				return false
			}
			*out = append(*out, st.Expr)
		default:
			if st.Expr != nil {
				*out = append(*out, st.Expr)
			}
		}
	}
	// get_blocks → recurse (Block* always live; nil hole fails closed sticky)
	for _, b := range GetBlocksStmt(st) {
		if b == nil {
			SetError(ErrGeneric)
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
	exprDepthCnts = nil
	if !FunctionsComplete(funcs) {
		SetError(ErrGeneric)
		return
	}
	for _, f := range funcs {
		if f.IsBuiltin {
			continue
		}
		// user func body always live after build; nil sticky clear
		if f.Body == nil {
			exprDepthCnts = nil
			SetError(ErrGeneric)
			return
		}
		for i := range f.Body.Stmts {
			var exprs []*Expression
			if !collectStmtExprs(&f.Body.Stmts[i], &exprs) {
				// collectStmtExprs may already sticky
				exprDepthCnts = nil
				if !HasError() {
					SetError(ErrGeneric)
				}
				return
			}
			for _, e := range exprs {
				c := ExpressionComplexity(e)
				if c < 0 {
					// ExpressionComplexity may already sticky
					exprDepthCnts = nil
					if !HasError() {
						SetError(ErrGeneric)
					}
					return
				}
				IncrCounter(&exprDepthCnts, c)
			}
		}
	}
}

// StatBlkDepths mirrors Bookkeeper::stat_blk_depths.
// Bookkeeper.cpp:128–152 — non-block stmts counted at get_blk_depth()-1.
// Incomplete Funcs / Block* holes fail closed sticky zero counts (no invent partial depths).
func StatBlkDepths(funcs []*Function) int {
	blkDepthCnts = nil
	cnt := 0
	if !FunctionsComplete(funcs) {
		SetError(ErrGeneric)
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
		IncrCounter(&blkDepthCnts, depth)
		cnt++
		// get_blocks → recurse into Then/Else stmts with that block as parent
		// Block* always live; nil hole sticky clear counts
		for _, blk := range GetBlocksStmt(st) {
			if blk == nil {
				incomplete = true
				blkDepthCnts = nil
				cnt = 0
				SetError(ErrGeneric)
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
			blkDepthCnts = nil
			SetError(ErrGeneric)
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
	var b strings.Builder
	outputStructUnionStatistics(&b, opts)
	b.WriteString("\n")
	outputExprStatistics(&b, funcs)
	b.WriteString("\n")
	outputPointerStatistics(&b)
	b.WriteString("\n")
	outputVolatileAccessStatistics(&b)
	b.WriteString("\n")
	outputJumpStatistics(&b)
	b.WriteString("\n")
	outputStmtsStatistics(&b, funcs)
	b.WriteString("\n")
	outputVarFreshness(&b)
	if relyOnIntSize {
		b.WriteString("FYI: the random generator makes assumptions about the integer size. See platform.info for more details.\n")
	}
	if relyOnPtrSize {
		b.WriteString("FYI: the random generator makes assumptions about the pointer size. See platform.info for more details.\n")
	}
	formattedOutput(&b, "total OOB instances added: ", oobCnt)
	return b.String()
}

func outputStructUnionStatistics(b *strings.Builder, opts Options) {
	maxD := len(structDepthCnts) - 1
	if maxD < 0 {
		maxD = 0
	}
	// empty vector → size()-1 wraps in C++; we emit 0
	if len(structDepthCnts) == 0 {
		formattedOutput(b, "max struct depth: ", -1) // size_t(-1) style when empty? C++ size 0 → (0-1)=max
		// upstream: (struct_depth_cnts.size() - 1) as int when empty is -1
	} else {
		formattedOutput(b, "max struct depth: ", maxD)
	}
	b.WriteString("breakdown:\n")
	for i, c := range structDepthCnts {
		b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, c))
	}
	formattedOutput(b, "total union variables: ", unionVarCnt)
	outputBitfields(b, opts)
}

func outputBitfields(b *strings.Builder, opts Options) {
	if !opts.Bitfields {
		return
	}
	b.WriteString("\n")
	formattedOutput(b, "non-zero bitfields defined in structs: ", bitfieldsInTotal)
	formattedOutput(b, "zero bitfields defined in structs: ", unamedBitfieldsInTotal)
	formattedOutput(b, "const bitfields defined in structs: ", constBitfieldsInTotal)
	formattedOutput(b, "volatile bitfields defined in structs: ", volatileBitfieldsInTotal)
	formattedOutput(b, "structs with bitfields in the program: ", CalcTotal(varsWithBitfields))
	b.WriteString("breakdown:\n")
	for i, c := range varsWithBitfields {
		b.WriteString(fmt.Sprintf("   indirect level: %d, occurrence: %d\n", i, c))
	}
	formattedOutput(b, "times a bitfields struct's address is taken: ", varsWithBitfieldsAddressTakenCnt)
	formattedOutput(b, "times a bitfields struct is read: ", rhsBitfieldsStructsVarsCnt)
	formattedOutput(b, "times a bitfields struct is write: ", lhsBitfieldsStructsVarsCnt)
	formattedOutput(b, "times a bitfield is read: ", rhsBitfieldCnt)
	formattedOutput(b, "times a bitfield is write: ", lhsBitfieldCnt)
}

func outputExprStatistics(b *strings.Builder, funcs []*Function) {
	StatExprDepths(funcs)
	maxD := len(exprDepthCnts) - 1
	if len(exprDepthCnts) == 0 {
		formattedOutput(b, "max expression depth: ", -1)
	} else {
		formattedOutput(b, "max expression depth: ", maxD)
	}
	b.WriteString("breakdown:\n")
	for i, c := range exprDepthCnts {
		if c != 0 {
			b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, c))
		}
	}
}

func outputPointerStatistics(b *strings.Builder) {
	// Bookkeeper.cpp:245–318 — all_ptrs / all_aliases when present
	ptrs := AllPtrs
	formattedOutput(b, "total number of pointers: ", len(ptrs))
	if len(ptrs) > 0 {
		b.WriteString("\n")
	}
	formattedOutput(b, "times a variable address is taken: ", addressTakenCnt)
	formattedOutput(b, "times a pointer is dereferenced on RHS: ", CalcTotal(readDereferenceCnts))
	b.WriteString("breakdown:\n")
	for i := 1; i < len(readDereferenceCnts); i++ {
		b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, readDereferenceCnts[i]))
	}
	formattedOutput(b, "times a pointer is dereferenced on LHS: ", CalcTotal(writeDereferenceCnts))
	b.WriteString("breakdown:\n")
	for i := 1; i < len(writeDereferenceCnts); i++ {
		b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, writeDereferenceCnts[i]))
	}
	formattedOutput(b, "times a pointer is compared with null: ", cmpPtrToNull)
	formattedOutput(b, "times a pointer is compared with address of another variable: ", cmpPtrToAddr)
	formattedOutput(b, "times a pointer is compared with another pointer: ", cmpPtrToPtr)
	formattedOutput(b, "times a pointer is qualified to be dereferenced: ", pointerAvailForDeref)
	if len(dereferenceLevelCnts) > 0 {
		b.WriteString("\n")
		formattedOutput(b, "max dereference level: ", len(dereferenceLevelCnts)-1)
		b.WriteString("breakdown:\n")
		for i, c := range dereferenceLevelCnts {
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
				SetError(ErrGeneric)
				break
			}
			// Bookkeeper.cpp:260 — assert(t->eType == ePointer); skip non-pointer aggregates
			if !p.Type.IsPointerLike() {
				continue
			}
			if i < len(AllAliases) {
				totalAlias += len(AllAliases[i])
				if IsVariableInSet(AllAliases[i], NullPtr) {
					hasNull++
				}
			}
			if p.Type.IndirectLevel() > 1 {
				ptPtr++
			} else if pt := p.Type.PtrType(); pt != nil {
				if pt.IsSimple() {
					ptScalar++
				} else if pt.IsStruct() {
					ptStruct++
				}
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
	formattedOutput(b, "times a non-volatile is read: ", readNonVolatileCnt)
	formattedOutput(b, "times a non-volatile is write: ", writeNonVolatileCnt)
	formattedOutput(b, "times a volatile is read: ", readVolatileCnt)
	formattedOutput(b, "   times read thru a pointer: ", readVolatileThruPtrCnt)
	formattedOutput(b, "times a volatile is write: ", writeVolatileCnt)
	formattedOutput(b, "   times written thru a pointer: ", writeVolatileThruPtrCnt)
	total := readNonVolatileCnt + writeNonVolatileCnt + readVolatileCnt + writeVolatileCnt
	percentage := 0.0
	if total > 0 {
		percentage = float64(readNonVolatileCnt+writeNonVolatileCnt) * 100.0 / float64(total)
	}
	formattedOutputf(b, "times a volatile is available for access: ", float64(volatileAvail))
	formattedOutputf(b, "percentage of non-volatile access: ", percentage)
}

func outputJumpStatistics(b *strings.Builder) {
	formattedOutput(b, "forward jumps: ", forwardJumpCnt)
	formattedOutput(b, "backward jumps: ", backwardJumpCnt)
}

func outputStmtsStatistics(b *strings.Builder, funcs []*Function) {
	stmtCnt := StatBlkDepths(funcs)
	formattedOutput(b, "stmts: ", stmtCnt)
	maxD := len(blkDepthCnts) - 1
	if len(blkDepthCnts) == 0 {
		formattedOutput(b, "max block depth: ", -1)
	} else {
		formattedOutput(b, "max block depth: ", maxD)
	}
	b.WriteString("breakdown:\n")
	for i, c := range blkDepthCnts {
		if c != 0 {
			b.WriteString(fmt.Sprintf("   depth: %d, occurrence: %d\n", i, c))
		}
	}
}

func outputVarFreshness(b *strings.Builder) {
	total := useNewVarCnt + useOldVarCnt
	fresh, exist := 0.0, 0.0
	if total > 0 {
		fresh = float64(useNewVarCnt) * 100.0 / float64(total)
		exist = float64(useOldVarCnt) * 100.0 / float64(total)
	}
	formattedOutputf(b, "percentage a fresh-made variable is used: ", fresh)
	formattedOutputf(b, "percentage an existing variable is used: ", exist)
}

// OutputTail mirrors OutputMgr::OutputTail — statistics comment after main.
// OutputMgr.cpp:223–233.
func OutputTail(funcs []*Function, opts Options) string {
	if opts.Concise {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n/************************ statistics *************************\n")
	b.WriteString(OutputStatistics(funcs, opts))
	b.WriteString("********************* end of statistics **********************/\n")
	return b.String()
}
