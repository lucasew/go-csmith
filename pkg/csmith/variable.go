// Upstream: Variable.h / Variable.cpp (CreateVariable, is_global/is_local/is_argument).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// ctrl_vars pool mirrors Variable::ctrl_vars_vectors / ctrl_vars_count.
// Variable.cpp:73–74, 747–776.
var (
	ctrlVarsVectors [][]*Variable
	ctrlVarsCount   uint64
)

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

	// Init mirrors Variable::init when the expression is a Constant.
	Init *Constant
	// InitExpr is full Variable::init (Expression*) when non-constant (e.g. &x).
	// OutputDef prefers InitExpr over Init when set.
	InitExpr *Expression
	// IsAccessOnce mirrors Variable::isAccessOnce (ACCESS_ONCE wrap).
	IsAccessOnce bool
	// IsAddrTaken mirrors Variable::isAddrTaken (disables ACCESS_ONCE).
	IsAddrTaken bool
	// UseVolRVal mirrors wrap_volatiles path for VOL_RVAL emit.
	UseVolRVal bool
	// AsArray points to the ArrayVariable wrapper when this is an array collective.
	AsArray *ArrayVariable
}

// GetActualName mirrors Variable::get_actual_name.
// Variable.cpp:678–686 — globals may get_prefixed_name; default RNG returns name as-is.
func (v *Variable) GetActualName(prefixName bool) string {
	if v == nil {
		return ""
	}
	if v.IsGlobal() {
		return GetPrefixedName(v.Name, prefixName)
	}
	return v.Name
}

// GetPrefixedName mirrors get_prefixed_name (random.cpp:44–54).
// DefaultRndNumGenerator returns name unchanged when prefix is on (DefaultRndNumGenerator.cpp:105–106).
func GetPrefixedName(name string, prefixName bool) string {
	if !prefixName {
		return name
	}
	// sequence_name_prefix / DFS count prefix not used in default random mode
	return name
}

// OutputDecl mirrors Variable::OutputDecl — static? + qualified type + name.
// Variable.cpp:670–676.
func (v *Variable) OutputDecl(forceStatic bool) string {
	return v.OutputDeclOpts(forceStatic, false)
}

// OutputDeclOpts includes prefix_name option.
func (v *Variable) OutputDeclOpts(forceStatic, prefixName bool) string {
	if v == nil {
		return ""
	}
	var b strings.Builder
	if forceStatic && v.IsGlobal() {
		b.WriteString("static ")
	}
	b.WriteString(v.Qfer.OutputQualifiedType(v.Type))
	b.WriteString(" ")
	b.WriteString(v.GetActualName(prefixName))
	return b.String()
}

// OutputDef mirrors Variable::OutputDef — decl + init + ";" + optional volatile comment.
// Variable.cpp:640–665.
func (v *Variable) OutputDef(forceStatic bool) string {
	return v.OutputDefOpts(forceStatic, false)
}

// OutputDefOpts adds prefix_name and VOLATILE GLOBAL comment for volatile globals.
func (v *Variable) OutputDefOpts(forceStatic, prefixName bool) string {
	return v.OutputDefFull(forceStatic, prefixName, false, nil)
}

// OutputDefFull mirrors Variable::OutputDef with optional __attribute__.
// Variable.cpp:640–665 — decl, attr_generator.Output, init, volatile comment.
func (v *Variable) OutputDefFull(forceStatic, prefixName, withAttrs bool, r *Rng) string {
	if v == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(v.OutputDeclOpts(forceStatic, prefixName))
	// Variable.cpp:655 — var_attr_generator.Output when attributes enabled
	if withAttrs && r != nil {
		b.WriteString(EnsureVarAttrGenerator().Output(r))
	}
	// Variable.cpp:656 — init->Output when present
	if v.InitExpr != nil {
		if s := v.InitExpr.Output(); s != "" {
			b.WriteString(" = ")
			b.WriteString(s)
		}
	} else if v.Init != nil && v.Init.Value != "" {
		b.WriteString(" = ")
		b.WriteString(v.Init.Value)
	}
	b.WriteString(";")
	// Variable.cpp:658–661 — volatile global comment on same line path uses comment helper
	if v.IsGlobal() && v.IsVolatile() {
		b.WriteString(" /* VOLATILE GLOBAL ")
		b.WriteString(v.GetActualName(prefixName))
		b.WriteString(" */")
	}
	return b.String()
}

