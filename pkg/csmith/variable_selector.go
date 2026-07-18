// Upstream: VariableSelector.cpp / VariableSelector.h
// (GlobalList, choose_ok_var, GenerateNewGlobal name+qfer path, SelectGlobal empty→create).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VariableSelector holds AllVars / GlobalList inventories (static vectors in C++).
type VariableSelector struct {
	AllVars                []*Variable
	GlobalList             []*Variable
	GlobalNonvolatilesList []*Variable
	Sym                    GenSym
	Opts                   Options
	Probs                  *Probabilities
	// TmpCount is VariableSelector::tmp_count (incremented in GenerateNewGlobal).
	TmpCount int
	// VarCreated is VariableSelector::var_created.
	VarCreated bool
	// Types is session Type::derived_types (optional).
	Types *TypeEnv
	// Arrays tracks ArrayVariables for emission.
	Arrays []*ArrayVariable
}

// NewVariableSelector constructs an empty selector with opts-derived probs.
func NewVariableSelector(opts Options) *VariableSelector {
	return &VariableSelector{
		Opts:  opts,
		Probs: NewProbabilities(opts),
	}
}

// RandomGlobalName mirrors VariableSelector.cpp RandomGlobalName → gensym("g_").
func (vs *VariableSelector) RandomGlobalName() string {
	return vs.Sym.Next("g_")
}

// RandomLocalName mirrors RandomLocalName → gensym("l_").
func (vs *VariableSelector) RandomLocalName() string {
	return vs.Sym.Next("l_")
}

// RandomParamName mirrors RandomParamName → gensym("p_").
func (vs *VariableSelector) RandomParamName() string {
	return vs.Sym.Next("p_")
}

// ChooseOKVar mirrors VariableSelector::choose_ok_var(vector<Variable*>).
// VariableSelector.cpp:318–337 — rnd pick; collective array → itemize.
func ChooseOKVar(r *Rng, vars []*Variable) *Variable {
	n := len(vars)
	if n == 0 {
		return nil
	}
	var v *Variable
	if n == 1 {
		v = vars[0]
	} else {
		// DepthSpec::depth_guard_by_depth(1) for multi-choice — random mode always GOOD
		_ = DepthGuardByDepth(Options{}, 1)
		if r == nil {
			return vars[0]
		}
		v = vars[r.RndUpto(uint32(n))]
	}
	// if collective array, return itemized member (VariableSelector.cpp:332–337)
	if v != nil && v.IsArray && v.AsArray != nil && v.AsArray.Collective == nil && r != nil {
		if item := v.AsArray.Itemize(r); item != nil {
			return &item.Variable
		}
	}
	return v
}

// ChooseOKVarExactType filters vars whose Type matches want with eExact.
func ChooseOKVarExactType(r *Rng, vars []*Variable, want *Type) *Variable {
	return ChooseOKVarMatch(r, vars, want, MatchExact, false)
}

// FindAllVisibleVars mirrors VariableSelector::find_all_visible_vars.
// VariableSelector.cpp:752–759 — GlobalList + block chain locals (no params).
func (vs *VariableSelector) FindAllVisibleVars(b *Block) []*Variable {
	var vars []*Variable
	if vs != nil {
		vars = append(vars, vs.GlobalList...)
	}
	for b != nil {
		vars = append(vars, b.LocalVars...)
		b = b.Parent
	}
	return vars
}

// FindAllNonArrayVisibleVars mirrors find_all_non_array_visible_vars.
// VariableSelector.cpp:713–735 — non-array globals, params, non-array locals.
func (vs *VariableSelector) FindAllNonArrayVisibleVars(b *Block) []*Variable {
	var vars []*Variable
	if vs != nil {
		for _, v := range vs.GlobalList {
			if v != nil && !v.IsArray {
				vars = append(vars, v)
			}
		}
	}
	// resolve func via block or parents (inner blocks may leave Func nil)
	var f *Function
	for bb := b; bb != nil; bb = bb.Parent {
		if bb.Func != nil {
			f = bb.Func
			break
		}
	}
	if f != nil {
		vars = append(vars, f.Param...)
	}
	for b != nil {
		for _, v := range b.LocalVars {
			if v != nil && !v.IsArray {
				vars = append(vars, v)
			}
		}
		b = b.Parent
	}
	return vars
}

