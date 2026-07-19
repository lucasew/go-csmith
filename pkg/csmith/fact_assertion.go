// Upstream: Fact::OutputAssertion; FactPointTo::Output / is_assertable / has_invisible;
// FactMgr::output_assertions; Statement::post_output.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strings"
)

// IsTop mirrors FactPointTo::is_top — empty points-to set.
// FactPointTo.h:93.
func (f *FactPointTo) IsTop() bool {
	return f == nil || len(f.PointTo) == 0
}

// HasInvisible mirrors FactPointTo::has_invisible.
// FactPointTo.cpp:87–99 — subject or pointee not visible at stm parent.
func (f *FactPointTo) HasInvisible(stParent *Block) bool {
	if f == nil || f.Var == nil {
		return true
	}
	if !f.Var.IsVisible(stParent) {
		return true
	}
	for _, p := range f.PointTo {
		if p == nil || IsSpecialPtr(p) {
			continue
		}
		if !p.IsVisible(stParent) {
			return true
		}
	}
	return false
}

// IsAssertable mirrors FactPointTo::is_assertable.
// FactPointTo.cpp:661–666 — no array subject; no garbage/tbd; no invisible.
func (f *FactPointTo) IsAssertable(stParent *Block) bool {
	if f == nil || f.Var == nil {
		return false
	}
	// get_array != null → not assertable
	if f.Var.IsArray || f.Var.AsArray != nil {
		return false
	}
	if IsVariableInSet(f.PointTo, GarbagePtr) || IsVariableInSet(f.PointTo, TBDPtr) {
		return false
	}
	return !f.HasInvisible(stParent)
}

// OutputCondition mirrors FactPointTo::Output — C expression for the fact.
// FactPointTo.cpp:627–658.
func (f *FactPointTo) OutputCondition() string {
	if f == nil || f.Var == nil {
		return ""
	}
	lhs := outputFactVar(f.Var)
	// subject always live Output; no invent " == 0" / " >= &" without lhs
	if lhs == "" {
		return ""
	}
	var parts []string
	for _, pointee := range f.PointTo {
		// FactPointTo.cpp: point_to_vars[i] always live; no invent skip nil holes
		if pointee == nil {
			return ""
		}
		if pointee.IsArray || (pointee.AsArray != nil) {
			// range form: (p >= &lo && p <= &hi)
			// OutputLower/UpperBound always live; no invent "(p >= & && p <= &)"
			lo := pointee.OutputLowerBound(false)
			hi := pointee.OutputUpperBound(false)
			if lo == "" || hi == "" {
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
			// pointee->Output always live name; no invent bare "&"
			nm := pointee.GetActualName(false)
			if nm == "" {
				return ""
			}
			rhs = "&" + nm
		}
		parts = append(parts, lhs+" == "+rhs)
	}
	return strings.Join(parts, " || ")
}

func outputFactVar(v *Variable) string {
	if v == nil {
		return ""
	}
	s := v.GetActualName(false)
	// no invent "[0]" indices without identifier
	if s == "" {
		return ""
	}
	// FactPointTo.cpp:612–621 — output_var: for array, [0] per get_dimension()
	// no soft invent dim=1 when sizes empty
	if v.IsArray || v.AsArray != nil {
		dim := 0
		if v.AsArray != nil {
			dim = len(v.AsArray.Sizes)
		} else if len(v.ArraySizes) > 0 {
			dim = len(v.ArraySizes)
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
	if f == nil || f.IsTop() {
		return ""
	}
	cond := f.OutputCondition()
	if cond == "" {
		return ""
	}
	prefix := ""
	if !f.IsAssertable(stParent) {
		prefix = "//"
	}
	return indent + prefix + "assert (" + cond + ");\n"
}

// OutputAssertions mirrors FactMgr::output_assertions.
// FactMgr.cpp:614–649 — post_condition uses updated final facts; filter unused globals.
func (fm *FactMgr) OutputAssertions(st *Stmt, stParent *Block, indent string, postCondition bool) string {
	if fm == nil || st == nil || st.StmID <= 0 {
		return ""
	}
	var facts []*FactPointTo
	if !postCondition {
		facts = fm.MapFactsInFinal[st.StmID]
	} else {
		facts = fm.FindUpdatedFinalFacts(st.StmID)
		// fall back to non-final updated if final empty
		if len(facts) == 0 {
			facts = fm.FindUpdatedFacts(st.StmID)
		}
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
		// Fact* always live in fact maps; no invent skip nil holes
		if f == nil || f.Var == nil {
			return ""
		}
		// skip globals neither read nor written in this function
		if f.Var.IsGlobal() && !eff.IsRead(f.Var) && !eff.IsWritten(f.Var) {
			continue
		}
		// IsTop / empty OutputAssertion intentionally silent; non-nil fact still live
		body.WriteString(f.OutputAssertion(stParent, indent))
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
	if st == nil {
		return "", false
	}
	label := ""
	// Statement.cpp:908–914 — find_jump_sources → first goto label
	if fm != nil && st.StmID > 0 {
		if srcs := fm.FindJumpSources(st.StmID); len(srcs) > 0 {
			label = FindJumpLabel(fm, st.StmID)
			// resolve from source stmt when FindJumpLabel missed registry
			if label == "" && fm.Func != nil {
				if src := FindStmtByID(fm.Func, srcs[0]); src != nil {
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
	if st == nil || fm == nil || !paranoid || concise {
		return ""
	}
	if st.Kind == StmtBlock {
		return ""
	}
	return fm.OutputAssertions(st, stParent, indent, true)
}