// OutputC mirrors Variable::Output — VOL_RVAL / ACCESS_ONCE / bare name.
// Variable.cpp:689–700.
func (v *Variable) OutputC() string {
	return v.OutputCOpts(false)
}

// OutputCOpts is Output with prefix_name.
// ArrayVariable::Output overrides Variable::Output for itemized members
// (ArrayVariable.cpp:539–571) — name + indices, no VOL_RVAL wrap.
func (v *Variable) OutputCOpts(prefixName bool) string {
	if v == nil {
		return ""
	}
	// ArrayVariable.cpp:539 — virtual Output for array (itemized or collective)
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return v.AsArray.OutputAccess()
	}
	name := v.GetActualName(prefixName)
	if v.UseVolRVal && v.IsVolatile() {
		ty := "int"
		if v.Type != nil {
			ty = v.Type.CName()
		}
		return "VOL_RVAL(" + name + ", " + ty + ")"
	}
	if v.IsAccessOnce && !v.IsAddrTaken {
		return "ACCESS_ONCE(" + name + ")"
	}
	return name
}

// OutputLhsC mirrors Lhs::Output — VOL_LVAL when wrap_volatiles.
// Lhs.cpp:207–218.
func (v *Variable) OutputLhsC() string {
	return v.OutputLhsCOpts(false)
}

// OutputLhsCOpts is OutputLhs with prefix_name.
// Itemized arrays use ArrayVariable::Output (indices) as LHS text.
func (v *Variable) OutputLhsCOpts(prefixName bool) string {
	if v == nil {
		return ""
	}
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return v.AsArray.OutputAccess()
	}
	name := v.GetActualName(prefixName)
	if v.UseVolRVal && v.IsVolatile() {
		ty := "int"
		if v.Type != nil {
			ty = v.Type.CName()
		}
		return "VOL_LVAL(" + name + ", " + ty + ")"
	}
	return name
}

// OutputAddrOf mirrors Variable::OutputAddrOf — always &actual_name (no VOL_RVAL).
// Variable.cpp:707–710.
func (v *Variable) OutputAddrOf(prefixName bool) string {
	if v == nil {
		return "&0"
	}
	return "&" + v.GetActualName(prefixName)
}

// OutputForComment mirrors Variable::OutputForComment — bare actual name.
// Variable.cpp:711–713.
func (v *Variable) OutputForComment(prefixName bool) string {
	if v == nil {
		return ""
	}
	return v.GetActualName(prefixName)
}

// OutputUpperBound mirrors Variable::OutputUpperBound — field path for bounds.
// Variable.cpp:721–732; ArrayVariable.cpp:572–577 for arrays.
func (v *Variable) OutputUpperBound(prefixName bool) string {
	if v == nil {
		return ""
	}
	if v.AsArray != nil && len(v.AsArray.Sizes) > 0 {
		return v.AsArray.OutputUpperBoundArray()
	}
	if v.FieldVarOf != nil {
		base := v.FieldVarOf.OutputUpperBound(prefixName)
		dot := strings.LastIndex(v.Name, ".")
		if dot < 0 {
			return base
		}
		return base + v.Name[dot:]
	}
	return v.GetActualName(prefixName)
}

// OutputLowerBound mirrors Variable::OutputLowerBound.
// Variable.cpp:734–745. Arrays override separately.
func (v *Variable) OutputLowerBound(prefixName bool) string {
	if v == nil {
		return ""
	}
	if v.AsArray != nil {
		return v.AsArray.OutputLowerBound()
	}
	if v.FieldVarOf != nil {
		base := v.FieldVarOf.OutputLowerBound(prefixName)
		dot := strings.LastIndex(v.Name, ".")
		if dot < 0 {
			return base
		}
		return base + v.Name[dot:]
	}
	return v.GetActualName(prefixName)
}