// GetAllLocalVars mirrors VariableSelector::get_all_local_vars.
// VariableSelector.cpp:747–751.
func GetAllLocalVars(b *Block) []*Variable {
	var vars []*Variable
	for b != nil {
		vars = append(vars, b.LocalVars...)
		b = b.Parent
	}
	return vars
}

// IsEligibleVar mirrors VariableSelector::is_eligible_var (core effect rules).
// VariableSelector.cpp:216–290 — collective/FactUnion deferred.
func IsEligibleVar(v *Variable, derefLevel int, access Access, cg CGContext) bool {
	if v == nil {
		return false
	}
	// VariableSelector.cpp:232–234 — partial volatile through pointer
	if derefLevel > 0 && v.IsPartialVolatileAfterDeref(derefLevel) {
		return false
	}
	eff := cg.EffectContext()
	isConst := v.IsConstAfterDeref(derefLevel)
	isVol := v.IsVolatileAfterDeref(derefLevel) || v.IsVolatile()

	// volatile + non-SE-free context → reject
	if isVol && !eff.IsSideEffectFree() {
		return false
	}
	// cannot read/write a var being written in context
	if access == AccessRead || access == AccessWrite {
		if eff.IsWrittenPartially(v) {
			return false
		}
	}
	// cannot write a var being read (deref_level==0)
	if access == AccessWrite && derefLevel == 0 && eff.IsReadPartially(v) {
		return false
	}
	// cannot write const
	if access == AccessWrite && isConst {
		return false
	}
	// VariableSelector.cpp:277–287 — nonreadable / nonwritable from context
	if access == AccessRead && cg.IsNonReadable(v) {
		return false
	}
	if access == AccessWrite && cg.IsNonWritable(v) {
		return false
	}
	return true
}

// HasDereferenceableVar mirrors VariableSelector::has_dereferenceable_var.
// VariableSelector.cpp:198–210 — type is_dereferenced_from + is_valid_ptr.
func HasDereferenceableVar(vars []*Variable, typ *Type, cg CGContext, opts Options) bool {
	if typ == nil {
		return false
	}
	var facts []*FactPointTo
	if cg.FM != nil {
		facts = cg.FM.GlobalFacts
	}
	for _, v := range vars {
		if v == nil || v.Type == nil {
			continue
		}
		if typ.IsDereferencedFrom(v.Type) && IsValidPtr(v, facts, opts.NullPointerDerefProb, opts.DeadPointerDerefProb) {
			return true
		}
	}
	return false
}

// ChooseVar mirrors VariableSelector::choose_var type+eligibility filter.
// VariableSelector.cpp:394–447 subset — expand, match, is_eligible_var.
func ChooseVar(
	r *Rng,
	vars []*Variable,
	access Access,
	cg CGContext,
	want *Type,
	mt MatchType,
) *Variable {
	if want == nil {
		// still filter eligibility on raw list
		var ok []*Variable
		for _, v := range vars {
			if v != nil && IsEligibleVar(v, 0, access, cg) {
				ok = append(ok, v)
			}
		}
		return ChooseOKVar(r, ok)
	}
	cands := vars
	if want.IsSimple() || want.IsAggregate() {
		cands = ExpandStructUnionVars(vars, want)
	}
	var ok []*Variable
	for _, x := range cands {
		if x == nil || x.Type == nil {
			continue
		}
		if !want.Match(x.Type, mt) {
			continue
		}
		deref := 0
		if x.Type != nil {
			deref = x.Type.IndirectLevel() - want.IndirectLevel()
		}
		if !IsEligibleVar(x, deref, access, cg) {
			continue
		}
		ok = append(ok, x)
	}
	return ChooseOKVar(r, ok)
}

