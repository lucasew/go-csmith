// Upstream: Type.cpp SelectLType, make_random_pointer_type, find_pointer_type.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// TypeEnv holds derived_types / session type lists (Type.cpp statics).
type TypeEnv struct {
	// DerivedTypes mirrors Type::derived_types (pointer types created so far).
	DerivedTypes []*Type
	// AllTypes mirrors Type::AllTypes (simples + aggregates available for fields).
	AllTypes []*Type
	// StructTypes are created struct types (subset of AllTypes).
	StructTypes []*Type
	// UnionTypes are created union types.
	UnionTypes []*Type
}

// FindPointerType mirrors Type::find_pointer_type(t, add).
// Type.cpp:423+ — return existing pointer-to-t or create if add.
func (env *TypeEnv) FindPointerType(t *Type, add bool) *Type {
	if t == nil {
		return nil
	}
	// PointerTo already caches by pointee identity.
	p := PointerTo(t)
	if env != nil && add {
		found := false
		for _, d := range env.DerivedTypes {
			if d == p {
				found = true
				break
			}
		}
		if !found {
			env.DerivedTypes = append(env.DerivedTypes, p)
		}
	}
	return p
}

// HasPointerType mirrors Type::has_pointer_type.
func (env *TypeEnv) HasPointerType() bool {
	return env != nil && len(env.DerivedTypes) > 0
}

// FindType mirrors Type::find_type — pointer identity in AllTypes.
// Type.cpp:410–417.
func (env *TypeEnv) FindType(t *Type) *Type {
	if env == nil || t == nil {
		return nil
	}
	for _, x := range env.AllTypes {
		if x == t {
			return x
		}
	}
	return nil
}

// GetAllOKStructUnionTypes mirrors Type::get_all_ok_struct_union_types.
// Type.cpp:487–503 — filter const/volatile aggregates and optional int field.
func (env *TypeEnv) GetAllOKStructUnionTypes(noConst, noVolatile, needIntField, wantStruct bool) []*Type {
	if env == nil {
		return nil
	}
	var ok []*Type
	for _, t := range env.AllTypes {
		if t == nil {
			continue
		}
		if wantStruct {
			if !t.IsStruct() {
				continue
			}
		} else {
			if !t.IsUnion() {
				continue
			}
		}
		if noConst && t.IsConstStructUnion() {
			continue
		}
		if noVolatile && t.IsVolatileStructUnion() {
			continue
		}
		if needIntField && !t.HasIntField() {
			continue
		}
		ok = append(ok, t)
	}
	return ok
}

// ChooseRandomStructUnionType mirrors Type::choose_random_struct_union_type.
// Type.cpp:521–533 — rnd_upto(ok_types).
func ChooseRandomStructUnionType(r *Rng, ok []*Type) *Type {
	if r == nil || len(ok) == 0 {
		return nil
	}
	return ok[r.RndUpto(uint32(len(ok)))]
}

// ChooseRandomStructFromType mirrors Type::choose_random_struct_from_type.
// Type.cpp:570–586 — if type is struct return it; else random from env.
func (env *TypeEnv) ChooseRandomStructFromType(r *Rng, typ *Type, noVolatile bool) *Type {
	if typ != nil && typ.IsStruct() {
		if noVolatile && typ.IsVolatileStructUnion() {
			// fall through to pick another
		} else {
			return typ
		}
	}
	ok := env.GetAllOKStructUnionTypes(false, noVolatile, false, true)
	// Type.cpp:581 — DEPTH_GUARD_BY_DEPTH_RETURN(1, nullptr) when candidates exist
	if len(ok) > 0 {
		if DepthGuardByDepth(Defaults(), 1) == BadDepth {
			return nil
		}
	}
	return ChooseRandomStructUnionType(r, ok)
}

// ChooseRandomPointerType mirrors Type::choose_random_pointer_type.
// Type.cpp:536–539 — rnd_upto(derived_types.size()).
func (env *TypeEnv) ChooseRandomPointerType(r *Rng) *Type {
	if env == nil || len(env.DerivedTypes) == 0 || r == nil {
		return nil
	}
	return env.DerivedTypes[r.RndUpto(uint32(len(env.DerivedTypes)))]
}