// NewCtrlVars mirrors Variable::new_ctrl_vars — i,j,k… (±suffix when fresh).
// Variable.cpp:747–767. maxDim is CGOptions::max_array_dimensions().
func NewCtrlVars(maxDim int, freshNames bool) []*Variable {
	if maxDim < 1 {
		maxDim = 1
	}
	suffix := ctrlVarsCount
	ctrl := make([]*Variable, 0, maxDim)
	name := byte('i')
	for i := 0; i < maxDim; i++ {
		nm := string([]byte{name})
		if freshNames {
			nm += itoa(int(suffix))
		}
		ctrl = append(ctrl, &Variable{
			Name: nm,
			Type: GetIntType(),
			Qfer: NewCVQualifiers([]bool{false}, []bool{false}),
		})
		name++
	}
	ctrlVarsCount++
	ctrlVarsVectors = append(ctrlVarsVectors, ctrl)
	return ctrl
}

// GetNewCtrlVars mirrors Variable::get_new_ctrl_vars.
func GetNewCtrlVars(opts Options) []*Variable {
	return NewCtrlVars(opts.MaxArrayDim, opts.FreshArrayCtrlVarNames)
}

// GetLastCtrlVars mirrors Variable::get_last_ctrl_vars.
// Variable.cpp:774–776. Returns nil if none allocated.
func GetLastCtrlVars() []*Variable {
	if len(ctrlVarsVectors) == 0 {
		return nil
	}
	return ctrlVarsVectors[len(ctrlVarsVectors)-1]
}

// CtrlVarsDoFinalization mirrors Variable::doFinalization for ctrl var pool.
// Variable.cpp:779–786.
func CtrlVarsDoFinalization() {
	ctrlVarsVectors = nil
	ctrlVarsCount = 0
}

// CtrlVarNames returns actual names of a ctrl-var slice.
func CtrlVarNames(ctrl []*Variable) []string {
	out := make([]string, len(ctrl))
	for i, v := range ctrl {
		if v == nil {
			out[i] = "i" + itoa(i)
			continue
		}
		out[i] = v.GetActualName(false)
	}
	return out
}

// OutputArrayCtrlVars mirrors OutputArrayCtrlVars — "int i, j, k;".
// Variable.cpp:800–811.
func OutputArrayCtrlVars(ctrl []*Variable, dimen int, indent string) string {
	if dimen <= 0 || len(ctrl) == 0 {
		return ""
	}
	if dimen > len(ctrl) {
		dimen = len(ctrl)
	}
	var b strings.Builder
	b.WriteString(indent + "int ")
	for i := 0; i < dimen; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		if ctrl[i] != nil {
			b.WriteString(ctrl[i].GetActualName(false))
		} else {
			b.WriteString(string([]byte{byte('i' + i)}))
		}
	}
	b.WriteString(";\n")
	return b.String()
}

// GetMaxArrayDimension mirrors Variable::GetMaxArrayDimension.
// Variable.cpp:813–826.
func GetMaxArrayDimension(vars []*Variable) int {
	dimen := 0
	for _, v := range vars {
		if v == nil || !v.IsArray {
			continue
		}
		n := len(v.ArraySizes)
		if v.AsArray != nil && len(v.AsArray.Sizes) > n {
			n = len(v.AsArray.Sizes)
		}
		if n > dimen {
			dimen = n
		}
	}
	return dimen
}

