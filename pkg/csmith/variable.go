// Upstream: Variable.h / Variable.cpp (CreateVariable, is_global/is_local/is_argument).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Variable mirrors Variable for non-array, non-field cases first.
type Variable struct {
	Name string
	Type *Type
	Qfer CVQualifiers

	IsAuto     bool
	IsStatic   bool
	IsRegister bool
	IsBitfield bool
	IsArray    bool
	// FieldVarOf is field_var_of (nil if not a field).
	FieldVarOf *Variable

	// Init mirrors Variable::init (Expression*); Constant only for now.
	Init *Constant
}

// CreateVariableQfer mirrors
// Variable::CreateVariable(name, type, init, qfer) without aggregate field expansion
// and without forcing Constant::make_random (init left nil until Constant port).
// Variable.cpp:405–421.
func CreateVariableQfer(name string, typ *Type, qfer CVQualifiers) *Variable {
	if typ != nil && typ.IsSimple() && typ.Simple() == EVoid {
		// Upstream asserts non-void simple; refuse quietly for Go.
		return nil
	}
	return &Variable{
		Name: name,
		Type: typ,
		Qfer: qfer,
	}
}

// CreateVariableScalars mirrors
// Variable::CreateVariable(name, type, isConst, isVolatile, …) for a scalar.
// Variable.cpp:368–378 → vectors of one bool each.
func CreateVariableScalars(name string, typ *Type, isConst, isVolatile bool) *Variable {
	return CreateVariableQfer(name, typ, NewCVQualifiers([]bool{isConst}, []bool{isVolatile}))
}

// IsGlobal mirrors Variable::is_global — name prefix "g_" (or field of global).
func (v *Variable) IsGlobal() bool {
	if v == nil {
		return false
	}
	if v.FieldVarOf != nil {
		return v.FieldVarOf.IsGlobal()
	}
	return len(v.Name) >= 2 && v.Name[0] == 'g' && v.Name[1] == '_'
}

// IsLocal mirrors Variable::is_local — name prefix "l_".
func (v *Variable) IsLocal() bool {
	if v == nil {
		return false
	}
	return len(v.Name) >= 2 && v.Name[0] == 'l' && v.Name[1] == '_'
}

// IsArgument mirrors Variable::is_argument — name prefix "p_".
func (v *Variable) IsArgument() bool {
	if v == nil {
		return false
	}
	return len(v.Name) >= 2 && v.Name[0] == 'p' && v.Name[1] == '_'
}

// IsConst mirrors Variable::is_const → qfer is_const_after_deref(0).
func (v *Variable) IsConst() bool {
	if v == nil {
		return false
	}
	return v.Qfer.IsConst()
}

// IsVolatile mirrors Variable::is_volatile.
func (v *Variable) IsVolatile() bool {
	if v == nil {
		return false
	}
	return v.Qfer.IsVolatile()
}

// IsPointer mirrors Variable::is_pointer.
func (v *Variable) IsPointer() bool {
	return v != nil && v.Type != nil && v.Type.PtrType() != nil
}
