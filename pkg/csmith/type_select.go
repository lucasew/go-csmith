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
	// AggregateSeq mirrors Type.cpp Type(struct/union) static sequence for sid.
	// Shared across structs and unions: first aggregate S0 or U0, next S1/U1, …
	// Type.cpp:298–302; Output uses "struct S"<<sid / "union U"<<sid.
	AggregateSeq int
}

// FindPointerType mirrors Type::find_pointer_type(t, add).
// Type.cpp:423+ — return existing pointer-to-t or create if add.
// Incomplete DerivedTypes fails closed nil when add (no invent soft-skip hole
// then match/append as if the pool were complete).
func (env *TypeEnv) FindPointerType(t *Type, add bool) *Type {
	// Type* pointee always live; sticky no invent pointer-to-nil / soft-skip hole
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// PointerTo already caches by pointee identity.
	p := PointerTo(t)
	if env != nil && add {
		// Type* always live on derived_types; hole → fail closed sticky (no invent soft-skip hole)
		if !typesComplete(env.DerivedTypes) {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
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
// Incomplete DerivedTypes fails closed sticky false (no invent "no pointers" /
// soft re-pick scalar paths past DerivedTypes holes via typesComplete false).
func (env *TypeEnv) HasPointerType() bool {
	// nil TypeEnv is complete empty pool (no pointers) — not incomplete IR
	if env == nil {
		return false
	}
	if !typesComplete(env.DerivedTypes) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	return len(env.DerivedTypes) > 0
}

// FindType mirrors Type::find_type — pointer identity in AllTypes.
// Type.cpp:410–417.
// Type* always live on AllTypes; nil hole fails closed (nil — no invent soft-skip
// hole and still match a later entry, or invent "not found" past incomplete pool).
func (env *TypeEnv) FindType(t *Type) *Type {
	// nil TypeEnv is complete empty pool — soft not-found (not incomplete IR)
	if env == nil {
		return nil
	}
	// Type* always live as query subject; sticky no invent soft-skip nil key
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !typesComplete(env.AllTypes) {
		sessNoteError(nil, ErrGeneric)
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
// Type* always live on AllTypes; nil hole fails closed IncompleteTypes
// (not bare nil invent empty-complete ok_types pool via typesComplete(nil)/len==0).
// Complete empty filter result returns non-nil empty slice.
func (env *TypeEnv) GetAllOKStructUnionTypes(noConst, noVolatile, needIntField, wantStruct bool) []*Type {
	if env == nil {
		return []*Type{}
	}
	if !typesComplete(env.AllTypes) {
		// incomplete AllTypes fails closed sticky (no invent soft re-pick empty/partial ok pool)
		sessNoteError(nil, ErrGeneric)
		return IncompleteTypes()
	}
	ok := make([]*Type, 0)
	for _, t := range env.AllTypes {
		if wantStruct {
			if !t.IsStruct() {
				// residual ERROR sticky — no invent soft-skip then pick later past IsStruct residual
				if sessHasError(nil) {
					return IncompleteTypes()
				}
				continue
			}
		} else {
			if !t.IsUnion() {
				// residual ERROR sticky — no invent soft-skip then pick later past IsUnion residual
				if sessHasError(nil) {
					return IncompleteTypes()
				}
				continue
			}
		}
		// residual ERROR sticky — no invent soft-continue keep past IsStruct/IsUnion residual true
		if sessHasError(nil) {
			return IncompleteTypes()
		}
		if noConst && t.IsConstStructUnion() {
			// residual ERROR sticky — no invent soft-skip then pick later past field-Type hole
			if sessHasError(nil) {
				return IncompleteTypes()
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue keep past IsConstStructUnion residual false
		if sessHasError(nil) {
			return IncompleteTypes()
		}
		if noVolatile && t.IsVolatileStructUnion() {
			// residual ERROR sticky — no invent soft-skip then pick later past field-Type hole
			if sessHasError(nil) {
				return IncompleteTypes()
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue keep past IsVolatileStructUnion residual false
		if sessHasError(nil) {
			return IncompleteTypes()
		}
		if needIntField && !t.HasIntField() {
			// residual ERROR sticky — no invent soft-skip then pick later past HasIntField hole
			if sessHasError(nil) {
				return IncompleteTypes()
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue keep past HasIntField residual false
		if sessHasError(nil) {
			return IncompleteTypes()
		}
		ok = append(ok, t)
	}
	return ok
}

// typesComplete reports Type* slices have no nil holes (Type* always live on
// AllTypes / derived_types / ok_types in C++).
// Note: typesComplete(nil)==true (complete empty). Fail-closed incomplete
// wipes must use IncompleteTypes() so len(nil)==0 cannot invent empty-complete
// type-pool success.
func typesComplete(ts []*Type) bool {
	for _, t := range ts {
		if t == nil {
			return false
		}
	}
	return true
}

// IncompleteTypes is the fail-closed incomplete Type* list marker.
// typesComplete returns false. Distinct from complete empty (nil or {}).
func IncompleteTypes() []*Type {
	return []*Type{nil}
}

// ChooseRandomStructUnionType mirrors Type::choose_random_struct_union_type.
// Type.cpp:521–533 — rnd_upto(ok_types); ERROR_GUARD(0); mark used.
// Type* always live on ok_types; nil hole fails closed (nil — no invent pick past hole).
func ChooseRandomStructUnionType(r *Rng, ok []*Type) *Type {
	// Type.cpp always has process RNG; sticky no invent pick without it
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// Type.cpp:523 — assert(sz > 0); empty pool fail closed (non-sticky soft when no candidates)
	if len(ok) == 0 {
		return nil
	}
	if !typesComplete(ok) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	rv := ok[r.RndUpto(uint32(len(ok)))]
	// Type.cpp:526 — ERROR_GUARD(0)
	if sessHasError(nil) {
		return nil
	}
	// pre-validated complete
	if !rv.Used {
		// Type.cpp:528–531
		RecordTypeWithBitfields(rv)
		rv.Used = true
	}
	return rv
}

// ChooseRandomStructFromType mirrors Type::choose_random_struct_from_type.
// Type.cpp:570–586 — if type is struct return it; else random from env.
func (env *TypeEnv) ChooseRandomStructFromType(r *Rng, typ *Type, noVolatile bool) *Type {
	if typ != nil && typ.IsStruct() {
		// residual ERROR sticky — no invent soft-return typ past IsStruct residual hole
		if sessHasError(nil) {
			return nil
		}
		if noVolatile && typ.IsVolatileStructUnion() {
			// residual ERROR sticky — no invent soft-fallthrough past IsVolatileStructUnion residual
			if sessHasError(nil) {
				return nil
			}
			// fall through to pick another
		} else if sessHasError(nil) {
			// residual ERROR sticky — no invent soft-return typ past IsVolatileStructUnion residual false
			return nil
		} else {
			return typ
		}
	} else if sessHasError(nil) {
		// residual ERROR sticky — no invent soft-continue pool pick past IsStruct residual false
		return nil
	}
	ok := env.GetAllOKStructUnionTypes(false, noVolatile, false, true)
	// residual ERROR sticky — no invent soft-pick past GetAllOK residual hole
	if sessHasError(nil) {
		return nil
	}
	// incomplete AllTypes pool — fail closed sticky (no invent pick from partial)
	if !typesComplete(ok) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// Type.cpp:581 — DEPTH_GUARD_BY_DEPTH_RETURN(1, nullptr) when candidates exist
	// process CGOptions (dfs_exhaustive / max_exhaustive_depth); no Defaults invent
	if len(ok) > 0 {
		if DepthGuardByDepth(ProcessOptions(), 1) == BadDepth {
			return nil
		}
	}
	return ChooseRandomStructUnionType(r, ok)
}

// ChooseRandomPointerType mirrors Type::choose_random_pointer_type.
// Type.cpp:536–539 — rnd_upto(derived_types.size()); ERROR_GUARD(nullptr).
// Type* always live on derived_types; nil hole fails closed (nil — no invent pick past hole).
func (env *TypeEnv) ChooseRandomPointerType(r *Rng) *Type {
	// Type.cpp always has process RNG; sticky no invent pointer pick without it
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// empty derived: soft re-pick (no pointer types available)
	if env == nil || len(env.DerivedTypes) == 0 {
		return nil
	}
	if !typesComplete(env.DerivedTypes) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	p := env.DerivedTypes[r.RndUpto(uint32(len(env.DerivedTypes)))]
	// Type.cpp:538 — ERROR_GUARD(nullptr)
	if sessHasError(nil) {
		return nil
	}
	return p
}

// ChooseRandom mirrors Type::choose_random via ChooseRandomTypeFilter.
// Type.cpp:1181–1191 / ChooseRandomTypeFilter::filter (Type.cpp:223–244).
// forFieldVar=false for return types; marks used + bitfields bookkeeping.
// Type* always live on AllTypes; nil hole fails closed (nil — no invent filter-out
// hole as absent and still pick from remaining types).
func (env *TypeEnv) ChooseRandom(r *Rng, opts Options, probs *Probabilities, forFieldVar bool) *Type {
	return env.chooseRandomTypeFilter(r, opts, probs, forFieldVar, true)
}

// chooseRandomForStructField mirrors Type::make_one_struct_field type pick:
// Type.cpp:658–666 — rnd_upto(AllTypes, ChooseRandomTypeFilter for_field_var=true)
// without marking used (only Type::choose_random sets used).
func (env *TypeEnv) chooseRandomForStructField(r *Rng, opts Options, probs *Probabilities) *Type {
	return env.chooseRandomTypeFilter(r, opts, probs, true, false)
}

// chooseRandomTypeFilter is ChooseRandomTypeFilter + rnd_upto (Type.cpp:223–244, 1181–1191).
// markUsed mirrors Type::choose_random (true) vs make_one_struct_field (false).
func (env *TypeEnv) chooseRandomTypeFilter(r *Rng, opts Options, probs *Probabilities, forFieldVar, markUsed bool) *Type {
	if r == nil {
		// Type.cpp always has RNG; sticky no invent AllTypes[0]
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if env == nil || len(env.AllTypes) == 0 {
		// Type.cpp:1208–1209 — ERROR_GUARD(nullptr); non-sticky soft when env not seeded
		return nil
	}
	if !typesComplete(env.AllTypes) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// Type.cpp:1206–1216 — rnd_upto(AllTypes.size(), ChooseRandomTypeFilter)
	filt := filterFunc(func(v uint32) bool {
		i := int(v)
		if i < 0 || i >= len(env.AllTypes) {
			return true
		}
		t := env.AllTypes[i]
		// pre-validated complete
		if t.IsSimple() {
			// residual ERROR sticky — no invent soft-filter past IsSimple residual true
			if sessHasError(nil) {
				return true
			}
			// SIMPLE_TYPES_PROB_FILTER (Type.cpp:226–228)
			return probs != nil && probs.SimpleTypeWeight(int(t.Simple())) == 0
		}
		// residual ERROR sticky — no invent soft-continue filter past IsSimple residual false
		if sessHasError(nil) {
			return true
		}
		// Type.cpp:229–231 — !return_structs rejects structs only (not unions)
		if t.IsStruct() && !opts.ReturnStructs {
			// residual ERROR sticky — no invent soft-reject/keep past IsStruct residual
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent soft-continue filter past IsStruct residual false
		if sessHasError(nil) {
			return true
		}
		// Type.cpp ChooseRandomTypeFilter has no return_unions gate (unlike arg_unions on NonVoidNonVolatile)
		if forFieldVar && t.IsStruct() {
			// residual ERROR sticky — no invent soft-continue depth filter past IsStruct residual
			if sessHasError(nil) {
				return true
			}
			// Type.cpp:240–242 — reject when depth >= max_nested_struct_level
			d := t.StructDepth()
			// residual ERROR sticky — no invent soft-continue filter past StructDepth hole
			// (RndUptoFilter also fails closed on residual; reject candidate immediately)
			if sessHasError(nil) {
				return true
			}
			if d >= opts.MaxNestedStructLevel {
				return true
			}
		} else if sessHasError(nil) {
			// residual ERROR sticky — no invent soft-keep past IsStruct residual false
			return true
		}
		return false
	})
	idx := r.RndUptoFilter(uint32(len(env.AllTypes)), filt)
	// Type.cpp:1208–1209 — ERROR_GUARD(nullptr) after rnd_upto
	if sessHasError(nil) {
		return nil
	}
	t := env.AllTypes[idx]
	// Type.cpp:1186–1190 choose_random only — make_one_struct_field does not mark used
	if markUsed && !t.Used {
		RecordTypeWithBitfields(t)
		t.Used = true
	}
	return t
}

// RandomTypeFromType mirrors Type::random_type_from_type.
// Type.cpp:589–606 — nil → choose nonvoid; simple && !strict_simple_type → re-roll.
// Defaults in Type.h: no_volatile=false, strict_simple_type=false.
func RandomTypeFromType(
	r *Rng,
	env *TypeEnv,
	opts Options,
	probs *Probabilities,
	typ *Type,
	noVolatile bool,
	strictSimple bool,
) *Type {
	// Type.cpp:592 — DEPTH_GUARD_BY_TYPE_RETURN(dtRandomTypeFromType, nullptr)
	if DepthGuardByType(opts, DtRandomTypeFromType) == BadDepth {
		return nil
	}
	// Type.cpp always has process RNG sticky; no invent keep-simple / pick-nonvoid without RNG
	if typ == nil {
		if r == nil {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		// Type.cpp:594–597 — choose_random_nonvoid(_nonvolatile) + ERROR_GUARD; no soft invent simple
		// TypeEnv + AllTypes always live after GenerateAllTypes; sticky nil (no invent empty pick)
		if env == nil || len(env.AllTypes) == 0 {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		if noVolatile {
			return env.ChooseRandomNonvoidNonvolatile(r, opts, probs)
		}
		return env.ChooseRandomNonvoid(r, opts, probs)
	}
	// Type.cpp:599–601 — eSimple && !strict_simple_type → choose_random_simple
	// no soft invent re-roll when strict_simple_type (make_init_value pointer create)
	simple := typ.IsSimple()
	// residual ERROR sticky — no invent soft-type past IsSimple residual
	if sessHasError(nil) {
		return nil
	}
	if simple && !strictSimple {
		if r == nil {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		// Type.cpp:1242 — DEPTH_GUARD_BY_TYPE_RETURN(dtTypeChooseSimple, nullptr)
		if DepthGuardByType(opts, DtTypeChooseSimple) == BadDepth {
			return nil
		}
		st := ChooseRandomNonvoidSimple(r, probs)
		// Type.cpp:603–605 — assert(simple != eVoid) sticky; no soft invent int for void
		if st == EVoid {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return nil
		}
		return GetSimpleType(st)
	}
	// Type.cpp:602–605 — strict simple or non-simple: keep t (assert non-void simple) sticky
	if simple && typ.Simple() == EVoid {
		sessNoteError(nil, ErrGeneric)
		return nil
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
// Type* always live on AllTypes; nil hole fails closed (nil — no invent filter-out
// hole as absent and still pick from remaining types).
func (env *TypeEnv) chooseRandomFiltered(r *Rng, opts Options, probs *Probabilities, noVolatileAgg bool) *Type {
	// Type.cpp:1218+ — rnd_upto(AllTypes); sticky no invent simple without RNG
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// empty env non-sticky soft re-pick (unit paths without GenerateSimpleTypes)
	if env == nil || len(env.AllTypes) == 0 {
		return nil
	}
	if !typesComplete(env.AllTypes) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	filt := filterFunc(func(v uint32) bool {
		i := int(v)
		if i < 0 || i >= len(env.AllTypes) {
			return true
		}
		t := env.AllTypes[i]
		// pre-validated complete
		simple := t.IsSimple()
		// residual ERROR sticky — no invent soft-filter past IsSimple residual
		if sessHasError(nil) {
			return true
		}
		if simple && t.Simple() == EVoid {
			return true
		}
		if simple && probs != nil && probs.SimpleTypeWeight(int(t.Simple())) == 0 {
			return true
		}
		if noVolatileAgg && t.IsAggregate() && t.IsVolatileStructUnion() {
			// residual ERROR sticky — no invent soft-skip then pick later past IsVolatileStructUnion hole
			if sessHasError(nil) {
				return true
			}
			return true
		}
		// residual ERROR sticky — no invent keep candidate past IsVolatileStructUnion residual false
		if sessHasError(nil) {
			return true
		}
		// arg_structs / arg_unions: NonVoidNonVolatileTypeFilter only (Type.cpp:171–177)
		if noVolatileAgg {
			if t.IsStruct() && !opts.ArgStructs {
				if sessHasError(nil) {
					return true
				}
				return true
			}
			if sessHasError(nil) {
				return true
			}
			if t.IsUnion() && !opts.ArgUnions {
				if sessHasError(nil) {
					return true
				}
				return true
			}
			if sessHasError(nil) {
				return true
			}
		}
		return false
	})
	idx := r.RndUptoFilter(uint32(len(env.AllTypes)), filt)
	// Type.cpp:1221–1224 — ERROR_GUARD(nullptr); assert(typ)
	if sessHasError(nil) {
		return nil
	}
	t := env.AllTypes[idx]
	if !t.Used {
		RecordTypeWithBitfields(t)
		// residual ERROR sticky — no invent soft-mark used past RecordTypeWithBitfields residual
		if sessHasError(nil) {
			return nil
		}
		t.Used = true
	}
	return t
}

// MakeRandomPointerType mirrors Type::make_random_pointer_type.
// Type.cpp:1141–1166.
func (env *TypeEnv) MakeRandomPointerType(r *Rng, opts Options, probs *Probabilities) *Type {
	// Type.cpp always has process RNG; sticky no invent int* without it
	if r == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// nil env non-sticky soft re-pick
	if env == nil {
		return nil
	}
	// Type.cpp:1145–1154 — always rnd_flipcoin(20) first, then size check.
	// Do not invent short-circuit skip of the flip when derived_types is empty
	// (that desyncs the stream vs C++ which still draws).
	// Type* always live on derived_types; nil hole fails closed (no invent skip hole
	// and fall through to choose_random as if derived were empty).
	if r.RndFlipcoin(20) {
		// Type.cpp:1146 — ERROR_GUARD after flipcoin
		if sessHasError(nil) {
			return nil
		}
		if len(env.DerivedTypes) > 0 {
			if !typesComplete(env.DerivedTypes) {
				sessNoteError(nil, ErrGeneric)
				return nil
			}
			idx := r.RndUpto(uint32(len(env.DerivedTypes)))
			// Type.cpp:1149 ERROR_GUARD after rnd_upto
			if sessHasError(nil) {
				return nil
			}
			t := env.DerivedTypes[idx]
			if t.IndirectLevel() < opts.MaxPointerDepth {
				// residual ERROR sticky — no invent soft-return pointer past IndirectLevel residual
				if sessHasError(nil) {
					return nil
				}
				return env.FindPointerType(t, true)
			}
			// residual ERROR sticky — no invent soft-continue choose_random past IndirectLevel residual false
			if sessHasError(nil) {
				return nil
			}
		}
	}
	// Type.cpp:1158–1165 — choose_random then consolidate all simple → int*
	base := env.ChooseRandom(r, opts, probs, false)
	// residual ERROR sticky — no invent soft-empty pointer past ChooseRandom residual
	if sessHasError(nil) {
		return nil
	}
	if base == nil {
		// ERROR_GUARD(nullptr) — no soft invent GetIntType
		return nil
	}
	// Type.cpp:1161–1164 — any eSimple consolidates to get_int_type()
	if base.IsSimple() {
		// residual ERROR sticky — no invent soft-consolidate past IsSimple residual hole
		if sessHasError(nil) {
			return nil
		}
		base = GetIntType()
	} else if sessHasError(nil) {
		// residual ERROR sticky — no invent soft-continue past IsSimple residual false
		return nil
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
		// C++ always has RNG; sticky no soft invent default int
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	var typ *Type
	// Type.cpp:1609–1614 — pointer as LType (simple assign only); ERROR_GUARD after flip + make
	if op == AssignSimple && probs != nil && r.RndFlipcoin(uint32(probs.Single(PPointerAsLTypeProb))) {
		// Type.cpp:1610 — ERROR_GUARD(nullptr) after flipcoin
		if sessHasError(nil) {
			return nil
		}
		if env != nil {
			typ = env.MakeRandomPointerType(r, opts, probs)
		}
		// no soft invent PointerTo(int) when env missing
	}
	// Type.cpp:1613 — ERROR_GUARD(nullptr) always after pointer branch (before default int)
	// sticky error must not soft invent get_int_type()
	if sessHasError(nil) {
		return nil
	}
	// Type.cpp:1616–1625 — struct as LType only (bStruct=true); no union soft path
	if typ == nil && op == AssignSimple && env != nil && probs != nil {
		// Type.cpp:1617–1618 — get_all_ok_struct_union_types(ok, no_const=true, no_volatile, need_int=false, bStruct=true)
		cands := env.GetAllOKStructUnionTypes(true, noVolatile, false, true)
		// incomplete ok_types fail closed sticky (no invent fall-through get_int_type past hole pool)
		if !typesComplete(cands) {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		if len(cands) > 0 && r.RndFlipcoin(uint32(probs.Single(PStructAsLTypeProb))) {
			if sessHasError(nil) {
				return nil
			}
			typ = ChooseRandomStructUnionType(r, cands)
			// Type.cpp:526 ERROR_GUARD inside choose_random_struct_union_type
			if sessHasError(nil) {
				return nil
			}
		}
	}

	// Type.cpp:1628–1633 — float as LType
	if typ == nil && AssignOpWorksForFloat(op) && probs != nil &&
		r.RndFlipcoin(uint32(probs.Single(PFloatAsLTypeProb))) {
		if sessHasError(nil) {
			return nil
		}
		if opts.EnableFloat {
			typ = GetSimpleType(EFloat)
		}
	}
	// Type.cpp:1635–1637 — default get_int_type()
	if typ == nil {
		typ = GetIntType()
	}
	return typ
}

// okStructUnionLTypes filters struct/union types for SelectLType (no_volatile etc.).
// Type.cpp get_all_ok_struct_union_types subset.
// Type* always live; nil hole fails closed sticky IncompleteTypes (not bare nil invent
// empty-complete ok pool / soft re-pick SelectLType past env holes).
// Complete empty filter result returns non-nil empty slice.
func okStructUnionLTypes(env *TypeEnv, noVolatile, wantStruct, wantUnion bool) []*Type {
	if env == nil {
		return []*Type{}
	}
	out := make([]*Type, 0)
	if wantStruct {
		for _, t := range env.StructTypes {
			if t == nil {
				sessNoteError(nil, ErrGeneric)
				return IncompleteTypes()
			}
			if noVolatile && t.IsVolatileStructUnion() {
				// residual ERROR sticky — no invent soft-skip then pick later past field-Type hole
				if sessHasError(nil) {
					return IncompleteTypes()
				}
				continue
			}
			out = append(out, t)
		}
	}
	if wantUnion {
		for _, t := range env.UnionTypes {
			if t == nil {
				sessNoteError(nil, ErrGeneric)
				return IncompleteTypes()
			}
			if noVolatile && t.IsVolatileStructUnion() {
				// residual ERROR sticky — no invent soft-skip then pick later past field-Type hole
				if sessHasError(nil) {
					return IncompleteTypes()
				}
				continue
			}
			out = append(out, t)
		}
	}
	return out
}