// ExpandStructUnionVars mirrors VariableSelector::expand_struct_union_vars.
// VariableSelector.cpp:156–173 — replace non-matching aggregates with field_vars.
func ExpandStructUnionVars(vars []*Variable, want *Type) []*Variable {
	out := append([]*Variable(nil), vars...)
	for i := 0; i < len(out); i++ {
		v := out[i]
		if v == nil || v.IsVirtual() {
			continue
		}
		// don't break up a struct if it matches the given type
		if v.Type != nil && v.Type.IsAggregate() && v.Type != want {
			// erase i, append field_vars at end (upstream insert end + i--)
			fields := v.FieldVars
			out = append(out[:i], out[i+1:]...)
			out = append(out, fields...)
			i--
		}
	}
	return out
}

// ChooseOKVarMatch mirrors choose_var type filter + choose_ok_var.
// VariableSelector.cpp choose_var — expand_struct_union_vars; Type::match(mt); optional skip const.
func ChooseOKVarMatch(r *Rng, vars []*Variable, want *Type, mt MatchType, skipConst bool) *Variable {
	if want == nil {
		return ChooseOKVar(r, vars)
	}
	// expand aggregates when want is simple or aggregate (choose_var:403–406)
	cands := vars
	if want.IsSimple() || want.IsAggregate() {
		cands = ExpandStructUnionVars(vars, want)
	}
	var ok []*Variable
	for _, x := range cands {
		if x == nil || x.Type == nil {
			continue
		}
		if skipConst && x.IsConst() {
			continue
		}
		if want.Match(x.Type, mt) {
			ok = append(ok, x)
		}
	}
	return ChooseOKVar(r, ok)
}

func typesMatchExact(a, b *Type) bool {
	// Type::match eExact is pointer identity (cached simple/pointer types).
	if a == nil || b == nil {
		return a == b
	}
	return a.Match(b, MatchExact)
}

// createAndInitialize mirrors VariableSelector::create_and_initialize.
// VariableSelector.cpp:518+ — NewArrayVariableProb flip → CreateArrayVariable.
func (vs *VariableSelector) createAndInitialize(
	access Access,
	cg CGContext,
	t *Type,
	qfer CVQualifiers,
	blk *Block,
	name string,
	r *Rng,
) *Variable {
	if vs == nil || t == nil || r == nil {
		return nil
	}
	// rnd_flipcoin(NewArrayVariableProb()) when arrays enabled
	if vs.Opts.Arrays && r.RndFlipcoin(uint32(vs.Probs.Single(PNewArrayVariableProb))) {
		init := MakeRandom(t, vs.Opts, r)
		av := CreateArrayVariable(r, vs.Opts, blk, name, t, init, qfer)
		if av != nil {
			vs.AllVars = append(vs.AllVars, &av.Variable)
			if blk != nil {
				blk.LocalVars = append(blk.LocalVars, &av.Variable)
			}
			// store full array on a side list for emission
			vs.Arrays = append(vs.Arrays, av)
			vs.VarCreated = true
			return &av.Variable
		}
	}
	v := CreateVariableQfer(name, t, qfer)
	if v == nil {
		return nil
	}
	v.Init = MakeRandom(t, vs.Opts, r)
	// VariableSelector.cpp:568–569 — access_once flip
	if vs.Opts.AccessOnce && vs.Probs != nil && r.RndFlipcoin(uint32(vs.Probs.Single(PAccessOnceVariableProb))) {
		v.IsAccessOnce = true
	}
	// wrap_volatiles → VOL_RVAL on Output
	if vs.Opts.WrapVolatiles && v.IsVolatile() {
		v.UseVolRVal = true
	}
	if blk != nil {
		blk.LocalVars = append(blk.LocalVars, v)
	}
	vs.AllVars = append(vs.AllVars, v)
	// FactMgr::add_new_var_fact_and_update_inout_maps when FM present
	if cg.FM != nil {
		cg.FM.AddNewVarFactAndUpdate(blk, v)
	}
	vs.VarCreated = true
	return v
}

