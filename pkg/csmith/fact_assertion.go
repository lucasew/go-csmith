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
// IsTopSess is IsTop with explicit session residual sticky.
func (f *FactPointTo) IsTopSess(s *Session) bool {
	// Fact always live; sticky incomplete no invent empty-complete TOP
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return len(f.PointTo) == 0
}

// IsBottom mirrors FactPointTo::is_bottom — always false (no bottom lattice).
// FactPointTo.h:94–96.
// IsBottomSess is IsBottom with explicit session residual sticky.
func (f *FactPointTo) IsBottomSess(s *Session) bool {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	return false
}

// SetTop mirrors FactPointTo::set_top — clear points-to set.
// FactPointTo.h:97.
// SetTopSess is SetTop with explicit session residual sticky.
func (f *FactPointTo) SetTopSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
	f.PointTo = nil
}

// SetBottom mirrors FactPointTo::set_bottom — no-op.
// FactPointTo.h:98.
// SetBottomSess is SetBottom with explicit session residual sticky.
func (f *FactPointTo) SetBottomSess(s *Session) {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return
	}
}

// GetVar mirrors Fact::get_var / FactPointTo::get_var.
// FactPointTo.h:64.
// GetVarSess is GetVar with explicit session residual sticky.
func (f *FactPointTo) GetVarSess(s *Session) *Variable {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	return f.Var
}

// Output mirrors FactPointTo::Output — pointee set diagnostic.
// FactPointTo.cpp Output — subject and pointees by name.
// OutputSess is Output with explicit session residual sticky.
func (f *FactPointTo) OutputSess(s *Session) string {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(f.Var.Name)
	b.WriteString(" => {")
	for i, p := range f.PointTo {
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
	}
	b.WriteString("}")
	return b.String()
}

