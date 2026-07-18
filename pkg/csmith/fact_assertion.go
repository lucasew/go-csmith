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
	var parts []string
	for _, pointee := range f.PointTo {
		if pointee == nil {
			continue
		}
		lhs := outputFactVar(f.Var)
		if pointee.IsArray || (pointee.AsArray != nil) {
			// range form: (p >= &lo && p <= &hi)
			lo := pointee.OutputLowerBound(false)
			hi := pointee.OutputUpperBound(false)
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
			rhs = "&" + pointee.GetActualName(false)
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
	// array of pointers: assert first element only
	if v.IsArray || v.AsArray != nil {
		dim := 1
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
	eff := EmptyEffect()
	if fm.Func != nil {
		eff = fm.Func.FEffect
	}
	for _, f := range facts {
		if f == nil || f.Var == nil {
			continue
		}
		// skip globals neither read nor written in this function
		if f.Var.IsGlobal() && !eff.IsRead(f.Var) && !eff.IsWritten(f.Var) {
			continue
		}
		b.WriteString(f.OutputAssertion(stParent, indent))
	}
	return b.String()
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