// ChooseRandom mirrors Type::choose_random via ChooseRandomTypeFilter.
// Type.cpp:1206–1216 / ChooseRandomTypeFilter::filter (Type.cpp:223–244).
// forFieldVar=false for return types.
func (env *TypeEnv) ChooseRandom(r *Rng, opts Options, probs *Probabilities, forFieldVar bool) *Type {
	if env == nil || len(env.AllTypes) == 0 {
		st := ChooseRandomNonvoidSimple(r, probs)
		return GetSimpleType(st)
	}
	if r == nil {
		return env.AllTypes[0]
	}
	// rnd_upto(AllTypes.size(), filter) — reject void-like simple weights 0, !return_structs
	filt := filterFunc(func(v uint32) bool {
		i := int(v)
		if i < 0 || i >= len(env.AllTypes) {
			return true
		}
		t := env.AllTypes[i]
		if t == nil {
			return true
		}
		if t.IsSimple() {
			// SIMPLE_TYPES_PROB_FILTER
			return probs != nil && probs.SimpleTypeWeight(int(t.Simple())) == 0
		}
		if t.IsStruct() && !opts.ReturnStructs {
			return true
		}
		if t.IsUnion() && !opts.ReturnUnions {
			return true
		}
		if forFieldVar && t.IsStruct() {
			// Type.cpp:240–242 — reject when depth >= max_nested_struct_level
			if t.StructDepth() >= opts.MaxNestedStructLevel {
				return true
			}
		}
		return false
	})
	idx := r.RndUptoFilter(uint32(len(env.AllTypes)), filt)
	t := env.AllTypes[idx]
	if t != nil {
		t.Used = true
	}
	return t
}

// RandomTypeFromType mirrors Type::random_type_from_type.
// Type.cpp:589–606 — nil → choose nonvoid; simple → re-roll choose_random_simple.
func RandomTypeFromType(
	r *Rng,
	env *TypeEnv,
	opts Options,
	probs *Probabilities,
	typ *Type,
	noVolatile bool,
) *Type {
	// Type.cpp:592 — DEPTH_GUARD_BY_TYPE_RETURN(dtRandomTypeFromType, nullptr)
	if DepthGuardByType(opts, DtRandomTypeFromType) == BadDepth {
		return nil
	}
	if typ == nil {
		if env != nil && len(env.AllTypes) > 0 {
			// Type.cpp:594–595 — no_volatile → nonvoid_nonvolatile
			if noVolatile {
				return env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
			}
			return env.ChooseRandomNonvoid(r, opts, probs)
		}
		return GetSimpleType(ChooseRandomNonvoidSimple(r, probs))
	}
	// simple + !strict_simple_type → choose_random_simple (always for our callers)
	// Type.cpp:1242 — DEPTH_GUARD_BY_TYPE_RETURN(dtTypeChooseSimple, nullptr)
	if typ.IsSimple() {
		if DepthGuardByType(opts, DtTypeChooseSimple) == BadDepth {
			return nil
		}
		st := ChooseRandomNonvoidSimple(r, probs)
		return GetSimpleType(st)
	}
	return typ
}

// ChooseRandomNonvoid mirrors Type::choose_random_nonvoid.
// Type.cpp:1218–1227 — NonVoidTypeFilter rejects void.
func (env *TypeEnv) ChooseRandomNonvoid(r *Rng, opts Options, probs *Probabilities) *Type {
	// Type.cpp:1219 — DEPTH_GUARD_BY_DEPTH_RETURN(1, nullptr)
	if DepthGuardByDepth(opts, 1) == BadDepth {
		return nil
	}
	return env.chooseRandomFiltered(r, opts, probs, false)
}

// ChooseRandomNonvoidNonvolatile mirrors Type::choose_random_nonvoid_nonvolatile.
// Type.cpp:1229+ / NonVoidNonVolatileTypeFilter — also reject volatile aggregates.
func (env *TypeEnv) ChooseRandomNonvoidNonvolatile(r *Rng, opts Options, probs *Probabilities) *Type {
	// Type.cpp:1230 — DEPTH_GUARD_BY_DEPTH_RETURN(1, nullptr)
	if DepthGuardByDepth(opts, 1) == BadDepth {
		return nil
	}
	return env.chooseRandomFiltered(r, opts, probs, true)
}

// chooseRandomFiltered shared filter for nonvoid (+ optional nonvolatile aggregate).
func (env *TypeEnv) chooseRandomFiltered(r *Rng, opts Options, probs *Probabilities, noVolatileAgg bool) *Type {
	if env == nil || len(env.AllTypes) == 0 {
		st := ChooseRandomNonvoidSimple(r, probs)
		return GetSimpleType(st)
	}
	filt := filterFunc(func(v uint32) bool {
		i := int(v)
		if i < 0 || i >= len(env.AllTypes) {
			return true
		}
		t := env.AllTypes[i]
		if t == nil {
			return true
		}
		if t.IsSimple() && t.Simple() == EVoid {
			return true
		}
		if t.IsSimple() && probs != nil && probs.SimpleTypeWeight(int(t.Simple())) == 0 {
			return true
		}
		if noVolatileAgg && t.IsAggregate() && t.IsVolatileStructUnion() {
			return true
		}
		// arg_structs / arg_unions gates (used for local array element types)
		if t.IsStruct() && !opts.ArgStructs {
			return true
		}
		if t.IsUnion() && !opts.ArgUnions {
			return true
		}
		return false
	})
	idx := r.RndUptoFilter(uint32(len(env.AllTypes)), filt)
	t := env.AllTypes[idx]
	if t != nil {
		t.Used = true
	}
	return t
}