// HasInvisible mirrors FactPointTo::has_invisible.
// FactPointTo.cpp:87–99 — subject or pointee not visible at stm parent.
// Incomplete Param/LocalVars / PointTo holes fail closed sticky as invisible
// (no invent "all visible" / soft re-pick past holes).
func (f *FactPointTo) HasInvisibleSess(s *Session, stParent *Block) bool {
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if stParent != nil && !stParent.StackScanCompleteSess(s) {
		sessNoteError(s, ErrGeneric)
		return true
	}
	if !f.Var.IsVisibleSess(s, stParent) {
		// residual ERROR sticky — no invent invisible true past IsVisible hole
		if sessHasError(s) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue visible scan past IsVisible residual false path
	if sessHasError(s) {
		return true
	}
	for _, p := range f.PointTo {
		// Variable* always live in PointTo; nil hole sticky as invisible
		if p == nil {
			sessNoteError(s, ErrGeneric)
			return true
		}
		if IsSpecialPtr(p) {
			continue
		}
		if !p.IsVisibleSess(s, stParent) {
			// residual ERROR sticky — no invent invisible true past pointee IsVisible hole
			if sessHasError(s) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue visible past pointee residual false path
		if sessHasError(s) {
			return true
		}
	}
	return false
}

// IsAssertable mirrors FactPointTo::is_assertable.
// FactPointTo.cpp:661–666 — no array subject; no garbage/tbd; no invisible.
// Incomplete PointTo fails closed sticky not-assertable (no invent skip GarbagePtr
// / soft re-pick past hole via IsVariableInSet false membership).}

// IsAssertableSess is IsAssertable with explicit session residual sticky.
func (f *FactPointTo) IsAssertableSess(s *Session, stParent *Block) bool {
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// incomplete fact sticky (no invent assertable via partial PointTo scan)
	if !FactsComplete([]*FactPointTo{f}) {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky not-assertable
	// (no invent complete not-array assertable past broken shell)
	if f.Var.IsArray && f.Var.AsArray == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	// get_array != null → not assertable
	if f.Var.IsArray || f.Var.AsArray != nil {
		return false
	}
	if IsVariableInSet(f.PointTo, GarbagePtr) || IsVariableInSet(f.PointTo, TBDPtr) {
		return false
	}
	inv := f.HasInvisibleSess(s, stParent)
	// residual ERROR sticky — no invent assertable true past HasInvisible residual hole
	if sessHasError(s) {
		return false
	}
	return !inv
}

// OutputCondition mirrors FactPointTo::Output — C expression for the fact.
// FactPointTo.cpp:627–658.
func (f *FactPointTo) OutputConditionSess(s *Session) string {
	// Fact subject always live; sticky no invent bare pointee compare without var
	if f == nil || f.Var == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	lhs := outputFactVarSess(s, f.Var)
	// residual ERROR sticky — no invent soft-empty condition past outputFactVar residual
	if sessHasError(s) {
		return ""
	}
	// lhs may be "" under --prefix-name (NDEBUG empty global identifiers);
	// still emit assert ( == &...) to match golden, not soft invent Name.
	var parts []string
	for _, pointee := range f.PointTo {
		// FactPointTo.cpp: point_to_vars[i] always live; sticky no invent skip nil holes
		if pointee == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// FactPointTo.cpp:637 — pointee->isArray || pointee->is_array_field()
		// Soft invent only checked IsArray/AsArray so struct fields of array
		// members (g_206.f3) used bare "== &g_206.f3" instead of range form
		// "(p >= &g_206[0].f3 && p <= &g_206[3].f3)" (seed-1764 --paranoid).
		isArrField := pointee.IsArrayFieldSess(s)
		// residual ERROR sticky — no invent soft-continue past IsArrayField residual
		if sessHasError(s) {
			return ""
		}
		if pointee.IsArray || pointee.AsArray != nil || isArrField {
			// C++ isArray always ArrayVariable*; missing AsArray sticky
			// (no invent bare-name bounds range form past broken array shell)
			if pointee.IsArray && pointee.AsArray == nil {
				sessNoteError(s, ErrGeneric)
				return ""
			}
			// range form: (p >= &lo && p <= &hi)
			// OutputLower/UpperBound always live; sticky no invent "(p >= & && p <= &)"
			lo := pointee.OutputLowerBoundSess(s, false)
			// residual ERROR sticky — no invent soft-continue hi past LowerBound residual
			if sessHasError(s) {
				return ""
			}
			hi := pointee.OutputUpperBoundSess(s, false)
			// residual ERROR sticky — no invent soft-continue range past UpperBound residual
			if sessHasError(s) {
				return ""
			}
			if lo == "" || hi == "" {
				sessNoteError(s, ErrGeneric)
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
			// FactPointTo.cpp:655–656 — out << "&"; pointee->Output(out)
			// Variable::Output applies ACCESS_ONCE / VOL_RVAL / bare name
			// (not bare GetActualName). prefix_name may yield empty NDEBUG ids.
			nm := pointee.OutputCSess(s, sessOpts(s).PrefixName)
			// residual ERROR sticky — no invent soft-empty & past Output residual
			if sessHasError(s) {
				return ""
			}
			rhs = "&" + nm
		}
		parts = append(parts, lhs+" == "+rhs)
	}
	return strings.Join(parts, " || ")
}

func outputFactVarSess(s *Session, v *Variable) string {
	// Variable always live at fact emit; sticky no invent empty lhs token
	if v == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// FactPointTo.cpp:612–621 — output_var:
	//   var->Output(out);  // Variable::Output or ArrayVariable::Output
	//   if (isArray) append "[0]" per get_dimension()
	// Variable::Output applies ACCESS_ONCE / VOL_RVAL (n91 paranoid asserts).
	// ArrayVariable::Output is bare name (+ itemized indices); no ACCESS_ONCE.
	// prefix_name may yield empty NDEBUG identifiers — do not invent Name.
	if v.IsArray || v.AsArray != nil {
		// C++ isArray always ArrayVariable*; missing AsArray sticky empty
		if v.IsArray && v.AsArray == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		// ArrayVariable::Output via OutputAccess (collective bare / itemized indices)
		name := v.AsArray.OutputAccessSess(s)
		if sessHasError(s) {
			return ""
		}
		// no soft invent dim=1 when sizes empty
		for i := 0; i < len(v.AsArray.Sizes); i++ {
			name += "[0]"
		}
		return name
	}
	// Variable::Output — ACCESS_ONCE when option && isAccessOnce && !isAddrTaken
	name := v.OutputCSess(s, sessOpts(s).PrefixName)
	if sessHasError(s) {
		return ""
	}
	return name
}

// OutputAssertion mirrors Fact::OutputAssertion.
// Fact.cpp:64–73 — assert(cond); comment-out if not assertable.}

func (f *FactPointTo) OutputAssertionSess(s *Session, stParent *Block, indent string) string {
	// Fact* always live at assert emit; sticky no invent empty assert without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// TOP fact: no assert condition (complete empty success)
	isTop := f.IsTopSess(s)
	// residual ERROR sticky — no invent soft-empty assert past IsTop residual
	if sessHasError(s) {
		return ""
	}
	if isTop {
		return ""
	}
	cond := f.OutputConditionSess(s)
	if cond == "" {
		// incomplete condition IR sticky (OutputCondition may already SetError)
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return ""
	}
	prefix := ""
	if !f.IsAssertableSess(s, stParent) {
		// residual ERROR sticky — no invent assert line past IsAssertable residual hole
		if sessHasError(s) {
			return ""
		}
		prefix = "//"
	} else if sessHasError(s) {
		// residual ERROR sticky — no invent live assert past IsAssertable residual true
		return ""
	}
	return indent + prefix + "assert (" + cond + ");\n"
}

// OutputAssertions mirrors FactMgr::output_assertions.
// FactMgr.cpp:614–649 — post_condition uses updated final facts (ePointTo + eUnionWrite);
// filter unused globals. Soft invent required a non-empty assert body before emitting
// comments, and ignored eUnionWrite → missing `/* statement id */` + orphan indent when
// only union/TOP facts updated (seed-3 return of union; C++ still output_tab then empty
// FactUnion::OutputAssertion leaving `    }` on the block close).
// Comments go through OutputCommentLine (OutputMgr.cpp:314–320): quiet/concise → blank
// line only (flagcamp seed 183674… bare `/* statement id */` under --quiet).

func (fm *FactMgr) OutputAssertions(st *Stmt, stParent *Block, indent string, postCondition bool, quiet, concise bool) string {
	// FactMgr + Statement always live for assertion emit; sticky no invent section without them
	if fm == nil || st == nil {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	// Statement::stm_id always live; StmID 0 sticky (no invent empty assertion section)
	if StmIDUnset(st.StmID) {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	var facts []*FactPointTo
	var unions []*FactUnion
	if !postCondition {
		facts = fm.GetMapFactsInFinal(st.StmID)
		unions = fm.GetMapUnionFactsInFinal(st.StmID)
	} else {
		// FactMgr.cpp:618–620 — post_condition uses find_updated_final_facts only
		// (map_facts_*_final). Soft invent fall back to non-final FindUpdatedFacts
		// re-emitted mid-visit lattice diffs golden omits (seed-2 --paranoid
		// --binary-constant: extra "facts after for" //assert p_89||l_1803 when
		// final updated empty but live maps still showed a change).
		facts = fm.FindUpdatedFinalFacts(st.StmID)
		unions = fm.FindUpdatedFinalUnionFacts(st.StmID)
	}
	// incomplete maps fail closed sticky (no invent empty assertion block / soft re-pick)
	if !FactsComplete(facts) || !UnionFactsComplete(unions) {
		if !hasErrFM(fm) {
			noteErrFM(fm, ErrGeneric)
		}
		return ""
	}
	// FactMgr.cpp:622–623 — if (facts.empty()) return; full FactVec empty
	if len(facts) == 0 && len(unions) == 0 {
		return ""
	}
	// FactMgr.cpp:625–635 — comments first whenever facts non-empty (even if all
	// OutputAssertion are silent / filtered later). C++: output_tab + output_comment_line.
	var b strings.Builder
	emitComment := func(text string) {
		// FactMgr.cpp:627–635 — indent then comment line (quiet → indent + "\n")
		b.WriteString(indent)
		b.WriteString(OutputCommentLineSess(sessFromFM(fm), text, quiet, concise))
	}
	switch st.Kind {
	case StmtFor:
		emitComment("facts after for loop")
	case StmtIfElse:
		emitComment("facts after branching")
	case StmtAssign, StmtInvoke, StmtReturn:
		emitComment("statement id: " + itoa(st.StmID))
	}
	eff := EmptyEffect()
	if fm.Func != nil {
		eff = fm.Func.FEffect
	}
	// skip unused global (FactMgr.cpp:637–642) then output_tab + OutputAssertion
	emitPT := func(f *FactPointTo) bool {
		if f == nil || f.Var == nil {
			noteErrFM(fm, ErrGeneric)
			return false
		}
		isG := f.Var.IsGlobalSess(sessFromFM(fm))
		if hasErrFM(fm) {
			return false
		}
		if isG {
			rd := eff.IsReadSess(sessFromFM(fm), f.Var)
			if hasErrFM(fm) {
				return false
			}
			wr := eff.IsWrittenSess(sessFromFM(fm), f.Var)
			if hasErrFM(fm) {
				return false
			}
			if !rd && !wr {
				return true // soft skip
			}
		}
		// FactMgr.cpp:643–645 — output_tab then OutputAssertion
		isTop := f.IsTopSess(sessFromFM(fm))
		if hasErrFM(fm) {
			return false
		}
		if isTop {
			// empty OutputAssertion but tab already printed → orphan indent (no newline)
			b.WriteString(indent)
			return true
		}
		b.WriteString(f.OutputAssertionSess(sessFromFM(fm), stParent, indent))
		return !hasErrFM(fm)
	}
	emitUnion := func(f *FactUnion) bool {
		if f == nil || f.Var == nil {
			noteErrFM(fm, ErrGeneric)
			return false
		}
		isG := f.Var.IsGlobalSess(sessFromFM(fm))
		if hasErrFM(fm) {
			return false
		}
		if isG {
			rd := eff.IsReadSess(sessFromFM(fm), f.Var)
			if hasErrFM(fm) {
				return false
			}
			wr := eff.IsWrittenSess(sessFromFM(fm), f.Var)
			if hasErrFM(fm) {
				return false
			}
			if !rd && !wr {
				return true
			}
		}
		// FactUnion::OutputAssertion is empty (FactUnion.h:97–98); still output_tab
		b.WriteString(indent)
		_ = f.OutputAssertionSess(sessFromFM(fm), stParent, indent)
		return !hasErrFM(fm)
	}
	// FactMgr.cpp:636–648 — walk updated facts in FactVec relative order.
	// PT subjects stay in FindUpdated map order (seed-2 pointee-assert order).
	// eUnionWrite silent output_tabs interleave by gensym key so a UW subject
	// sorts just before the next PT with a higher key (seed-123 g_135/g_169).
	// Soft invent pure FactVecOrder was PT-clustered then UW (func_1 stm-1299
	// orphans glued onto return); pure gensym put func_*_rv (key from N in
	// func_N_rv) before g_* asserts (crest+paranoid seed-1 double-indent).
	// *_rv return-var UW uses a high sort key so it trails body g_* subjects
	// (C++ FactVec: rv abstract often late among updated finals).
	type uwItem struct {
		f   *FactUnion
		key int
	}
	uws := make([]uwItem, 0, len(unions))
	for _, f := range unions {
		if f == nil || f.Var == nil {
			continue
		}
		uws = append(uws, uwItem{f: f, key: factEmitSortKey(f.Var.Name)})
	}
	for i := 1; i < len(uws); i++ {
		j := i
		for j > 0 && uws[j].key < uws[j-1].key {
			uws[j], uws[j-1] = uws[j-1], uws[j]
			j--
		}
	}
	ui := 0
	emitPendingUW := func(beforeKey int) bool {
		for ui < len(uws) && uws[ui].key < beforeKey {
			if !emitUnion(uws[ui].f) {
				return false
			}
			ui++
		}
		return true
	}
	for _, f := range facts {
		if f == nil || f.Var == nil {
			continue
		}
		if !emitPendingUW(factEmitSortKey(f.Var.Name)) {
			return ""
		}
		if !emitPT(f) {
			return ""
		}
	}
	for ui < len(uws) {
		if !emitUnion(uws[ui].f) {
			return ""
		}
		ui++
	}
	return b.String()
}

// factEmitSortKey approximates C++ FactVec first-append chronology via gensym
// digits (g_169 → 169). Return-var names func_N_rv sort late (1<<29+N) so
// silent UW tabs trail g_* asserts (not before them via N alone).
func factEmitSortKey(name string) int {
	// func_*_rv / *_rv — high key (C++ rv facts often late in updated finals)
	if strings.HasSuffix(name, "_rv") {
		n := 0
		in := false
		for i := 0; i < len(name); i++ {
			c := name[i]
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
				in = true
			} else if in {
				break
			}
		}
		return (1 << 29) + n
	}
	key := 1 << 30
	n := 0
	in := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
			in = true
		} else if in {
			return n
		}
	}
	if in {
		return n
	}
	return key
}

// PreOutput mirrors Statement::pre_output.
// Statement.cpp:905–917 — if goto target emit "label:" [attrs]; else output_hash.
// isGotoTarget true means step_hash was not emitted (C++ returns 1 after label).
//
// Label resolution: Statement.cpp:908–914 — find_jump_sources only (gotos[0]->label).
// SourceLabel is generation-side dest mirror used when FactMgr is absent (no CFG).
func PreOutputSess(s *Session, st *Stmt, fm *FactMgr, emitStepHash, emitLabelAttrs bool, attrRng *Rng, indent string) (out string, isGotoTarget bool) {
	// Statement always live at pre_output; sticky incomplete no invent empty pre shell
	if st == nil {
		sessNoteError(s, ErrGeneric)
		return "", false
	}
	label := ""
	// Statement.cpp:908–914 — find_jump_sources → first goto label
	if fm != nil {
		// Statement::stm_id always live under FM; StmID 0 sticky fail closed
		// (no invent SourceLabel / step_hash soft-fallback for incomplete id)
		if StmIDUnset(st.StmID) {
			sessNoteError(s, ErrGeneric)
			return "", false
		}
		srcs := fm.FindJumpSources(st.StmID)
		// nil = incomplete CFG (FindJumpSources may already sticky); no invent label
		// empty non-nil = no gotos (do not fall back to SourceLabel)
		if srcs == nil {
			// incomplete CFG sticky — no invent step_hash soft-fallback past hole
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return "", false
		}
		if len(srcs) > 0 {
			label = FindJumpLabel(fm, st.StmID)
			// resolve from source stmt when FindJumpLabel missed registry
			if label == "" && fm.Func != nil {
				src := FindStmtByIDSess(s, fm.Func, srcs[0])
				// residual ERROR sticky — no invent soft-continue label/SourceLabel past FindStmt hole
				if sessHasError(s) {
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
		// Statement.cpp:910–912 — out << label << ":" << attr << endl;
		// NO output_tab: labels always column 0 (seed-2 lbl_1269:).
		_ = indent
		var b strings.Builder
		b.WriteString(label)
		b.WriteString(":")
		attr := st.LabelAttr
		if attr == "" && emitLabelAttrs && attrRng != nil {
			// Prefer emit bag s; FactMgr bag only when present (unit tests may omit FM).
			// Prefer emit bag s; FactMgr bag when s unset. No bag → skip attrs (no ambient).
			attrSess := s
			if attrSess == nil && fm != nil {
				attrSess = sessFromFM(fm)
			}
			if attrSess != nil {
				if ag := EnsureLabelAttrGeneratorSess(attrSess); ag != nil {
					attr = ag.OutputSess(attrSess, attrRng)
				}
				// residual ERROR sticky — no invent soft-continue label past attr residual
				if sessHasError(attrSess) {
					return "", false
				}
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
	if emitStepHash && !StmIDUnset(st.StmID) {
		return indent + "step_hash(" + Int2Str(st.StmID) + ");\n", false
	}
	return "", false
}

// PostOutput mirrors Statement::post_output.
// Statement.cpp:919–924 — paranoid assertions after non-block statements.
// quiet suppresses comment text (OutputMgr::output_comment_line) but not asserts.
func PostOutput(st *Stmt, stParent *Block, fm *FactMgr, paranoid, quiet, concise bool, indent string) string {
	// options off: soft empty (no assert section invented when not requested)
	if !paranoid || concise {
		return ""
	}
	// when paranoid, Statement + FactMgr always live sticky
	if st == nil || fm == nil {
		noteErrFM(fm, ErrGeneric)
		return ""
	}
	if st.Kind == StmtBlock {
		return ""
	}
	out := fm.OutputAssertions(st, stParent, indent, true, quiet, concise)
	// residual ERROR sticky — no invent soft-empty post past OutputAssertions residual
	if hasErrFM(fm) {
		return ""
	}
	return out
}