// GenerateNewGlobal mirrors VariableSelector::GenerateNewGlobal for simple types:
// random_qualifiers (or copy qfer), RandomGlobalName, CreateVariable, push GlobalList.
// VariableSelector.cpp:546–575.
func (vs *VariableSelector) GenerateNewGlobal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	if vs == nil || t == nil {
		return nil
	}
	if !vs.Opts.GlobalVariables {
		return nil
	}
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		// CVQualifiers::random_qualifiers(t, access, cg, false)
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, false, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	name := vs.RandomGlobalName()
	vs.TmpCount++
	v := vs.createAndInitialize(access, cg, t, varQfer, nil, name, r)
	if v == nil {
		return nil
	}
	vs.GlobalList = append(vs.GlobalList, v)
	if !varQfer.IsVolatile() {
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, v)
	}
	return v
}

// SelectGlobal mirrors VariableSelector::SelectGlobal.
// VariableSelector.cpp:669–695 — choose_var; expand_struct eager create; else GenerateNewGlobal.
func (vs *VariableSelector) SelectGlobal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	if vs == nil {
		return nil
	}
	// choose_var with eFlexible + is_eligible_var
	v := ChooseVar(r, vs.GlobalList, access, cg, t, MatchFlexible)
	if v != nil {
		return v
	}
	// VariableSelector.cpp:677–684 — expand_struct eager path
	if vs.Opts.ExpandStruct {
		if v = vs.EagerCreateGlobalStruct(access, cg, t, qfer, r, MatchFlexible); v != nil {
			return v
		}
	}
	// SelectGlobal.cpp:685–694 — random_type_from_type then GenerateNewGlobal
	noVol := qfer != nil && !qfer.Wildcard && !qfer.IsVolatile()
	t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, noVol)
	if t2 == nil {
		t2 = t
	}
	return vs.GenerateNewGlobal(access, cg, t2, qfer, r)
}

// chooseRandomStructFromType mirrors Type::choose_random_struct_from_type.
// Type.cpp:570–586 — if ok structs exist pick one; else return original type.
func chooseRandomStructFromType(env *TypeEnv, typ *Type, noVolatile bool, r *Rng) *Type {
	if typ == nil || r == nil {
		return typ
	}
	cands := okStructUnionLTypes(env, noVolatile, true, true)
	if len(cands) == 0 {
		return typ
	}
	st := cands[r.RndUpto(uint32(len(cands)))]
	if st == nil {
		return typ
	}
	return st
}

// EagerCreateGlobalStruct mirrors VariableSelector::eager_create_global_struct.
// VariableSelector.cpp:607–633 — create a random ok struct global, then choose_var field.
func (vs *VariableSelector) EagerCreateGlobalStruct(
	access Access,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	if vs == nil || typ == nil || r == nil {
		return nil
	}
	level := typ.IndirectLevel()
	if level > 1 {
		return nil
	}
	st := chooseRandomStructFromType(vs.Types, typ, false, r)
	if st == nil {
		return nil
	}
	// level 0: create struct; level 1: still create struct (fields may match *T after expand)
	_ = vs.GenerateNewGlobal(access, cg, st, qfer, r)
	return ChooseVar(r, vs.GlobalList, access, cg, typ, mt)
}

