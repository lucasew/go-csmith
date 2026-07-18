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
	// ArraySizes set when IsArray (element Type is Type field).
	ArraySizes []int
	// ArrayInits optional brace values for emit.
	ArrayInits []string
	// FieldVarOf is field_var_of (nil if not a field).
	FieldVarOf *Variable
	// FieldVars are expanded aggregate members (.f0, .f1, …).
	FieldVars []*Variable

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
	v := &Variable{
		Name: name,
		Type: typ,
		Qfer: qfer,
	}
	v.CreateFieldVars()
	return v
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

// CreateFieldVars mirrors Variable::create_field_vars for structs.
// Variable.cpp:337–370 — names name.f0, name.f1; OR parent const/vol into field qfer.
func (v *Variable) CreateFieldVars() {
	if v == nil || v.Type == nil || !v.Type.IsStruct() {
		return
	}
	if len(v.FieldVars) > 0 {
		return
	}
	isVol := v.IsVolatile()
	isConst := v.IsConst()
	j := 0
	for _, f := range v.Type.Fields {
		if f.Type == nil {
			continue
		}
		// Type::is_unamed_padding — zero-length bitfield skipped (Variable.cpp:351–352)
		if f.BitWidth == 0 {
			continue
		}
		fname := v.Name + ".f" + itoa(j)
		j++
		consts := append([]bool(nil), f.Qfer.IsConsts...)
		vols := append([]bool(nil), f.Qfer.IsVolatiles...)
		if len(consts) == 0 {
			consts = []bool{false}
		}
		if len(vols) == 0 {
			vols = []bool{false}
		}
		// quals.set_const(is_const_var || quals.is_const()) on storage (last)
		if isConst {
			consts[len(consts)-1] = true
		}
		if isVol {
			vols[len(vols)-1] = true
		}
		fv := &Variable{
			Name:       fname,
			Type:       f.Type,
			Qfer:       NewCVQualifiers(consts, vols),
			FieldVarOf: v,
		}
		// recursive expand nested structs
		fv.CreateFieldVars()
		v.FieldVars = append(v.FieldVars, fv)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// CollectExpandable returns v plus all field_vars recursively (expand_struct_union_vars-ish).
func (v *Variable) CollectExpandable() []*Variable {
	if v == nil {
		return nil
	}
	out := []*Variable{v}
	for _, f := range v.FieldVars {
		out = append(out, f.CollectExpandable()...)
	}
	return out
}
