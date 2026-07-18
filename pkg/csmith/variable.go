// Upstream: Variable.h / Variable.cpp (CreateVariable, is_global/is_local/is_argument).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

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
	// IsAccessOnce mirrors Variable::isAccessOnce (ACCESS_ONCE wrap).
	IsAccessOnce bool
	// IsAddrTaken mirrors Variable::isAddrTaken (disables ACCESS_ONCE).
	IsAddrTaken bool
	// UseVolRVal mirrors wrap_volatiles path for VOL_RVAL emit.
	UseVolRVal bool
}

// OutputC mirrors Variable::Output — VOL_RVAL / ACCESS_ONCE / bare name.
// Variable.cpp:689–700.
func (v *Variable) OutputC() string {
	if v == nil {
		return ""
	}
	if v.UseVolRVal && v.IsVolatile() {
		ty := "int"
		if v.Type != nil {
			ty = v.Type.CName()
		}
		return "VOL_RVAL(" + v.Name + ", " + ty + ")"
	}
	if v.IsAccessOnce && !v.IsAddrTaken {
		return "ACCESS_ONCE(" + v.Name + ")"
	}
	return v.Name
}

// OutputLhsC mirrors Lhs::Output — VOL_LVAL when wrap_volatiles.
// Lhs.cpp:207–218.
func (v *Variable) OutputLhsC() string {
	if v == nil {
		return ""
	}
	if v.UseVolRVal && v.IsVolatile() {
		ty := "int"
		if v.Type != nil {
			ty = v.Type.CName()
		}
		return "VOL_LVAL(" + v.Name + ", " + ty + ")"
	}
	// ACCESS_ONCE not used for write LHS (upstream ExpressionVariable only on Lhs via Output)
	return v.Name
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

// IsFieldVar mirrors Variable::is_field_var.
func (v *Variable) IsFieldVar() bool {
	return v != nil && v.FieldVarOf != nil
}

// IsVisibleLocal mirrors Variable::is_visible_local.
// Variable.cpp:482–503 — params + block-chain locals; fields recurse parent.
func (v *Variable) IsVisibleLocal(blk *Block) bool {
	if v == nil {
		return false
	}
	if blk == nil {
		return v.IsGlobal()
	}
	if v.IsFieldVar() {
		return v.FieldVarOf.IsVisibleLocal(blk)
	}
	// params of blk's function
	f := blk.Func
	for b := blk; f == nil && b != nil; b = b.Parent {
		f = b.Func
	}
	if f != nil {
		for _, p := range f.Param {
			if p == v {
				return true
			}
		}
	}
	for b := blk; b != nil; b = b.Parent {
		for _, loc := range b.LocalVars {
			if loc == v {
				return true
			}
		}
	}
	return false
}

// IsConstAfterDeref mirrors Variable::is_const_after_deref.
// Variable.cpp:521–538.
func (v *Variable) IsConstAfterDeref(derefLevel int) bool {
	if v == nil || derefLevel < 0 {
		return false
	}
	if v.Qfer.IsConstAfterDeref(derefLevel) {
		return true
	}
	t := v.Type
	for i := 0; i < derefLevel && t != nil; i++ {
		t = t.PtrType()
	}
	if t != nil {
		return t.IsConstStructUnion()
	}
	return false
}

// IsVolatileAfterDeref mirrors Variable::is_volatile_after_deref (qfer path).
func (v *Variable) IsVolatileAfterDeref(derefLevel int) bool {
	if v == nil || derefLevel < 0 {
		return false
	}
	if v.Qfer.IsVolatileAfterDeref(derefLevel) {
		return true
	}
	return v.IsVolatile() && derefLevel == 0
}

// IsPointer mirrors Variable::is_pointer.
func (v *Variable) IsPointer() bool {
	return v != nil && v.Type != nil && v.Type.PtrType() != nil
}

// IsVirtual mirrors Variable::is_virtual — dummy statics (null/garbage/tbd) have nil Type.
func (v *Variable) IsVirtual() bool {
	return v != nil && v.Type == nil
}

// IsAggregate mirrors Variable::is_aggregate.
func (v *Variable) IsAggregate() bool {
	return v != nil && v.Type != nil && v.Type.IsAggregate()
}

// MakeDummyStaticVariable mirrors VariableSelector::make_dummy_static_variable.
// VariableSelector.cpp:1565–1568 — name only, type null.
func MakeDummyStaticVariable(name string) *Variable {
	return &Variable{Name: name, Type: nil}
}

// CreateFieldVars mirrors Variable::create_field_vars for structs.
// Variable.cpp:337–370 — names name.f0, name.f1; OR parent const/vol into field qfer.
func (v *Variable) CreateFieldVars() {
	if v == nil || v.Type == nil || !v.Type.IsAggregate() {
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
			// bitfields_length_[i] >= 0 → isBitfield (Type::is_bitfield)
			IsBitfield: f.BitWidth >= 0,
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

// HashOutput mirrors Variable::hash for compute_hash path.
// Variable.cpp:889–923 — aggregates recurse fields; simple transparent_crc;
// float uses transparent_crc_bytes; pointers emit nothing.
// FactUnion readability omitted (hash all union fields).
// declareIdx: if true, emit int iN decls (standalone use); HashGlobalVariables
// declares shared index vars once.
func (v *Variable) HashOutput() string {
	return v.hashOutput(true)
}

func (v *Variable) hashOutput(declareIdx bool) string {
	if v == nil || v.Type == nil {
		return ""
	}
	if v.IsArray && len(v.ArraySizes) > 0 {
		return hashArrayVariable(v, declareIdx)
	}
	if v.Type.IsAggregate() {
		var b strings.Builder
		for _, f := range v.FieldVars {
			b.WriteString(f.hashOutput(declareIdx))
		}
		return b.String()
	}
	if v.Type.IsSimple() {
		if v.Type.IsFloat() {
			return "    transparent_crc_bytes (&" + v.Name + ", sizeof(" + v.Name + "), \"" + v.Name + "\", print_hash_value);\n"
		}
		return "    transparent_crc(" + v.Name + ", \"" + v.Name + "\", print_hash_value);\n"
	}
	// ePointer: no hash (Variable.cpp:921–922)
	return ""
}

// hashArrayHasPayload reports whether array hashing would emit any transparent_crc.
func hashArrayHasPayload(v *Variable) bool {
	if v == nil || v.Type == nil {
		return false
	}
	if v.Type.IsSimple() {
		return true
	}
	if v.Type.IsAggregate() {
		j := 0
		for _, f := range v.Type.Fields {
			if f.Type == nil || f.BitWidth == 0 {
				continue
			}
			if f.Type.IsSimple() {
				return true
			}
			j++
		}
	}
	return false
}

// hashArrayVariable mirrors ArrayVariable::hash (loop over dims; simple elements).
// ArrayVariable.cpp:735+ simplified — index vars i0..; no FactUnion exclude.
// Skips arrays with no hashable payload (e.g. pointer element type).
func hashArrayVariable(v *Variable, declareIdx bool) string {
	if v == nil || len(v.ArraySizes) == 0 || !hashArrayHasPayload(v) {
		return ""
	}
	var b strings.Builder
	if declareIdx {
		for i := range v.ArraySizes {
			b.WriteString("    int i" + itoa(i) + ";\n")
		}
	}
	indent := "    "
	for i, sz := range v.ArraySizes {
		iv := "i" + itoa(i)
		b.WriteString(indent + "for (" + iv + " = 0; " + iv + " < " + itoa(sz) + "; " + iv + "++)\n")
		b.WriteString(indent + "{\n")
		indent += "    "
	}
	access := v.Name
	nameStr := v.Name
	for i := range v.ArraySizes {
		access += "[i" + itoa(i) + "]"
		nameStr += "[i" + itoa(i) + "]"
	}
	if v.Type != nil && v.Type.IsAggregate() {
		j := 0
		for _, f := range v.Type.Fields {
			if f.Type == nil || f.BitWidth == 0 {
				continue
			}
			if f.Type.IsSimple() && !f.Type.IsFloat() {
				fn := ".f" + itoa(j)
				b.WriteString(indent + "transparent_crc(" + access + fn + ", \"" + nameStr + fn + "\", print_hash_value);\n")
			} else if f.Type.IsSimple() && f.Type.IsFloat() {
				fn := ".f" + itoa(j)
				b.WriteString(indent + "transparent_crc_bytes (&" + access + fn + ", sizeof(" + access + fn + "), \"" + nameStr + fn + "\", print_hash_value);\n")
			}
			j++
		}
	} else if v.Type != nil && v.Type.IsSimple() {
		if v.Type.IsFloat() {
			b.WriteString(indent + "transparent_crc_bytes (&" + access + ", sizeof(" + access + "), \"" + nameStr + "\", print_hash_value);\n")
		} else {
			b.WriteString(indent + "transparent_crc(" + access + ", \"" + nameStr + "\", print_hash_value);\n")
		}
	}
	for range v.ArraySizes {
		indent = indent[:len(indent)-4]
		b.WriteString(indent + "}\n")
	}
	return b.String()
}