// EagerCreateLocalStruct mirrors VariableSelector::eager_create_local_struct.
// VariableSelector.cpp:635–666 — create struct local on block, choose_var field.
func (vs *VariableSelector) EagerCreateLocalStruct(
	block *Block,
	access Access,
	cg CGContext,
	typ *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	if vs == nil || block == nil || typ == nil || r == nil {
		return nil
	}
	level := typ.IndirectLevel()
	if level > 1 {
		return nil
	}
	// choose_random_struct_from_type(type, true) — no_volatile for locals
	st := chooseRandomStructFromType(vs.Types, typ, true, r)
	if st == nil {
		return nil
	}
	_ = vs.GenerateNewParentLocal(block, access, cg, st, qfer, r)
	return ChooseVar(r, block.LocalVars, access, cg, typ, mt)
}

// GenerateParameterVariableTyped mirrors
// VariableSelector::GenerateParameterVariable(type, qfer).
// VariableSelector.cpp:955–957.
func (vs *VariableSelector) GenerateParameterVariableTyped(typ *Type, qfer CVQualifiers) *Variable {
	if vs == nil {
		return nil
	}
	name := vs.RandomParamName()
	v := CreateVariableQfer(name, typ, qfer)
	if v == nil {
		return nil
	}
	vs.AllVars = append(vs.AllVars, v)
	return v
}

// GenerateParameterVariable mirrors VariableSelector::GenerateParameterVariable(Function&).
// VariableSelector.cpp:963–979 — 40% pointer when derived exist; else nonvoid simple.
func (vs *VariableSelector) GenerateParameterVariable(f *Function, r *Rng) *Variable {
	if vs == nil || f == nil || r == nil {
		return nil
	}
	var t *Type
	// VariableSelector.cpp:966–972 — has_pointer_type() && flipcoin(40)
	if vs.Types != nil && vs.Types.HasPointerType() && r.RndFlipcoin(40) {
		// Type::choose_random_pointer_type → pick derived or make new
		t = vs.Types.MakeRandomPointerType(r, vs.Opts, vs.Probs)
	} else {
		// Type::choose_random_nonvoid_nonvolatile ≈ nonvoid simple under no structs
		st := ChooseRandomNonvoidSimple(r, vs.Probs)
		t = GetSimpleType(st)
	}
	// CVQualifiers::random_qualifiers(t) — READ empty no_volatile path
	qfer := RandomQualifiersNoContextNoVolatile(t, vs.Opts, vs.Probs, r)
	v := vs.GenerateParameterVariableTyped(t, qfer)
	if v != nil {
		f.Param = append(f.Param, v)
	}
	return v
}

// SelectLoopCtrlVar mirrors VariableSelector::SelectLoopCtrlVar.
// VariableSelector.cpp:1146–1179 — non-array visible, has_int_field, drop union+ptr; WRITE eConvert.
func (vs *VariableSelector) SelectLoopCtrlVar(r *Rng, cg CGContext, invalid map[*Variable]bool) *Variable {
	if vs == nil || r == nil {
		return nil
	}
	ty := GetIntType()
	blk := cg.CurrentBlock()
	vars := vs.FindAllNonArrayVisibleVars(blk)
	// remove no-int-field and union-with-pointer (VariableSelector.cpp:1155–1168)
	var filtered []*Variable
	for _, v := range vars {
		if v == nil || v.Type == nil || invalid[v] {
			continue
		}
		if !v.Type.HasIntField() {
			continue
		}
		if v.Type.IsUnion() && v.Type.ContainPointerField() {
			continue
		}
		filtered = append(filtered, v)
	}
	// choose_var WRITE eConvert; no_bitfield true (loop IV not bitfield)
	var ok []*Variable
	for _, x := range ExpandStructUnionVars(filtered, ty) {
		if x == nil || x.Type == nil || x.IsBitfield {
			continue
		}
		if !ty.Match(x.Type, MatchConvert) {
			continue
		}
		if !IsEligibleVar(x, x.Type.IndirectLevel()-ty.IndirectLevel(), AccessWrite, cg) {
			continue
		}
		ok = append(ok, x)
	}
	if v := ChooseOKVar(r, ok); v != nil {
		return v
	}
	if vs.Opts.GlobalVariables {
		return vs.GenerateNewGlobal(AccessWrite, cg, ty, nil, r)
	}
	if blk != nil {
		return vs.GenerateNewParentLocal(blk, AccessWrite, cg, ty, nil, r)
	}
	return nil
}

