// Upstream: Fact::OutputAssertion; FactPointTo::Output / is_assertable / has_invisible;
// FactMgr::output_assertions; Statement::post_output.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strings"
)

// IsTop mirrors FactPointTo::is_top — empty points-to set.
// FactPointTo.h:93.
// Incomplete Fact shell sticky false (no invent TOP / soft re-pick past hole).
func (f *FactPointTo) IsTop() bool {
	// Fact always live; sticky incomplete no invent empty-complete TOP
	if f == nil {
		SetError(ErrGeneric)
		return false
	}
	return len(f.PointTo) == 0
}

// HasInvisible mirrors FactPointTo::has_invisible.
// FactPointTo.cpp:87–99 — subject or pointee not visible at stm parent.
// Incomplete Param/LocalVars / PointTo holes fail closed sticky as invisible
// (no invent "all visible" / soft re-pick past holes).
func (f *FactPointTo) HasInvisible(stParent *Block) bool {
	if f == nil || f.Var == nil {
		SetError(ErrGeneric)
		return true
	}
	if stParent != nil && !stParent.StackScanComplete() {
		SetError(ErrGeneric)
		return true
	}
	if !f.Var.IsVisible(stParent) {
		// residual ERROR sticky — no invent invisible true past IsVisible hole
		if HasError() {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue visible scan past IsVisible residual false path
	if HasError() {
		return true
	}
	for _, p := range f.PointTo {
		// Variable* always live in PointTo; nil hole sticky as invisible
		if p == nil {
			SetError(ErrGeneric)
			return true
		}
		if IsSpecialPtr(p) {
			continue
		}
		if !p.IsVisible(stParent) {
			// residual ERROR sticky — no invent invisible true past pointee IsVisible hole
			if HasError() {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue visible past pointee residual false path
		if HasError() {
			return true
		}
	}
	return false
}

// IsAssertable mirrors FactPointTo::is_assertable.
// FactPointTo.cpp:661–666 — no array subject; no garbage/tbd; no invisible.
// Incomplete PointTo fails closed sticky not-assertable (no invent skip GarbagePtr
// / soft re-pick past hole via IsVariableInSet false membership).
func (f *FactPointTo) IsAssertable(stParent *Block) bool {
	if f == nil || f.Var == nil {
		SetError(ErrGeneric)
		return false
	}
	// incomplete fact sticky (no invent assertable via partial PointTo scan)
	if !FactsComplete([]*FactPointTo{f}) {
		SetError(ErrGeneric)
		return false
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky not-assertable
	// (no invent complete not-array assertable past broken shell)
	if f.Var.IsArray && f.Var.AsArray == nil {
		SetError(ErrGeneric)
		return false
	}
	// get_array != null → not assertable
	if f.Var.IsArray || f.Var.AsArray != nil {
		return false
	}
	if IsVariableInSet(f.PointTo, GarbagePtr) || IsVariableInSet(f.PointTo, TBDPtr) {
		return false
	}
	inv := f.HasInvisible(stParent)
	// residual ERROR sticky — no invent assertable true past HasInvisible residual hole
	if HasError() {
		return false
	}
	return !inv
}

// OutputCondition mirrors FactPointTo::Output — C expression for the fact.
// FactPointTo.cpp:627–658.
func (f *FactPointTo) OutputCondition() string {
	// Fact subject always live; sticky no invent bare pointee compare without var
	if f == nil || f.Var == nil {
		SetError(ErrGeneric)
		return ""
	}
	lhs := outputFactVar(f.Var)
	// residual ERROR sticky — no invent soft-empty condition past outputFactVar residual
	if HasError() {
		return ""
	}
	// subject always live Output; sticky no invent " == 0" / " >= &" without lhs
	if lhs == "" {
		SetError(ErrGeneric)
		return ""
	}
	var parts []string
	for _, pointee := range f.PointTo {
		// FactPointTo.cpp: point_to_vars[i] always live; sticky no invent skip nil holes
		if pointee == nil {
			SetError(ErrGeneric)
			return ""
		}
		if pointee.IsArray || (pointee.AsArray != nil) {
			// C++ isArray always ArrayVariable*; missing AsArray sticky
			// (no invent bare-name bounds range form past broken array shell)
			if pointee.IsArray && pointee.AsArray == nil {
				SetError(ErrGeneric)
				return ""
			}
			// range form: (p >= &lo && p <= &hi)
			// OutputLower/UpperBound always live; sticky no invent "(p >= & && p <= &)"
			lo := pointee.OutputLowerBound(false)
			// residual ERROR sticky — no invent soft-continue hi past LowerBound residual
			if HasError() {
				return ""
			}
			hi := pointee.OutputUpperBound(false)
			// residual ERROR sticky — no invent soft-continue range past UpperBound residual
			if HasError() {
				return ""
			}
			if lo == "" || hi == "" {
				SetError(ErrGeneric)
				return ""
			}
			parts = append(parts, "("+lhs+" >= &"+lo+" && "+lhs+" <= &"+hi+")")
			continue
		}
		rhs := ""
		switch {
		case pointee == GarbagePtr:
			rhs = "dangling"
		case pointee == TBDPtr:
			rhs = "tbd"
		case pointee == NullPtr:
			rhs = "0"
		default:
			// pointee->Output always live name; sticky no invent bare "&"
			nm := pointee.GetActualName(false)
			// residual ERROR sticky — no invent soft-empty & past GetActualName residual
			if HasError() {
				return ""
			}
			if nm == "" {
				SetError(ErrGeneric)
				return ""
			}
			rhs = "&" + nm
		}
		parts = append(parts, lhs+" == "+rhs)
	}
	return strings.Join(parts, " || ")
}

func outputFactVar(v *Variable) string {
	// Variable always live at fact emit; sticky no invent empty lhs token
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	s := v.GetActualName(false)
	// sticky no invent "[0]" indices without identifier
	if s == "" {
		SetError(ErrGeneric)
		return ""
	}
	// FactPointTo.cpp:612–621 — output_var: for array, [0] per get_dimension()
	// no soft invent dim=1 when sizes empty
	// C++ isArray always ArrayVariable*; missing AsArray sticky empty
	// (no invent [0] indices from ArraySizes alone past broken shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	if v.IsArray || v.AsArray != nil {
		dim := 0
		if v.AsArray != nil {
			dim = len(v.AsArray.Sizes)
		}
		for i := 0; i < dim; i++ {
			s += "[0]"
		}
	}
	return s
}

// OutputAssertion mirrors Fact::OutputAssertion.
// Fact.cpp:64–73 — assert(cond); comment-out if not assertable.
func (f *FactPointTo) OutputAssertion(stParent *Block, indent string) string {
	// Fact* always live at assert emit; sticky no invent empty assert without it
	if f == nil {
		SetError(ErrGeneric)
		return ""
	}
	// TOP fact: no assert condition (complete empty success)
	if f.IsTop() {
		return ""
	}
	cond := f.OutputCondition()
	if cond == "" {
		// incomplete condition IR sticky (OutputCondition may already SetError)
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	prefix := ""
	if !f.IsAssertable(stParent) {
		// residual ERROR sticky — no invent assert line past IsAssertable residual hole
		if HasError() {
			return ""
		}
		prefix = "//"
	} else if HasError() {
		// residual ERROR sticky — no invent live assert past IsAssertable residual true
		return ""
	}
	return indent + prefix + "assert (" + cond + ");\n"
}

// OutputAssertions mirrors FactMgr::output_assertions.
// FactMgr.cpp:614–649 — post_condition uses updated final facts; filter unused globals.
func (fm *FactMgr) OutputAssertions(st *Stmt, stParent *Block, indent string, postCondition bool) string {
	// FactMgr + Statement always live for assertion emit; sticky no invent section without them
	if fm == nil || st == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Statement::stm_id always live; StmID 0 sticky (no invent empty assertion section)
	if st.StmID <= 0 {
		SetError(ErrGeneric)
		return ""
	}
	var facts []*FactPointTo
	if !postCondition {
		facts = fm.GetMapFactsInFinal(st.StmID)
	} else {
		facts = fm.FindUpdatedFinalFacts(st.StmID)
		// fall back to non-final updated if final empty (complete empty only)
		if FactsComplete(facts) && len(facts) == 0 {
			facts = fm.FindUpdatedFacts(st.StmID)
		}
	}
	// incomplete maps fail closed sticky (no invent empty assertion block / soft re-pick)
	if !FactsComplete(facts) {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	if len(facts) == 0 {
		return ""
	}
	// emit assertions first; no invent comment-only shell when all facts filtered
	eff := EmptyEffect()
	if fm.Func != nil {
		eff = fm.Func.FEffect
	}
	var body strings.Builder
	for _, f := range facts {
		// Fact* always live in fact maps; nil hole sticky (no invent skip holes)
		if f == nil || f.Var == nil {
			SetError(ErrGeneric)
			return ""
		}
		// skip globals neither read nor written in this function
		if f.Var.IsGlobal() && !eff.IsRead(f.Var) && !eff.IsWritten(f.Var) {
			// residual ERROR sticky — no invent soft-skip then partial assert emit past hole
			if HasError() {
				return ""
			}
			continue
		}
		// IsTop / empty OutputAssertion intentionally silent; non-nil fact still live
		body.WriteString(f.OutputAssertion(stParent, indent))
		// residual ERROR sticky — no invent partial assertion section past hard IR hole
		if HasError() {
			return ""
		}
	}
	if body.Len() == 0 {
		return ""
	}
	var b strings.Builder
	// comments for compound / simple (FactMgr.cpp:625–635)
	switch st.Kind {
	case StmtFor:
		b.WriteString(indent + "/* facts after for loop */\n")
	case StmtIfElse:
		b.WriteString(indent + "/* facts after branching */\n")
	case StmtAssign, StmtInvoke, StmtReturn:
		b.WriteString(indent + "/* statement id: " + itoa(st.StmID) + " */\n")
	}
	b.WriteString(body.String())
	return b.String()
}

// PreOutput mirrors Statement::pre_output.
// Statement.cpp:905–917 — if goto target emit "label:" [attrs]; else output_hash.
// isGotoTarget true means step_hash was not emitted (C++ returns 1 after label).
//
// Label resolution: Statement.cpp:908–914 — find_jump_sources only (gotos[0]->label).
// SourceLabel is generation-side dest mirror used when FactMgr is absent (no CFG).
func PreOutput(st *Stmt, fm *FactMgr, emitStepHash, emitLabelAttrs bool, attrRng *Rng, indent string) (out string, isGotoTarget bool) {
	// Statement always live at pre_output; sticky incomplete no invent empty pre shell
	if st == nil {
		SetError(ErrGeneric)
		return "", false
	}
	label := ""
	// Statement.cpp:908–914 — find_jump_sources → first goto label
	if fm != nil {
		// Statement::stm_id always live under FM; StmID 0 sticky fail closed
		// (no invent SourceLabel / step_hash soft-fallback for incomplete id)
		if st.StmID <= 0 {
			SetError(ErrGeneric)
			return "", false
		}
		srcs := fm.FindJumpSources(st.StmID)
		// nil = incomplete CFG (FindJumpSources may already sticky); no invent label
		// empty non-nil = no gotos (do not fall back to SourceLabel)
		if srcs == nil {
			// incomplete CFG sticky — no invent step_hash soft-fallback past hole
			if !HasError() {
				SetError(ErrGeneric)
			}
			return "", false
		}
		if len(srcs) > 0 {
			label = FindJumpLabel(fm, st.StmID)
			// resolve from source stmt when FindJumpLabel missed registry
			if label == "" && fm.Func != nil {
				src := FindStmtByID(fm.Func, srcs[0])
				// residual ERROR sticky — no invent soft-continue label/SourceLabel past FindStmt hole
				if HasError() {
					return "", false
				}
				if src != nil {
					label = src.Label
				}
			}
		}
		// with FactMgr: do not fall back to SourceLabel without jump sources
		// (C++ only labels when find_jump_sources non-empty)
	} else if st.SourceLabel != "" {
		// no DFA / no FM: emit generation-time dest label
		label = st.SourceLabel
	}
	if label != "" {
		var b strings.Builder
		b.WriteString(indent)
		b.WriteString(label)
		b.WriteString(":")
		attr := st.LabelAttr
		if attr == "" && emitLabelAttrs && attrRng != nil {
			attr = EnsureLabelAttrGenerator().Output(attrRng)
			// residual ERROR sticky — no invent soft-continue label past attr residual
			if HasError() {
				return "", false
			}
		}
		if attr != "" {
			b.WriteString(attr)
		}
		b.WriteString("\n")
		// Statement.cpp:905–914 — return 1 after label (no output_hash)
		return b.String(), true
	}
	// Statement.cpp:916 — output_hash when not a jump target
	// Statement.cpp:927–931 / OutputMgr.cpp:161–167
	// emitStepHash is set only when StepHashByStmt && ComputeHash (Block make)
	// so step_hash(n) is never invented without live helper defs
	if emitStepHash && st.StmID > 0 {
		return indent + "step_hash(" + Int2Str(st.StmID) + ");\n", false
	}
	return "", false
}

// PostOutput mirrors Statement::post_output.
// Statement.cpp:919–924 — paranoid assertions after non-block statements.
func PostOutput(st *Stmt, stParent *Block, fm *FactMgr, paranoid, concise bool, indent string) string {
	// options off: soft empty (no assert section invented when not requested)
	if !paranoid || concise {
		return ""
	}
	// when paranoid, Statement + FactMgr always live sticky
	if st == nil || fm == nil {
		SetError(ErrGeneric)
		return ""
	}
	if st.Kind == StmtBlock {
		return ""
	}
	return fm.OutputAssertions(st, stParent, indent, true)
}