// MakeRandomPointerType mirrors Type::make_random_pointer_type.
// Type.cpp:1141–1166.
func (env *TypeEnv) MakeRandomPointerType(r *Rng, opts Options, probs *Probabilities) *Type {
	if r == nil {
		return PointerTo(GetIntType())
	}
	// occasionally choose pointer to pointers (20%)
	if r.RndFlipcoin(20) && env != nil && len(env.DerivedTypes) > 0 {
		idx := r.RndUpto(uint32(len(env.DerivedTypes)))
		t := env.DerivedTypes[idx]
		if t != nil && t.IndirectLevel() < opts.MaxPointerDepth {
			return env.FindPointerType(t, true)
		}
	}
	// choose_random then consolidate integer pointers to int* (Type.cpp:1158–1166)
	base := env.ChooseRandom(r, opts, probs, false)
	if base == nil {
		base = GetIntType()
	}
	// consolidate all integer pointer types into "int*"
	if base.IsSimple() && base.Simple() != EVoid && base.Simple() != EFloat {
		base = GetIntType()
	}
	return env.FindPointerType(base, true)
}

// AssignOpWorksForFloat mirrors StatementAssign::AssignOpWorksForFloat.
// Typically simple assign and some arith; bit ops and incr often false.
func AssignOpWorksForFloat(op AssignOp) bool {
	switch op {
	case AssignSimple, AssignAdd, AssignSub, AssignMul, AssignDiv:
		return true
	default:
		return false
	}
}

// SelectLType mirrors Type::SelectLType.
// Type.cpp:1603–1637.
func SelectLType(r *Rng, opts Options, probs *Probabilities, env *TypeEnv, noVolatile bool, op AssignOp) *Type {
	if r == nil {
		return GetIntType()
	}
	// pointer as LType (simple assign only)
	if op == AssignSimple && r.RndFlipcoin(uint32(probs.Single(PPointerAsLTypeProb))) {
		if env != nil {
			return env.MakeRandomPointerType(r, opts, probs)
		}
		return PointerTo(GetIntType())
	}
	// struct/union as LType — get_all_ok_struct_union_types filtered (Type.cpp:1616–1622)
	if op == AssignSimple && env != nil {
		if len(env.StructTypes) > 0 && r.RndFlipcoin(uint32(probs.Single(PStructAsLTypeProb))) {
			cands := okStructUnionLTypes(env, noVolatile, true, false)
			if len(cands) > 0 {
				return cands[r.RndUpto(uint32(len(cands)))]
			}
		}
		if len(env.UnionTypes) > 0 && r.RndFlipcoin(uint32(probs.Single(PUnionAsLTypeProb))) {
			cands := okStructUnionLTypes(env, noVolatile, false, true)
			if len(cands) > 0 {
				return cands[r.RndUpto(uint32(len(cands)))]
			}
		}
	}

	// float as LType
	if AssignOpWorksForFloat(op) && r.RndFlipcoin(uint32(probs.Single(PFloatAsLTypeProb))) {
		if opts.EnableFloat {
			return GetSimpleType(EFloat)
		}
	}
	// default is any integer type → get_int_type()
	return GetIntType()
}

// okStructUnionLTypes filters struct/union types for SelectLType (no_volatile etc.).
// Type.cpp get_all_ok_struct_union_types subset.
func okStructUnionLTypes(env *TypeEnv, noVolatile, wantStruct, wantUnion bool) []*Type {
	if env == nil {
		return nil
	}
	var out []*Type
	if wantStruct {
		for _, t := range env.StructTypes {
			if t == nil {
				continue
			}
			if noVolatile && t.IsVolatileStructUnion() {
				continue
			}
			out = append(out, t)
		}
	}
	if wantUnion {
		for _, t := range env.UnionTypes {
			if t == nil {
				continue
			}
			if noVolatile && t.IsVolatileStructUnion() {
				continue
			}
			out = append(out, t)
		}
	}
	return out
}