// GenerateNewParentLocal mirrors VariableSelector::GenerateNewParentLocal.
// VariableSelector.cpp:915–947 — local name, random_qualifiers no_volatile-ish, push block.local_vars.
func (vs *VariableSelector) GenerateNewParentLocal(
	block *Block,
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	if vs == nil || block == nil || t == nil || r == nil {
		return nil
	}
	var varQfer CVQualifiers
	if qfer == nil || qfer.Wildcard {
		// random_qualifiers(t, access, cg, true) — no_volatile true for locals path in some call sites;
		// GenerateNewParentLocal uses random_qualifiers(t, access, cg, true) — third bool is no_volatile.
		varQfer = RandomQualifiersDefaultProbs(t, access, cg, true, vs.Opts, vs.Probs, r)
	} else {
		varQfer = *qfer
	}
	// restrict(access, cg): WRITE clears const; non-SE-free clears vol
	if access == AccessWrite && len(varQfer.IsConsts) > 0 {
		varQfer.IsConsts[len(varQfer.IsConsts)-1] = false
	}
	if !cg.EffectContext().IsSideEffectFree() && len(varQfer.IsVolatiles) > 0 {
		varQfer.IsVolatiles[len(varQfer.IsVolatiles)-1] = false
	}
	name := vs.RandomLocalName()
	v := vs.createAndInitialize(access, cg, t, varQfer, block, name, r)
	return v
}

// SelectArray mirrors VariableSelector::select_array.
// VariableSelector.cpp:1384–1436 — visible non-itemized arrays; else create_random_array.
func (vs *VariableSelector) SelectArray(r *Rng, cg CGContext) *ArrayVariable {
	if vs == nil || r == nil {
		return nil
	}
	var arrayVars []*ArrayVariable
	// From tracked Arrays list (collectives only)
	for _, av := range vs.Arrays {
		if av == nil || av.Collective != nil {
			continue
		}
		if av.IsConst() {
			continue
		}
		if !cg.EffectContext().IsSideEffectFree() && av.IsVolatile() {
			continue
		}
		arrayVars = append(arrayVars, av)
	}
	n := len(arrayVars)
	if n == 0 {
		return vs.CreateRandomArray(r, cg)
	}
	if n == 1 {
		return arrayVars[0]
	}
	return arrayVars[r.RndUpto(uint32(n))]
}

