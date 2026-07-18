// Upstream: VariableSelector.cpp / VariableSelector.h
// (GlobalList, choose_ok_var, GenerateNewGlobal name+qfer path, SelectGlobal empty→create).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// VariableSelector holds AllVars / GlobalList inventories (static vectors in C++).
type VariableSelector struct {
	AllVars                 []*Variable
	GlobalList              []*Variable
	GlobalNonvolatilesList  []*Variable
	Sym                     GenSym
	Opts                    Options
	Probs                   *Probabilities
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
// len==0 → nil; len==1 → sole; len>1 → rnd_upto(len).
// Array itemize deferred (no ArrayVariable yet).
// VariableSelector.cpp:318–337.
func ChooseOKVar(r *Rng, vars []*Variable) *Variable {
	n := len(vars)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return vars[0]
	}
	// DEPTH_GUARD omitted (no DepthSpec port).
	idx := r.RndUpto(uint32(n))
	return vars[idx]
}

// ChooseOKVarExactType filters vars whose Type is the same simple/pointer pointer chain
// as want (pointer equality of *Type is not required — match simple kind + indirect level).
// Full choose_var / Type::match deferred; this is a minimal exact filter for tests.
func ChooseOKVarExactType(r *Rng, vars []*Variable, want *Type) *Variable {
	if want == nil {
		return ChooseOKVar(r, vars)
	}
	var ok []*Variable
	for _, v := range vars {
		if v != nil && typesMatchExact(v.Type, want) {
			ok = append(ok, v)
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
	if blk != nil {
		blk.LocalVars = append(blk.LocalVars, v)
	}
	vs.AllVars = append(vs.AllVars, v)
	vs.VarCreated = true
	return v
}

// GenerateNewGlobal mirrors VariableSelector::GenerateNewGlobal for simple types:
// random_qualifiers (or copy qfer), RandomGlobalName, CreateVariable, push GlobalList.
// create_and_initialize / FactMgr / access_once deferred.
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

// SelectGlobal mirrors VariableSelector::SelectGlobal simplified:
// choose among GlobalList by exact type; if empty/miss → GenerateNewGlobal
// (skips expand_struct / random_type_from_type retype for now — uses t as given).
// VariableSelector.cpp:669–695.
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
	// choose_var(GlobalList, …) simplified → exact type match + choose_ok_var
	v := ChooseOKVarExactType(r, vs.GlobalList, t)
	if v != nil {
		return v
	}
	// Empty / no match → GenerateNewGlobal (expand_struct skipped when ExpandStruct false).
	return vs.GenerateNewGlobal(access, cg, t, qfer, r)
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

// SelectLoopCtrlVar mirrors VariableSelector::SelectLoopCtrlVar (simplified).
// VariableSelector.cpp:1146–1179 — non-array visible ints, WRITE, eConvert; else new global.
func (vs *VariableSelector) SelectLoopCtrlVar(r *Rng, cg CGContext, invalid map[*Variable]bool) *Variable {
	if vs == nil || r == nil {
		return nil
	}
	ty := GetIntType()
	var cands []*Variable
	// Globals (find_all_non_array_visible without full block chain)
	for _, v := range vs.GlobalList {
		if v == nil || invalid[v] || v.IsArray || v.IsVolatile() {
			continue
		}
		if v.Type != nil && v.Type.IsSimple() && ty.Match(v.Type, MatchConvert) {
			cands = append(cands, v)
		}
	}
	// Locals on function stack
	if cg.CurrentFunc != nil {
		for _, blk := range cg.CurrentFunc.Stack {
			if blk == nil {
				continue
			}
			for _, v := range blk.LocalVars {
				if v == nil || invalid[v] || v.IsArray || v.IsVolatile() {
					continue
				}
				if v.Type != nil && v.Type.IsSimple() && ty.Match(v.Type, MatchConvert) {
					cands = append(cands, v)
				}
			}
		}
		// params
		for _, v := range cg.CurrentFunc.Param {
			if v == nil || invalid[v] || v.IsVolatile() {
				continue
			}
			if v.Type != nil && v.Type.IsSimple() && ty.Match(v.Type, MatchConvert) {
				cands = append(cands, v)
			}
		}
	}
	if v := ChooseOKVar(r, cands); v != nil {
		return v
	}
	if vs.Opts.GlobalVariables {
		return vs.GenerateNewGlobal(AccessWrite, cg, ty, nil, r)
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

// CreateRandomArray mirrors VariableSelector::create_random_array (simplified).
// VariableSelector.cpp:1347–1379.
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
	// type: nonvoid simple
	st := ChooseRandomNonvoidSimple(r, vs.Probs)
	elem := GetSimpleType(st)
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
