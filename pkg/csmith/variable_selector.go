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
	// create_and_initialize → CreateVariable with init; init nil until Constant.
	v := CreateVariableQfer(name, t, varQfer)
	if v == nil {
		return nil
	}
	// create_and_initialize → Constant::make_random for simple non-union (Variable.cpp CreateVariable path)
	v.Init = MakeRandom(t, vs.Opts, r)
	vs.GlobalList = append(vs.GlobalList, v)
	vs.AllVars = append(vs.AllVars, v)
	if !varQfer.IsVolatile() {
		vs.GlobalNonvolatilesList = append(vs.GlobalNonvolatilesList, v)
	}
	vs.VarCreated = true
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
	// has_pointer_type() && flipcoin(40) — no derived_types yet → always simple
	if false && r.RndFlipcoin(40) {
		// Type::choose_random_pointer_type deferred
	} else {
		// Type::choose_random_nonvoid_nonvolatile ≈ choose_random_nonvoid_simple under no structs
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