// CreateRandomArray mirrors VariableSelector::create_random_array.
// VariableSelector.cpp:1340–1379 — choose_random_nonvoid (global) / nonvoid_nonvolatile (local).
func (vs *VariableSelector) CreateRandomArray(r *Rng, cg CGContext) *ArrayVariable {
	if vs == nil || r == nil {
		return nil
	}
	asGlobal := vs.Opts.GlobalVariables && r.RndFlipcoin(25)
	var name string
	var blk *Block
	if asGlobal {
		name = vs.RandomGlobalName()
	} else {
		name = vs.RandomLocalName()
		if cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
			idx := r.RndUpto(uint32(len(cg.CurrentFunc.Stack)))
			blk = cg.CurrentFunc.Stack[idx]
		}
	}
	// Type::choose_random_nonvoid / nonvoid_nonvolatile; skip const aggregates
	var elem *Type
	for tries := 0; tries < 16; tries++ {
		if asGlobal {
			if vs.Types != nil {
				elem = vs.Types.ChooseRandomNonvoid(r, vs.Opts, vs.Probs)
			} else {
				elem = GetSimpleType(ChooseRandomNonvoidSimple(r, vs.Probs))
			}
		} else {
			if vs.Types != nil {
				elem = vs.Types.ChooseRandomNonvoidNonvolatile(r, vs.Opts, vs.Probs)
			} else {
				elem = GetSimpleType(ChooseRandomNonvoidSimple(r, vs.Probs))
			}
		}
		if elem == nil {
			continue
		}
		if elem.IsConstStructUnion() {
			continue
		}
		break
	}
	if elem == nil {
		elem = GetIntType()
	}
	qfer := NewCVQualifiers([]bool{false}, []bool{false})
	init := MakeRandom(elem, vs.Opts, r)
	av := CreateArrayVariable(r, vs.Opts, blk, name, elem, init, qfer)
	if av == nil {
		return nil
	}
	vs.AllVars = append(vs.AllVars, &av.Variable)
	vs.Arrays = append(vs.Arrays, av)
	if asGlobal {
		vs.GlobalList = append(vs.GlobalList, &av.Variable)
		if !av.IsVolatile() {
			vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, &av.Variable)
		}
	} else if blk != nil {
		blk.LocalVars = append(blk.LocalVars, &av.Variable)
	}
	vs.VarCreated = true
	return av
}

// VariableScope mirrors eVariableScope.
type VariableScope int

const (
	ScopeGlobal VariableScope = iota
	ScopeParentLocal
	ScopeParentParam
	ScopeNewValue
	MaxVarScope
)

// NewScopeThresholdTable mirrors InitScopeTable / VariableSelectionProbability table.
// VariableSelector.cpp:112–121 — with globals: 35 Global, 65 Local, 95 Param, 100 New;
// without: 50 Local, 95 Param, 100 New.
func NewScopeThresholdTable(opts Options) *ThresholdTable {
	t := &ThresholdTable{}
	if opts.GlobalVariables {
		t.Add(35, int(ScopeGlobal))
		t.Add(65, int(ScopeParentLocal))
		t.Add(95, int(ScopeParentParam))
		t.Add(100, int(ScopeNewValue))
	} else {
		t.Add(50, int(ScopeParentLocal))
		t.Add(95, int(ScopeParentParam))
		t.Add(100, int(ScopeNewValue))
	}
	return t
}

// VariableSelectionProbability mirrors VariableSelectionProbability.
// VariableSelector.cpp:1043–1059 — rnd_upto(100); scopeTable get_value.
func VariableSelectionProbability(r *Rng, opts Options) VariableScope {
	if r == nil {
		return ScopeNewValue
	}
	tab := NewScopeThresholdTable(opts)
	v := r.RndUpto(100)
	sc := tab.GetValue(int(v))
	if sc < 0 {
		return ScopeNewValue
	}
	return VariableScope(sc)
}

// VariableCreationProbability mirrors VariableCreationProbability.
// VariableSelector.cpp:1063–1070 — flipcoin(10) global if allowed else local.
func VariableCreationProbability(r *Rng, opts Options) VariableScope {
	if opts.GlobalVariables && r != nil && r.RndFlipcoin(10) {
		return ScopeGlobal
	}
	return ScopeParentLocal
}

// Select mirrors VariableSelector::select.
// VariableSelector.cpp:1189–1243 — pick scope then SelectGlobal / ParentLocal / Param / New.
func (vs *VariableSelector) Select(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	if vs == nil || r == nil {
		return nil
	}
	vs.VarCreated = false
	scope := VariableSelectionProbability(r, vs.Opts)
	var v *Variable
	switch scope {
	case ScopeGlobal:
		v = vs.SelectGlobal(access, cg, t, qfer, r)
	case ScopeParentLocal:
		v = vs.SelectParentLocal(access, cg, t, qfer, r, mt)
	case ScopeParentParam:
		v = vs.SelectParentParam(access, cg, t, qfer, r, mt)
	case ScopeNewValue:
		v = vs.GenerateNewVariable(access, cg, t, qfer, r)
	default:
		v = vs.GenerateNewVariable(access, cg, t, qfer, r)
	}
	// if scope pick failed (e.g. no params), fall through to create
	if v == nil {
		v = vs.GenerateNewVariable(access, cg, t, qfer, r)
	}
	return v
}

