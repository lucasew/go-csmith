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
	// choose pointer to basic type — choose_random deferred to nonvoid simple
	// consolidate simple → int* (Type.cpp:1164–1166)
	_ = ChooseRandomNonvoidSimple(r, probs) // still burns RNG as choose_random would
	base := GetIntType()
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
	_ = noVolatile
	if r == nil {
		return GetIntType()
	}
	// pointer as LType (simple assign only)
	if op == AssignSimple && r.RndFlipcoin(uint32(probs.Single(PPointerAsLTypeProb))) {
		return env.MakeRandomPointerType(r, opts, probs)
	}
	// struct as LType when any structs exist (Type.cpp:1616–1622)
	if op == AssignSimple && env != nil && len(env.StructTypes) > 0 {
		if r.RndFlipcoin(uint32(probs.Single(PStructAsLTypeProb))) {
			return env.StructTypes[r.RndUpto(uint32(len(env.StructTypes)))]
		}
	}
	// union as LType (UnionAsLTypeProb) when any unions exist
	if op == AssignSimple && env != nil && len(env.UnionTypes) > 0 {
		if r.RndFlipcoin(uint32(probs.Single(PUnionAsLTypeProb))) {
			return env.UnionTypes[r.RndUpto(uint32(len(env.UnionTypes)))]
		}
	}

	// float as LType
	if AssignOpWorksForFloat(op) && r.RndFlipcoin(uint32(probs.Single(PFloatAsLTypeProb))) {
		if opts.EnableFloat {
			return GetSimpleType(EFloat)
		}
	}
	// default integer
	return GetIntType()
}