// OutputArrayInitializers mirrors OutputArrayInitializers for loop-init arrays.
// Variable.cpp:829–841 — allocate ctrl vars, declare, emit output_init.
func OutputArrayInitializers(vars []*Variable, opts Options, indent string) string {
	dimen := GetMaxArrayDimension(vars)
	if dimen == 0 {
		return ""
	}
	ctrl := GetNewCtrlVars(opts)
	var b strings.Builder
	b.WriteString(OutputArrayCtrlVars(ctrl, dimen, indent))
	names := CtrlVarNames(ctrl)
	for _, v := range vars {
		if v == nil || !v.IsArray {
			continue
		}
		av := v.AsArray
		if av == nil {
			av = &ArrayVariable{Variable: *v, Sizes: v.ArraySizes, InitValues: v.ArrayInits}
		}
		if av.NoLoopInitializer() {
			continue
		}
		b.WriteString(av.OutputInit(indent, names))
	}
	return b.String()
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

// IsRV mirrors Variable::is_rv — return dummy name ends with "_rv".
func (v *Variable) IsRV() bool {
	if v == nil || len(v.Name) < 3 {
		return false
	}
	return v.Name[len(v.Name)-3:] == "_rv"
}

// IsTmpVar mirrors Variable::is_tmp_var — name prefix "t_".
// Variable.cpp:512–514.
func (v *Variable) IsTmpVar() bool {
	if v == nil {
		return false
	}
	return len(v.Name) >= 2 && v.Name[0] == 't' && v.Name[1] == '_'
}

// IsValidVolatile mirrors Variable::is_valid_volatile.
// Variable.cpp:1061–1072 — union fields recurse; const null pointer init invalid.
func (v *Variable) IsValidVolatile() bool {
	if v == nil {
		return false
	}
	if v.IsInsideUnionField() {
		uv := v.GetContainerUnion()
		if uv == nil {
			return false
		}
		return uv.IsValidVolatile()
	}
	// non-const, or non-zero init, or non-pointer → valid
	if !v.IsConst() {
		return true
	}
	if v.Type == nil || !v.Type.IsPointerLike() {
		return true
	}
	// const pointer: invalid only when init equals 0 (null)
	if v.Init == nil {
		// no init expression — treat as valid (cannot prove null)
		return true
	}
	return v.Init.NotEqualsZero()
}

// IsPackedAggregateFieldVar mirrors Variable::is_packed_aggregate_field_var.
// Variable.cpp:307–312 — any ancestor field_var_of has packed aggregate type.
func (v *Variable) IsPackedAggregateFieldVar() bool {
	if v == nil {
		return false
	}
	for p := v.FieldVarOf; p != nil; p = p.FieldVarOf {
		if p.Type != nil && p.Type.Packed {
			return true
		}
	}
	return false
}

// IsPackedAfterBitfield mirrors Variable::is_packed_after_bitfield.
// Variable.cpp:1240–1258 — packed struct field after a bitfield has unstable offset.
func (v *Variable) IsPackedAfterBitfield() bool {
	if v == nil || v.FieldVarOf == nil {
		return false
	}
	parent := v.FieldVarOf
	if parent.Type != nil && parent.Type.IsStruct() && parent.Type.Packed {
		for i, f := range parent.FieldVars {
			if f == v {
				break
			}
			if parent.Type.IsBitfieldIndex(i) {
				return true
			}
			if f != nil && f.Type != nil && f.Type.HasBitfields() {
				return true
			}
		}
	}
	return parent.IsPackedAfterBitfield()
}

// IsArrayField mirrors Variable::is_array_field.
// Variable.cpp:270–277 — field of an array variable (or recursive).
func (v *Variable) IsArrayField() bool {
	if v == nil || v.FieldVarOf == nil {
		return false
	}
	p := v.FieldVarOf
	if p.IsArray || p.AsArray != nil {
		return true
	}
	return p.IsArrayField()
}

// MatchVarName mirrors Variable::match_var_name.
// Variable.cpp:1205–1222 — name match, array Output text, or field recurse.
func (v *Variable) MatchVarName(vname string) *Variable {
	if v == nil || vname == "" {
		return nil
	}
	if v.Name == vname {
		return v
	}
	// array / array field: compare Output text
	if v.IsArray || v.IsArrayField() {
		if v.OutputC() == vname {
			return v
		}
		// itemized with indices
		if v.AsArray != nil && len(v.AsArray.Indices) > 0 {
			if v.AsArray.OutputAccess() == vname {
				return v
			}
		}
	}
	for _, f := range v.FieldVars {
		if m := f.MatchVarName(vname); m != nil {
			return m
		}
	}
	return nil
}

// IsSeenName mirrors Variable::is_seen_name.
// Variable.cpp:1048–1058 — name starts with seen+"[".
func IsSeenName(seen []string, name string) bool {
	for _, n := range seen {
		prefix := n + "["
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
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

// IsVisible mirrors Variable::is_visible — global or is_visible_local.
// Variable.h / usage in select_must_use_var.
func (v *Variable) IsVisible(blk *Block) bool {
	if v == nil {
		return false
	}
	if v.IsGlobal() {
		return true
	}
	return v.IsVisibleLocal(blk)
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

// IsPartialVolatileAfterDeref mirrors Variable::is_partial_volatile_after_deref.
// Variable.cpp:541–558 — not fully volatile at level, but pointee is volatile struct/union.
func (v *Variable) IsPartialVolatileAfterDeref(derefLevel int) bool {
	if v == nil || derefLevel < 0 {
		return false
	}
	// whole type volatile at this deref → not "partial"
	if v.Qfer.IsVolatileAfterDeref(derefLevel) {
		return false
	}
	t := v.Type
	for i := 0; i < derefLevel && t != nil; i++ {
		t = t.PtrType()
	}
	if t != nil {
		return t.IsVolatileStructUnion()
	}
	return false
}

// Compatible mirrors Variable::compatible.
// Variable.cpp:878–886 — no volatiles; same ptr; expand_struct only non-fields.
func (v *Variable) Compatible(other *Variable, expandStruct bool) bool {
	if v == nil || other == nil {
		return false
	}
	if v.IsVolatile() || other.IsVolatile() {
		return false
	}
	if v == other {
		return true
	}
	if expandStruct {
		return !v.IsFieldVar() && !other.IsFieldVar()
	}
	return false
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

// GetCollective mirrors Variable::get_collective — array items → parent array.
func (v *Variable) GetCollective() *Variable {
	if v == nil {
		return nil
	}
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return &v.AsArray.Collective.Variable
	}
	return v
}

// Match mirrors Variable::match — identity, or aggregate has field.
// Variable.cpp:254–258.
func (v *Variable) Match(other *Variable) bool {
	if v == nil || other == nil {
		return false
	}
	if v == other {
		return true
	}
	if v.Type != nil && v.Type.IsAggregate() {
		return v.HasFieldVar(other)
	}
	return false
}

// HasFieldVar mirrors Variable::has_field_var — other is this or nested field.
func (v *Variable) HasFieldVar(other *Variable) bool {
	if v == nil || other == nil {
		return false
	}
	if v == other {
		return true
	}
	for _, f := range v.FieldVars {
		if f.HasFieldVar(other) {
			return true
		}
	}
	return false
}

// GetContainerUnion mirrors Variable::get_container_union.
// Variable.cpp:226–232 — walk field_var_of until union type.
func (v *Variable) GetContainerUnion() *Variable {
	for p := v; p != nil; p = p.FieldVarOf {
		if p.Type != nil && p.Type.IsUnion() {
			return p
		}
	}
	return nil
}

// LooseMatch mirrors Variable::loose_match.
// Variable.cpp:239–250 — match collective, or same container union.
func (v *Variable) LooseMatch(other *Variable) bool {
	if v == nil || other == nil {
		return false
	}
	me := v.GetCollective()
	you := other.GetCollective()
	if me.Match(you) {
		return true
	}
	meU := me.GetContainerUnion()
	youU := you.GetContainerUnion()
	return meU != nil && youU != nil && meU == youU
}

// IsUnionField mirrors Variable::is_union_field — direct field of a union.
func (v *Variable) IsUnionField() bool {
	return v != nil && v.FieldVarOf != nil && v.FieldVarOf.Type != nil && v.FieldVarOf.Type.IsUnion()
}

// IsInsideUnionField mirrors Variable::is_inside_union_field.
func (v *Variable) IsInsideUnionField() bool {
	for p := v; p != nil; p = p.FieldVarOf {
		if p.IsUnionField() {
			return true
		}
	}
	return false
}

// GetFieldID mirrors Variable::get_field_id — index in parent FieldVars, or -1.
// Variable.cpp:323–333.
func (v *Variable) GetFieldID() int {
	if v == nil || v.FieldVarOf == nil {
		return -1
	}
	for i, f := range v.FieldVarOf.FieldVars {
		if f == v {
			return i
		}
	}
	return -1
}

// FindPointerFields mirrors Variable::find_pointer_fields.
// Variable.cpp:1228–1235 — recursive pointer fields of aggregates.
func (v *Variable) FindPointerFields() []*Variable {
	if v == nil {
		return nil
	}
	var out []*Variable
	for _, f := range v.FieldVars {
		if f == nil {
			continue
		}
		if f.IsPointer() {
			out = append(out, f)
		} else if f.IsAggregate() {
			out = append(out, f.FindPointerFields()...)
		}
	}
	return out
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

// OutputValueDump mirrors Variable::output_value_dump.
// Variable.cpp:1173–1203 — printf checksum lines for simples; recurse aggregates;
// arrays expand all index combinations; unions only readable fields.
func (v *Variable) OutputValueDump(prefix string, indent int, unionFacts []*FactUnion) string {
	if v == nil || v.Type == nil {
		return ""
	}
	// virtual array collective → expand all indices (Variable.cpp:1175–1183)
	if v.IsArray && len(v.ArraySizes) > 0 {
		if v.AsArray != nil && v.AsArray.Collective != nil {
			// already itemized member — treat as scalar element access
		} else {
			return outputValueDumpArray(v, prefix, indent, unionFacts)
		}
	}
	if v.Type.IsSimple() {
		// Variable.cpp:1184–1188
		name := v.GetActualName(false)
		dir := v.Type.PrintfDirective()
		return OutputTab(indent) + "printf(\"" + prefix + name + " = " + dir + "\\n\", " + name + ");\n"
	}
	if v.Type.IsStruct() {
		var b strings.Builder
		for _, f := range v.FieldVars {
			b.WriteString(f.OutputValueDump(prefix, indent, unionFacts))
		}
		return b.String()
	}
	if v.Type.IsUnion() {
		var b strings.Builder
		for i, f := range v.FieldVars {
			// Variable.cpp:1195–1200 — FactUnion::is_field_readable (program end facts)
			if !IsFieldReadable(v, i, unionFacts) {
				continue
			}
			b.WriteString(f.OutputValueDump(prefix, indent, unionFacts))
		}
		return b.String()
	}
	// pointers: dump as pointer directive
	if v.Type.IsPointerLike() {
		name := v.GetActualName(false)
		dir := v.Type.PrintfDirective()
		return OutputTab(indent) + "printf(\"" + prefix + name + " = " + dir + "\\n\", " + name + ");\n"
	}
	return ""
}

// outputValueDumpArray expands array into all index combinations and dumps elements.
func outputValueDumpArray(v *Variable, prefix string, indent int, unionFacts []*FactUnion) string {
	if v == nil || len(v.ArraySizes) == 0 {
		return ""
	}
	all := expandWithinRanges(v.ArraySizes)
	var b strings.Builder
	for _, idx := range all {
		// build access name g_a[0][1]
		name := v.GetActualName(false)
		for _, i := range idx {
			name += "[" + itoa(i) + "]"
		}
		if v.Type != nil && v.Type.IsSimple() {
			dir := v.Type.PrintfDirective()
			b.WriteString(OutputTab(indent) + "printf(\"" + prefix + name + " = " + dir + "\\n\", " + name + ");\n")
			continue
		}
		if v.Type != nil && v.Type.IsAggregate() && len(v.FieldVars) > 0 {
			// dump fields with indexed prefix path via synthetic names
			for fi, f := range v.FieldVars {
				if v.Type.IsUnion() && !IsFieldReadable(v, fi, unionFacts) {
					continue
				}
				if f == nil || f.Type == nil || !f.Type.IsSimple() {
					continue
				}
				// field name is typically g_a.f0 — replace base with indexed access
				fname := f.Name
				// f.Name like "g_a.f0" → use name + ".f" + idx
				suffix := ""
				if dot := lastDot(fname); dot >= 0 {
					suffix = fname[dot:]
				} else {
					suffix = ".f" + itoa(fi)
				}
				acc := name + suffix
				dir := f.Type.PrintfDirective()
				b.WriteString(OutputTab(indent) + "printf(\"" + prefix + acc + " = " + dir + "\\n\", " + acc + ");\n")
			}
		}
	}
	return b.String()
}

// expandWithinRanges mirrors expand_within_ranges — all index vectors in [0,size).
func expandWithinRanges(sizes []int) [][]int {
	if len(sizes) == 0 {
		return nil
	}
	var out [][]int
	var rec func(prefix []int, dim int)
	rec = func(prefix []int, dim int) {
		if dim == len(sizes) {
			cp := append([]int(nil), prefix...)
			out = append(out, cp)
			return
		}
		n := sizes[dim]
		if n < 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			rec(append(prefix, i), dim+1)
		}
	}
	rec(nil, 0)
	return out
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
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
// Standalone path allocates temporary letter ctrl names (i,j,…) without
// mutating the package ctrl-var pool when no last set exists.
// unionFacts nil → hash all union fields (no FactMgr); non-nil → IsFieldReadable.
func (v *Variable) HashOutput() string {
	return v.hashOutput(nil, nil)
}

// HashOutputWithUnionFacts is HashOutput with FactUnion last-write filtering.
func (v *Variable) HashOutputWithUnionFacts(unionFacts []*FactUnion) string {
	return v.hashOutput(nil, unionFacts)
}

func (v *Variable) hashOutput(ctrl []*Variable, unionFacts []*FactUnion) string {
	if v == nil || v.Type == nil {
		return ""
	}
	if v.IsArray && len(v.ArraySizes) > 0 {
		return hashArrayVariable(v, ctrl, unionFacts)
	}
	if v.Type.IsAggregate() {
		var b strings.Builder
		for i, f := range v.FieldVars {
			// Variable.cpp:893–898 — skip unreadable union fields
			if v.Type.IsUnion() && unionFacts != nil {
				if !IsFieldReadable(v, i, unionFacts) {
					continue
				}
			}
			b.WriteString(f.hashOutput(ctrl, unionFacts))
		}
		return b.String()
	}
	if v.Type.IsSimple() {
		name := v.GetActualName(false)
		if v.Type.IsFloat() {
			return "    transparent_crc_bytes (&" + name + ", sizeof(" + name + "), \"" + v.Name + "\", print_hash_value);\n"
		}
		return "    transparent_crc(" + name + ", \"" + v.Name + "\", print_hash_value);\n"
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
// ArrayVariable.cpp:735–820 — uses get_last_ctrl_vars names (i,j,k…).
// Union array elements: exclude unreadable fields when unionFacts non-nil
// (ArrayVariable.cpp:741–752).
// Skips arrays with no hashable payload (e.g. pointer element type).
// ctrl nil → synthesize letter names for standalone HashOutput.
func hashArrayVariable(v *Variable, ctrl []*Variable, unionFacts []*FactUnion) string {
	if v == nil || len(v.ArraySizes) == 0 || !hashArrayHasPayload(v) {
		return ""
	}
	if ctrl == nil {
		ctrl = GetLastCtrlVars()
	}
	names := make([]string, len(v.ArraySizes))
	for i := range v.ArraySizes {
		if i < len(ctrl) && ctrl[i] != nil {
			names[i] = ctrl[i].GetActualName(false)
		} else {
			// fallback letter names matching new_ctrl_vars
			names[i] = string([]byte{byte('i' + i)})
		}
	}
	var b strings.Builder
	indent := "    "
	for i, sz := range v.ArraySizes {
		iv := names[i]
		b.WriteString(indent + "for (" + iv + " = 0; " + iv + " < " + itoa(sz) + "; " + iv + "++)\n")
		b.WriteString(indent + "{\n")
		indent += "    "
	}
	// ArrayVariable::output_with_indices
	access := v.GetActualName(false)
	nameStr := access
	for _, iv := range names {
		access += "[" + iv + "]"
		nameStr += "[" + iv + "]"
	}
	if v.Type != nil && v.Type.IsAggregate() {
		j := 0
		for i, f := range v.Type.Fields {
			if f.Type == nil || f.BitWidth == 0 {
				continue
			}
			// ArrayVariable.cpp:741–752 — skip unreadable union fields
			if v.Type.IsUnion() && unionFacts != nil && !IsFieldReadable(v, i, unionFacts) {
				j++
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