// SelectParentLocal mirrors VariableSelector::SelectParentLocal.
// VariableSelector.cpp:987–1041 — rnd stack block; expand_struct on empty; choose_var or create.
func (vs *VariableSelector) SelectParentLocal(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	if vs == nil || cg.CurrentFunc == nil || r == nil {
		return nil
	}
	stack := cg.CurrentFunc.Stack
	if len(stack) == 0 {
		return nil
	}
	// VariableSelector.cpp:1001–1004 — pick one block on the stack
	blk := stack[r.RndUpto(uint32(len(stack)))]
	if blk == nil {
		return nil
	}
	// empty locals: expand_struct eager then GenerateNewParentLocal
	if len(blk.LocalVars) == 0 {
		if vs.Opts.ExpandStruct {
			if v := vs.EagerCreateLocalStruct(blk, access, cg, t, qfer, r, mt); v != nil {
				return v
			}
		}
		t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true)
		if t2 == nil {
			t2 = t
		}
		return vs.GenerateNewParentLocal(blk, access, cg, t2, qfer, r)
	}
	// VariableSelector.cpp:1019–1028 — simple nonvoid → match as int; else random_type_from_type
	matchT := t
	if t != nil && t.IsSimple() && t.Simple() != EVoid {
		matchT = GetIntType()
	} else {
		matchT = RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, true)
		if matchT == nil {
			matchT = t
		}
	}
	if v := ChooseVar(r, blk.LocalVars, access, cg, matchT, mt); v != nil {
		return v
	}
	return vs.GenerateNewParentLocal(blk, access, cg, matchT, qfer, r)
}

// SelectParentParam mirrors VariableSelector::SelectParentParam.
// VariableSelector.cpp:1074–1087 — choose param; empty/miss → SelectParentLocal.
func (vs *VariableSelector) SelectParentParam(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
	mt MatchType,
) *Variable {
	if cg.CurrentFunc == nil {
		return nil
	}
	if len(cg.CurrentFunc.Param) == 0 {
		return vs.SelectParentLocal(access, cg, t, qfer, r, mt)
	}
	if v := ChooseVar(r, cg.CurrentFunc.Param, access, cg, t, mt); v != nil {
		return v
	}
	return vs.SelectParentLocal(access, cg, t, qfer, r, mt)
}

// GenerateNewVariable mirrors VariableSelector::GenerateNewVariable.
// VariableSelector.cpp:1090+ — VariableCreationProbability → global or parent local.
func (vs *VariableSelector) GenerateNewVariable(
	access Access,
	cg CGContext,
	t *Type,
	qfer *CVQualifiers,
	r *Rng,
) *Variable {
	if vs == nil || t == nil {
		return nil
	}
	// random_type_from_type like SelectGlobal create path
	t2 := RandomTypeFromType(r, vs.Types, vs.Opts, vs.Probs, t, false)
	if t2 == nil {
		t2 = t
	}
	scope := VariableCreationProbability(r, vs.Opts)
	switch scope {
	case ScopeGlobal:
		return vs.GenerateNewGlobal(access, cg, t2, qfer, r)
	default:
		if cg.CurrentFunc != nil && len(cg.CurrentFunc.Stack) > 0 {
			blk := cg.CurrentFunc.Stack[len(cg.CurrentFunc.Stack)-1]
			return vs.GenerateNewParentLocal(blk, access, cg, t2, qfer, r)
		}
		return vs.GenerateNewGlobal(access, cg, t2, qfer, r)
	}
}
