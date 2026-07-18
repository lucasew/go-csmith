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
	b.WriteString("XXX ")
	b.WriteString(msg)
	b.WriteString(fmt.Sprintf("%d\n", num))
}

func formattedOutputf(b *strings.Builder, msg string, num float64) {
	b.WriteString("XXX ")
	b.WriteString(msg)
	// Bookkeeper uses default precision; ~3 digits like upstream out.precision(3)
	b.WriteString(fmt.Sprintf("%g\n", num))
}

// RecordAddressTaken mirrors Bookkeeper::record_address_taken.
// Bookkeeper.cpp:324–334.
func RecordAddressTaken(v *Variable) {
	if v == nil || v.Type == nil {
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
	if v == nil {
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
func RecordBitfieldsReads(v *Variable) {
	if v == nil || v.Type == nil {
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
func RecordBitfieldsWrites(v *Variable) {
	if v == nil || v.Type == nil {
		return
	}
	if v.Type.HasBitfields() {
		lhsBitfieldsStructsVarsCnt++
	}
	if v.IsBitfield {
		lhsBitfieldCnt++
	}
}

// RecordPointerComparisons mirrors Bookkeeper::record_pointer_comparisons (expr terms).
// Bookkeeper.cpp:361–382 — simplified for our Expression terms.
func RecordPointerComparisons(lhs, rhs *Expression) {
	if lhs == nil || rhs == nil {
		return
	}
	if lhs.Term == TermFunction || rhs.Term == TermFunction {
		return
	}
	lt, rt := lhs.ExprType, rhs.ExprType
	if lt == nil && lhs.Var != nil {
		lt = lhs.Var.Type
	}
	if rt == nil && rhs.Var != nil {
		rt = rhs.Var.Type
	}
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
		if lhs.IndirectLevel() == rhs.IndirectLevel() {
			cmpPtrToPtr++
		} else {
			cmpPtrToAddr++
		}
	}
}

// RecordVarsWithBitfields mirrors Bookkeeper::record_vars_with_bitfields.
// Bookkeeper.cpp:460–475 subset.
func RecordVarsWithBitfields(t *Type) {
	if t == nil || !t.HasBitfields() {
		return
	}
	level := t.IndirectLevel()
	IncrCounter(&varsWithBitfields, level)
}

// RecordTypeWithBitfields mirrors Bookkeeper::record_type_with_bitfields.
// Bookkeeper.cpp:480–520 subset — count bitfield members when defining a type.
func RecordTypeWithBitfields(t *Type) {
	if t == nil || !t.IsAggregate() {
		return
	}
	hasBF := false
	for _, f := range t.Fields {
		if f.BitWidth == 0 {
			unamedBitfieldsInTotal++
			hasBF = true
			continue
		}
		if f.BitWidth > 0 {
			bitfieldsInTotal++
			hasBF = true
			if f.Qfer.IsConst() {
				constBitfieldsInTotal++
			}
			if f.Qfer.IsVolatile() {
				volatileBitfieldsInTotal++
			}
		}
	}
	if hasBF {
		structsWithBitfields++
	}
}

// RecordVarCreated mirrors use_new_var path in VariableSelector::SelectVariable.
// VariableSelector.cpp:1230–1236.
func RecordVarCreated(v *Variable) {
	if v == nil || v.Type == nil {
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

// ExpressionComplexity mirrors Expression::get_complexity (lightweight).
// Expression.cpp — constants/vars 0; binary-ish nest +1.
func ExpressionComplexity(e *Expression) int {
	if e == nil {
		return 0
	}
	switch e.Term {
	case TermConstant, TermVariable:
		return 0
	case TermFunction:
		return 1
	case TermAssignment:
		c := 1
		if e.Assign != nil && e.Assign.Expr != nil {
			c += ExpressionComplexity(e.Assign.Expr)
		}
		return c
	case TermCommaExpr:
		return 1 + ExpressionComplexity(e.CommaLHS) + ExpressionComplexity(e.CommaRHS)
	default:
		return 0
	}
}

// collectStmtExprs appends expressions reachable from one Stmt.
func collectStmtExprs(st *Stmt, out *[]*Expression) {
	if st == nil || out == nil {
		return
	}
	if st.Expr != nil {
		*out = append(*out, st.Expr)
	}
	if st.Loop != nil {
		// LoopControl may hold expressions via fields we walk if present
	}
	if st.Then != nil {
		for i := range st.Then.Stmts {
			collectStmtExprs(&st.Then.Stmts[i], out)
		}
	}
	if st.Else != nil {
		for i := range st.Else.Stmts {
			collectStmtExprs(&st.Else.Stmts[i], out)
		}
	}
}

// StatExprDepths mirrors Bookkeeper::stat_expr_depths over non-builtin funcs.
// Bookkeeper.cpp:224–230.
func StatExprDepths(funcs []*Function) {
	exprDepthCnts = nil
	for _, f := range funcs {
		if f == nil || f.IsBuiltin || f.Body == nil {
			continue
		}
		for i := range f.Body.Stmts {
			var exprs []*Expression
			collectStmtExprs(&f.Body.Stmts[i], &exprs)
			for _, e := range exprs {
				IncrCounter(&exprDepthCnts, ExpressionComplexity(e))
			}
		}
	}
}

// StatBlkDepths mirrors Bookkeeper::stat_blk_depths — count non-block stmts by nest depth.
// Bookkeeper.cpp:128–152 simplified (depth from nesting of Then/Else).
func StatBlkDepths(funcs []*Function) int {
	blkDepthCnts = nil
	cnt := 0
	var walk func(st *Stmt, depth int)
	walk = func(st *Stmt, depth int) {
		if st == nil {
			return
		}
		// non-block statements: our Stmt kinds are never eBlock containers as Stmt
		IncrCounter(&blkDepthCnts, depth)
		cnt++
		if st.Then != nil {
			for i := range st.Then.Stmts {
				walk(&st.Then.Stmts[i], depth+1)
			}
		}
		if st.Else != nil {
			for i := range st.Else.Stmts {
				walk(&st.Else.Stmts[i], depth+1)
			}
		}
	}
	for _, f := range funcs {
		if f == nil || f.IsBuiltin || f.Body == nil {
			continue
		}
		for i := range f.Body.Stmts {
			walk(&f.Body.Stmts[i], 0)
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
			if p == nil || p.Type == nil {
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
