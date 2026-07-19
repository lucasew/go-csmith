// Upstream: Variable.h / Variable.cpp (CreateVariable, is_global/is_local/is_argument).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"strings"
)

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
// Variable always has live name for emit; empty Name sticky (no invent empty
// identifier soft-skip past incomplete name shell; callers previously sticky only).
func (v *Variable) GetActualName(prefixName bool) string {
	// Variable* always live for name emit; sticky no invent empty identifier
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	if v.Name == "" {
		SetError(ErrGeneric)
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
	// Variable* always live at OutputDecl; sticky no invent decl shell without it
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Variable.cpp:670–676 — output_qualified_type always live type; sticky no invent " name"
	ty := v.Qfer.OutputQualifiedType(v.Type)
	if ty == "" {
		SetError(ErrGeneric)
		return ""
	}
	name := v.GetActualName(prefixName)
	// name always live; sticky no invent "int " without identifier
	if name == "" {
		SetError(ErrGeneric)
		return ""
	}
	var b strings.Builder
	if forceStatic && v.IsGlobal() {
		b.WriteString("static ")
	}
	b.WriteString(ty)
	b.WriteString(" ")
	b.WriteString(name)
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
// IsArray without AsArray sticky empty (no invent scalar "int g_a = 0;" past array shell).
func (v *Variable) OutputDefFull(forceStatic, prefixName, withAttrs bool, r *Rng) string {
	// Variable* always live at OutputDef; sticky no invent def shell without it
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky
	// (no invent scalar OutputDef for IsArray shell — use ArrayVariable::OutputDef)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Variable.cpp:659 — assert(init); sticky no soft invent empty "= ;" RHS
	var initOut string
	if v.InitExpr != nil {
		initOut = v.InitExpr.Output()
	} else if v.Init != nil {
		initOut = v.Init.Value
	}
	if initOut == "" {
		// missing init is broken IR sticky (union-field CreateVariable uses null init
		// and those fields are not OutputDef'd as standalone defs)
		SetError(ErrGeneric)
		return ""
	}
	// Variable.cpp:640–660 — OutputDecl always live; sticky no invent " = init;" without decl
	decl := v.OutputDeclOpts(forceStatic, prefixName)
	if decl == "" {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return ""
	}
	var b strings.Builder
	b.WriteString(decl)
	// Variable.cpp:655 — var_attr_generator.Output when attributes enabled
	if withAttrs && r != nil {
		b.WriteString(EnsureVarAttrGenerator().Output(r))
	}
	// Variable.cpp:656–660 — out << " = "; assert(init); init->Output
	b.WriteString(" = ")
	b.WriteString(initOut)
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
	// Variable* always live at Output; sticky no invent empty C name without it
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky
	// (no invent bare-name Output past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp:539 — virtual Output for array (itemized or collective)
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return v.AsArray.OutputAccess()
	}
	name := v.GetActualName(prefixName)
	// Variable always has live get_actual_name; sticky no invent VOL_RVAL(, T) / ACCESS_ONCE()
	if name == "" {
		SetError(ErrGeneric)
		return ""
	}
	if v.UseVolRVal && v.IsVolatile() {
		// Variable.cpp:690–693 — type->Output always live; sticky no invent "int"
		if v.Type == nil {
			SetError(ErrGeneric)
			return ""
		}
		ty := v.Type.CName()
		if ty == "" {
			SetError(ErrGeneric)
			return ""
		}
		return "VOL_RVAL(" + name + ", " + ty + ")"
	}
	// Variable.cpp:694–696 — CGOptions::access_once() && isAccessOnce && !isAddrTaken
	// sticky when IsAccessOnce but option off (assert enabled; no invent silent skip wrap)
	if v.IsAccessOnce && !v.IsAddrTaken {
		if !ProcessOptions().AccessOnce {
			SetError(ErrGeneric)
			return name
		}
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
// Incomplete Variable sticky empty (no invent empty LHS soft-skip past hole).
func (v *Variable) OutputLhsCOpts(prefixName bool) string {
	// Variable always live at LHS emit; sticky incomplete no invent empty token
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky
	// (no invent bare-name LHS past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return v.AsArray.OutputAccess()
	}
	name := v.GetActualName(prefixName)
	// sticky no invent VOL_LVAL(, T) / empty LHS identifier
	if name == "" {
		SetError(ErrGeneric)
		return ""
	}
	if v.UseVolRVal && v.IsVolatile() {
		// Lhs/Variable type->Output always live; sticky no invent "int"
		if v.Type == nil {
			SetError(ErrGeneric)
			return ""
		}
		ty := v.Type.CName()
		if ty == "" {
			SetError(ErrGeneric)
			return ""
		}
		return "VOL_LVAL(" + name + ", " + ty + ")"
	}
	return name
}

// OutputAddrOf mirrors Variable::OutputAddrOf — always &actual_name (no VOL_RVAL).
// Variable.cpp:707–710.
func (v *Variable) OutputAddrOf(prefixName bool) string {
	// Variable* always live; sticky no invent "&0" for nil
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	name := v.GetActualName(prefixName)
	if name == "" {
		// sticky no invent bare "&"
		SetError(ErrGeneric)
		return ""
	}
	return "&" + name
}

// OutputForComment mirrors Variable::OutputForComment — bare actual name.
// Variable.cpp:711–713.
func (v *Variable) OutputForComment(prefixName bool) string {
	// Variable* always live at comment emit; sticky no invent empty comment name
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	name := v.GetActualName(prefixName)
	if name == "" {
		SetError(ErrGeneric)
		return ""
	}
	return name
}

// OutputUpperBound mirrors Variable::OutputUpperBound — field path for bounds.
// Variable.cpp:721–732; ArrayVariable.cpp:572–577 for arrays.
// IsArray without AsArray sticky empty (no invent bare-name upper bound past shell).
func (v *Variable) OutputUpperBound(prefixName bool) string {
	// Variable* always live at bound emit; sticky no invent empty upper bound
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky
	// (no invent scalar name as upper bound past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	if v.AsArray != nil && len(v.AsArray.Sizes) > 0 {
		return v.AsArray.OutputUpperBoundArray()
	}
	if v.FieldVarOf != nil {
		// Variable.cpp:724–727 — assert(dot != npos); sticky no invent base-only without ".fN"
		base := v.FieldVarOf.OutputUpperBound(prefixName)
		if base == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		dot := strings.LastIndex(v.Name, ".")
		if dot < 0 {
			SetError(ErrGeneric)
			return ""
		}
		return base + v.Name[dot:]
	}
	name := v.GetActualName(prefixName)
	if name == "" {
		SetError(ErrGeneric)
		return ""
	}
	return name
}

// OutputLowerBound mirrors Variable::OutputLowerBound.
// Variable.cpp:734–745. Arrays override separately.
// Incomplete Variable sticky empty (no invent empty bound soft-skip past hole).
// IsArray without AsArray sticky empty (no invent bare-name lower bound past shell).
func (v *Variable) OutputLowerBound(prefixName bool) string {
	// Variable always live at bound emit; sticky incomplete no invent empty token
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky
	// (no invent scalar name as lower bound past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	if v.AsArray != nil {
		return v.AsArray.OutputLowerBound()
	}
	if v.FieldVarOf != nil {
		// Variable.cpp:737–740 — assert(dot != npos); sticky no invent base-only without ".fN"
		base := v.FieldVarOf.OutputLowerBound(prefixName)
		if base == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		dot := strings.LastIndex(v.Name, ".")
		if dot < 0 {
			SetError(ErrGeneric)
			return ""
		}
		return base + v.Name[dot:]
	}
	name := v.GetActualName(prefixName)
	if name == "" {
		SetError(ErrGeneric)
		return ""
	}
	return name
}

// NewCtrlVars mirrors Variable::new_ctrl_vars — i,j,k… (±suffix when fresh).
// Variable.cpp:747–767. maxDim is CGOptions::max_array_dimensions() as-is
// (no soft invent maxDim=1 when option is 0 — empty loop / empty vector).
func NewCtrlVars(maxDim int, freshNames bool) []*Variable {
	if maxDim < 0 {
		maxDim = 0
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
// Incomplete last vector fails closed sticky IncompleteVariables (not bare nil invent
// empty-complete ctrl set / soft re-pick array inits past hole).
func GetLastCtrlVars() []*Variable {
	if len(ctrlVarsVectors) == 0 {
		return nil
	}
	last := ctrlVarsVectors[len(ctrlVarsVectors)-1]
	if !VariablesComplete(last) {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	return last
}

// CtrlVarsDoFinalization mirrors Variable::doFinalization for ctrl var pool.
// Variable.cpp:779–786.
func CtrlVarsDoFinalization() {
	ctrlVarsVectors = nil
	ctrlVarsCount = 0
}

// CtrlVarNames returns actual names of a ctrl-var slice.
// Variable* always live; incomplete list fails closed sticky IncompleteLabelsSlice
// (not bare nil invent empty-complete name list / soft re-pick past hole).
func CtrlVarNames(ctrl []*Variable) []string {
	if !VariablesComplete(ctrl) {
		SetError(ErrGeneric)
		return IncompleteLabelsSlice()
	}
	out := make([]string, len(ctrl))
	for i, v := range ctrl {
		name := v.GetActualName(false)
		if name == "" {
			// empty actual name is broken IR — fail closed sticky incomplete names
			SetError(ErrGeneric)
			return IncompleteLabelsSlice()
		}
		out[i] = name
	}
	return out
}

// OutputArrayCtrlVars mirrors OutputArrayCtrlVars — "int i, j, k;".
// Variable.cpp:800–811 — assert(dimen <= ctrl_vars.size()); get_actual_name only.
// Incomplete ctrl list fails closed sticky empty (no invent "int , j;" / empty-complete
// decl for nil slots via soft return "").
func OutputArrayCtrlVars(ctrl []*Variable, dimen int, indent string) string {
	// dimen<=0 or empty/undersized ctrl: config soft re-pick (non-sticky empty)
	// Variable.cpp:802 assert(dimen <= size) — soft empty when MaxArrayDim < rank
	// (sticky here poisons Generate when array rank exceeds MaxArrayDim)
	if dimen <= 0 || len(ctrl) == 0 {
		return ""
	}
	if dimen > len(ctrl) {
		return ""
	}
	// Variable.cpp:806 — ctrl_vars[i]->get_actual_name(); always live names
	if !VariablesComplete(ctrl[:dimen]) {
		SetError(ErrGeneric)
		return ""
	}
	for i := 0; i < dimen; i++ {
		// Variable.cpp:806 — get_actual_name always live; sticky no invent "int i, ;"
		if ctrl[i].GetActualName(false) == "" {
			SetError(ErrGeneric)
			return ""
		}
	}
	var b strings.Builder
	b.WriteString(indent + "int ")
	for i := 0; i < dimen; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(ctrl[i].GetActualName(false))
	}
	b.WriteString(";\n")
	return b.String()
}

// GetMaxArrayDimension mirrors Variable::GetMaxArrayDimension.
// Variable.cpp:813–826.
// Incomplete vars list fails closed sticky as -1 (no invent skip partial max /
// soft re-pick zero-dim success past holes). Complete empty / no arrays → 0.
// IsArray without AsArray sticky -1 (no invent dim from ArraySizes past broken shell).
func GetMaxArrayDimension(vars []*Variable) int {
	if !VariablesComplete(vars) {
		SetError(ErrGeneric)
		return -1
	}
	dimen := 0
	for _, v := range vars {
		if !v.IsArray {
			continue
		}
		// C++ ArrayVariable* always live when isArray; missing AsArray sticky
		// (no invent max-dim from ArraySizes alone past incomplete shell)
		if v.AsArray == nil {
			SetError(ErrGeneric)
			return -1
		}
		n := len(v.AsArray.Sizes)
		if n == 0 {
			n = len(v.ArraySizes)
		}
		if n > dimen {
			dimen = n
		}
	}
	return dimen
}

// OutputArrayInitializers mirrors OutputArrayInitializers for loop-init arrays.
// Variable.cpp:829–841 — allocate ctrl vars, declare, emit output_init.
// Incomplete vars list (GetMaxArrayDimension -1) fails closed sticky empty
// (no invent treat incomplete as zero-dim empty success).
func OutputArrayInitializers(vars []*Variable, opts Options, indent string) string {
	dimen := GetMaxArrayDimension(vars)
	// dimen < 0 = incomplete; dimen == 0 = complete empty / no arrays
	if dimen < 0 {
		SetError(ErrGeneric)
		return ""
	}
	if dimen == 0 {
		return ""
	}
	ctrl := GetNewCtrlVars(opts)
	// Variable.cpp:802 assert(dimen <= ctrl_vars.size()); no invent inits without decl
	// undersize MaxArrayDim is config soft re-pick (non-sticky); broken name/IR sticky inside OutputArrayCtrlVars
	decl := OutputArrayCtrlVars(ctrl, dimen, indent)
	if decl == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(decl)
	names := CtrlVarNames(ctrl)
	// vars pre-validated complete by GetMaxArrayDimension
	for _, v := range vars {
		if !v.IsArray {
			continue
		}
		// C++ static_cast ArrayVariable* when isArray; missing AsArray is broken IR
		// sticky (no invent synthetic shell from ArraySizes / soft re-pick partial inits)
		if v.AsArray == nil {
			SetError(ErrGeneric)
			return ""
		}
		av := v.AsArray
		// itemized dual-count members — skip (collective emits init once)
		if av.Collective != nil {
			continue
		}
		if av.NoLoopInitializer() {
			// residual ERROR sticky — no invent soft-skip then partial inits past hole
			if HasError() {
				return ""
			}
			continue
		}
		// residual ERROR sticky — no invent soft-continue past NoLoopInitializer hole
		if HasError() {
			return ""
		}
		initOut := av.OutputInit(indent, names)
		// incomplete loop-init IR sticky — fail closed whole initializers
		if initOut == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		b.WriteString(initOut)
	}
	return b.String()
}

// CreateVariableQfer mirrors Variable::CreateVariable(name, type, init, qfer).
// Variable.cpp:405–421 — caller supplies init (may be nil); expand aggregates.
func CreateVariableQfer(name string, typ *Type, qfer CVQualifiers) *Variable {
	return CreateVariableWithInit(name, typ, nil, qfer)
}

// CreateVariableWithInit mirrors Variable::CreateVariable(name, type, init, qfer).
// Variable.cpp:405–421.
func CreateVariableWithInit(name string, typ *Type, init *Constant, qfer CVQualifiers) *Variable {
	// Variable.cpp:412–414 — assert(type); assert simple != eVoid sticky
	// name always live from gensym/caller; empty name non-sticky soft factory gate
	if name == "" {
		return nil
	}
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	if typ.IsSimple() && typ.Simple() == EVoid {
		SetError(ErrGeneric)
		return nil
	}
	v := &Variable{
		Name: name,
		Type: typ,
		Qfer: qfer,
		Init: init,
	}
	if typ.IsAggregate() {
		v.CreateFieldVars()
	}
	// Variable.cpp:420 — ERROR_GUARD_AND_DEL1(nullptr, var)
	if HasError() {
		return nil
	}
	return v
}

// createVarRng mirrors process DefaultRndNumGenerator for CreateVariable init.
// Variable.cpp:395 Constant::make_random uses the process RNG stream.
// Nil when ProcessRng unset — fail closed (no invent private advancing NewRng stream).
func createVarRng() *Rng {
	return ProcessRng()
}

// CreateVariableScalars mirrors
// Variable::CreateVariable(name, type, isConst, isVolatile, …) for a scalar.
// Variable.cpp:368–402 — vectors of one bool each; init = Constant::make_random
// unless outermost container is union (Variable.cpp:395).
func CreateVariableScalars(name string, typ *Type, isConst, isVolatile bool) *Variable {
	// Variable.cpp:388–390 — assert(type); assert simple != eVoid sticky
	// name always live; empty name non-sticky soft factory gate
	if name == "" {
		return nil
	}
	if typ == nil {
		SetError(ErrGeneric)
		return nil
	}
	if typ.IsSimple() && typ.Simple() == EVoid {
		SetError(ErrGeneric)
		return nil
	}
	qfer := NewCVQualifiers([]bool{isConst}, []bool{isVolatile})
	// Variable.cpp:392–395 — non-union top: Constant::make_random(type); union top: 0
	// Constant::make_random reads process CGOptions + Probabilities singleton
	var init *Constant
	if !typ.IsUnion() {
		// process RNG + probs; nil probs → fail closed for aggregates
		init = MakeRandom(typ, ProcessOptions(), ProcessProbabilities(), createVarRng())
		// Variable.cpp:397 — ERROR_GUARD_AND_DEL1 when make_random fails / nullptr
		if HasError() || init == nil {
			return nil
		}
	}
	// Variable.cpp:397 — ERROR_GUARD_AND_DEL1(nullptr, var)
	if HasError() {
		return nil
	}
	v := &Variable{
		Name: name,
		Type: typ,
		Qfer: qfer,
		Init: init,
	}
	if typ.IsAggregate() {
		v.CreateFieldVars()
	}
	// Variable.cpp:401 — ERROR_GUARD_AND_DEL1 after create_field_vars
	if HasError() {
		return nil
	}
	return v
}

// IsGlobal mirrors Variable::is_global — name prefix "g_" (or field of global).
// Incomplete Variable sticky false (no invent not-global soft-skip past hole).
func (v *Variable) IsGlobal() bool {
	// Variable always live; sticky incomplete no invent not-global soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.FieldVarOf != nil {
		return v.FieldVarOf.IsGlobal()
	}
	// ArrayVariable.cpp:414–415 — parent == 0 (no owning block)
	if v.AsArray != nil {
		return v.AsArray.Block == nil
	}
	return len(v.Name) >= 2 && v.Name[0] == 'g' && v.Name[1] == '_'
}

// IsLocal mirrors Variable::is_local — name prefix "l_".
// Incomplete Variable sticky false (no invent not-local soft-skip past hole).
func (v *Variable) IsLocal() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	return len(v.Name) >= 2 && v.Name[0] == 'l' && v.Name[1] == '_'
}

// IsArgument mirrors Variable::is_argument — name prefix "p_".
// Incomplete Variable sticky false (no invent not-arg soft-skip past hole).
func (v *Variable) IsArgument() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	return len(v.Name) >= 2 && v.Name[0] == 'p' && v.Name[1] == '_'
}

// IsRV mirrors Variable::is_rv — return dummy name ends with "_rv".
// Incomplete Variable sticky false (no invent not-rv soft-skip past hole).
func (v *Variable) IsRV() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if len(v.Name) < 3 {
		return false
	}
	return v.Name[len(v.Name)-3:] == "_rv"
}

// IsTmpVar mirrors Variable::is_tmp_var — name prefix "t_".
// Variable.cpp:512–514.
// Incomplete Variable sticky false (no invent not-tmp soft-skip past hole).
func (v *Variable) IsTmpVar() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	return len(v.Name) >= 2 && v.Name[0] == 't' && v.Name[1] == '_'
}

// IsValidVolatile mirrors Variable::is_valid_volatile.
// Variable.cpp:1061–1072 — union fields recurse; const null pointer init invalid.
// Incomplete Variable/Type/init sticky false (no invent valid-volatile soft re-pick).
func (v *Variable) IsValidVolatile() bool {
	// Variable always live; sticky incomplete no invent valid-vol soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.IsInsideUnionField() {
		uv := v.GetContainerUnion()
		if uv == nil {
			// Type-nil ancestry / incomplete container sticky invalid
			// (use ancestryTypeHole — not residual global HasError alone)
			if ancestryTypeHole(v) {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return false
			}
			// complete IsInsideUnionField true but no container is broken IR sticky
			SetError(ErrGeneric)
			return false
		}
		return uv.IsValidVolatile()
	}
	// Variable.cpp:1068 — assert(init); non-const / non-zero / non-pointer → valid
	if !v.IsConst() {
		return true
	}
	if v.Type == nil {
		// incomplete type sticky invalid for const vol check
		SetError(ErrGeneric)
		return false
	}
	if !v.Type.IsPointerLike() {
		return true
	}
	// const pointer: invalid when init is null (equals 0)
	// C++ assert(init) — missing init sticky invalid (no invent valid true)
	if v.InitExpr != nil {
		return v.InitExpr.NotEquals(0)
	}
	if v.Init == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.Init.NotEqualsZero()
}

// IsPackedAggregateFieldVar mirrors Variable::is_packed_aggregate_field_var.
// Variable.cpp:307–312 — any ancestor field_var_of has packed aggregate type.
// Incomplete Variable sticky false for nil shell; Type-nil ancestor sticky true
// (restrictive — no invent not-packed soft-skip past incomplete parent type).
func (v *Variable) IsPackedAggregateFieldVar() bool {
	// Variable always live; sticky incomplete no invent not-packed soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	for p := v.FieldVarOf; p != nil; p = p.FieldVarOf {
		if p.Type == nil {
			// incomplete parent Type sticky packed (restrictive)
			SetError(ErrGeneric)
			return true
		}
		if p.Type.Packed {
			return true
		}
	}
	return false
}

// IsPackedAfterBitfield mirrors Variable::is_packed_after_bitfield.
// Variable.cpp:1240–1258 — packed struct field after a bitfield has unstable offset.
// Incomplete parent FieldVars sticky true (restrictive — no invent not-packed
// by soft-skipping FieldVars holes before this field).
func (v *Variable) IsPackedAfterBitfield() bool {
	// Variable always live; non-field complete false
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.FieldVarOf == nil {
		return false
	}
	parent := v.FieldVarOf
	// incomplete parent Type sticky packed-after (restrictive — no invent
	// not-packed soft-skip past Type-nil parent shell)
	if parent.Type == nil {
		SetError(ErrGeneric)
		return true
	}
	if parent.Type.IsStruct() && parent.Type.Packed {
		if !parent.FieldVarsComplete() {
			// incomplete parent FieldVars sticky packed-after (restrictive)
			SetError(ErrGeneric)
			return true
		}
		for i, f := range parent.FieldVars {
			if f == v {
				break
			}
			if parent.Type.IsBitfieldIndex(i) {
				return true
			}
			// incomplete sibling Type sticky packed-after (restrictive)
			if f.Type == nil {
				SetError(ErrGeneric)
				return true
			}
			if f.Type.HasBitfields() {
				return true
			}
		}
	}
	return parent.IsPackedAfterBitfield()
}

// IsArrayField mirrors Variable::is_array_field.
// Variable.cpp:270–277 — field of an array variable (or recursive).
// Incomplete Variable sticky false (no invent not-array-field soft-skip past hole).
// Parent IsArray without AsArray sticky true (restrictive — no invent not-array-field
// soft-skip past broken array parent shell).
func (v *Variable) IsArrayField() bool {
	// Variable always live; sticky incomplete no invent not-array-field soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.FieldVarOf == nil {
		return false
	}
	p := v.FieldVarOf
	// C++ isArray always ArrayVariable*; missing AsArray sticky true restrictive
	if p.IsArray {
		if p.AsArray == nil {
			SetError(ErrGeneric)
		}
		return true
	}
	if p.AsArray != nil {
		return true
	}
	return p.IsArrayField()
}

// GetDimension mirrors Variable::get_dimension (default 0) / ArrayVariable override.
// Variable.h:88 — virtual size_t get_dimension() const { return 0; }
// ArrayVariable — sizes.size().
// Incomplete Variable sticky 0 (no invent dim soft-skip past hole).
// IsArray without AsArray sticky 0 (no invent dim from ArraySizes past broken shell).
func (v *Variable) GetDimension() int {
	// Variable always live; sticky incomplete no invent dim 0 soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return 0
	}
	if v.AsArray != nil {
		return v.AsArray.Dimension()
	}
	// C++ base Variable returns 0; IsArray without AsArray is broken IR sticky
	// (no invent non-zero dim from ArraySizes / soft re-pick NeedNestedLoop past hole)
	if v.IsArray {
		SetError(ErrGeneric)
		return 0
	}
	return 0
}

// MatchVarName mirrors Variable::match_var_name.
// Variable.cpp:1205–1222 — name match, array Output text, or field recurse.
// MatchVarName mirrors Variable::match_var_name — name or field path.
// Variable* always live in FieldVars; nil hole fails closed (nil match).
func (v *Variable) MatchVarName(vname string) *Variable {
	// Variable* always live for name match; sticky no invent miss without it
	if v == nil {
		SetError(ErrGeneric)
		return nil
	}
	// empty query: soft miss (not incomplete subject IR)
	if vname == "" {
		return nil
	}
	if v.Name == vname {
		return v
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky miss
	// (no invent bare-name OutputC match past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return nil
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
		// FieldVars hole sticky — no invent soft-skip field hole to later match
		if f == nil {
			SetError(ErrGeneric)
			return nil
		}
		if m := f.MatchVarName(vname); m != nil {
			return m
		}
	}
	return nil
}

// IsSeenName mirrors Variable::is_seen_name.
// Variable.cpp:1048–1058 — name starts with seen+"[".
// Seen entries and subject name always live; empty entry invents prefix "[" and
// empty name invents not-seen soft-miss — sticky false (no invent complete membership).
func IsSeenName(seen []string, name string) bool {
	if name == "" {
		SetError(ErrGeneric)
		return false
	}
	for _, n := range seen {
		if n == "" {
			// incomplete seen entry sticky (no invent is-seen via bare "[" prefix)
			SetError(ErrGeneric)
			return false
		}
		prefix := n + "["
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// IsConst mirrors Variable::is_const → qfer is_const_after_deref(0).
// Incomplete Variable sticky false (no invent non-const soft-skip past hole).
func (v *Variable) IsConst() bool {
	// Variable always live; sticky incomplete no invent non-const soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.Qfer.IsConst()
}

// IsVolatile mirrors Variable::is_volatile.
// Incomplete Variable sticky false (no invent non-vol soft-skip past hole).
func (v *Variable) IsVolatile() bool {
	// Variable always live; sticky incomplete no invent non-vol soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.Qfer.IsVolatile()
}

// IsFieldVar mirrors Variable::is_field_var.
// Incomplete Variable sticky false (no invent not-field soft-skip past hole).
func (v *Variable) IsFieldVar() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.FieldVarOf != nil
}

// IsVisible mirrors Variable::is_visible — global or is_visible_local.
// Variable.h / usage in select_must_use_var.
// Incomplete Variable sticky false (no invent not-visible soft-skip past hole).
func (v *Variable) IsVisible(blk *Block) bool {
	// Variable always live; sticky incomplete no invent not-visible soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.IsGlobal() {
		return true
	}
	return v.IsVisibleLocal(blk)
}

// IsVisibleLocal mirrors Variable::is_visible_local / ArrayVariable override.
// Variable.cpp:482–503 — params + block-chain locals; fields recurse parent.
// ArrayVariable.cpp:419–429 — walk blk parents until array's parent block.
// Variable* always live; nil FieldVarOf/Param/Local holes sticky false
// (no invent not-visible soft-skip past holes).
func (v *Variable) IsVisibleLocal(blk *Block) bool {
	// Variable always live; sticky incomplete no invent not-visible soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if blk == nil {
		return v.IsGlobal()
	}
	if v.IsFieldVar() {
		// FieldVarOf always live for field vars; nil sticky fail closed
		if v.FieldVarOf == nil {
			SetError(ErrGeneric)
			return false
		}
		return v.FieldVarOf.IsVisibleLocal(blk)
	}
	// ArrayVariable.cpp:419–429 — parent block chain for array (collective or itemized)
	if v.AsArray != nil && v.AsArray.Block != nil {
		for b := blk; b != nil; b = b.Parent {
			if b == v.AsArray.Block {
				return true
			}
		}
		// still check local_vars membership below for itemized push_back
	}
	// params of blk's function
	f := blk.Func
	for b := blk; f == nil && b != nil; b = b.Parent {
		f = b.Func
	}
	if f != nil {
		for _, p := range f.Param {
			if p == nil {
				// incomplete Param sticky not-visible
				SetError(ErrGeneric)
				return false
			}
			if p == v {
				return true
			}
		}
	}
	for b := blk; b != nil; b = b.Parent {
		for _, loc := range b.LocalVars {
			if loc == nil {
				// incomplete LocalVars sticky not-visible
				SetError(ErrGeneric)
				return false
			}
			if loc == v {
				return true
			}
		}
	}
	return false
}

// IsConstAfterDeref mirrors Variable::is_const_after_deref.
// Variable.cpp:521–538.
// Incomplete type / OOB peel sticky const (no invent non-const WRITE / soft re-pick).
func (v *Variable) IsConstAfterDeref(derefLevel int) bool {
	// Variable always live; sticky incomplete no invent non-const soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if derefLevel < 0 {
		return false
	}
	if v.Qfer.IsConstAfterDeref(derefLevel) {
		return true
	}
	// incomplete type / OOB peel sticky const (restrictive)
	if v.Type == nil {
		SetError(ErrGeneric)
		return true
	}
	t := v.Type
	for i := 0; i < derefLevel; i++ {
		t = t.PtrType()
		// Variable.cpp:535 assert(t); broken peel → sticky fail closed const
		if t == nil {
			SetError(ErrGeneric)
			return true
		}
	}
	return t.IsConstStructUnion()
}

// IsVolatileAfterDeref mirrors Variable::is_volatile_after_deref.
// Variable.cpp:561–578 — qfer then peel type; assert(t) after peel.
// Incomplete type / OOB peel sticky volatile (no invent non-vol access / soft re-pick).
func (v *Variable) IsVolatileAfterDeref(derefLevel int) bool {
	// Variable always live; sticky incomplete no invent non-vol soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if derefLevel < 0 {
		return false
	}
	if v.Qfer.IsVolatileAfterDeref(derefLevel) {
		return true
	}
	if v.Type == nil {
		SetError(ErrGeneric)
		return true
	}
	t := v.Type
	for i := 0; i < derefLevel; i++ {
		t = t.PtrType()
		// Variable.cpp:575 assert(t); OOB peel → sticky fail closed volatile
		if t == nil {
			SetError(ErrGeneric)
			return true
		}
	}
	return t.IsVolatileStructUnion()
}

// IsPartialVolatileAfterDeref mirrors Variable::is_partial_volatile_after_deref.
// Variable.cpp:541–558 — not fully volatile at level, but pointee is volatile struct/union.
// Incomplete type / OOB peel sticky partial-vol (restrictive IsEligibleVar).
func (v *Variable) IsPartialVolatileAfterDeref(derefLevel int) bool {
	// Variable always live; sticky incomplete no invent not-partial-vol soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if derefLevel < 0 {
		return false
	}
	// whole type volatile at this deref → not "partial"
	if v.Qfer.IsVolatileAfterDeref(derefLevel) {
		return false
	}
	if v.Type == nil {
		SetError(ErrGeneric)
		return true
	}
	t := v.Type
	for i := 0; i < derefLevel; i++ {
		t = t.PtrType()
		// Variable.cpp:555 assert(t)
		if t == nil {
			SetError(ErrGeneric)
			return true
		}
	}
	return t.IsVolatileStructUnion()
}

// Compatible mirrors Variable::compatible.
// Variable.cpp:878–886 — no volatiles; same ptr; expand_struct only non-fields.
// Incomplete Variable* sticky false (no invent soft-skip compatibility past holes).
func (v *Variable) Compatible(other *Variable, expandStruct bool) bool {
	// both Variable* always live; sticky incomplete no invent soft-skip
	if v == nil || other == nil {
		SetError(ErrGeneric)
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
// Incomplete Variable sticky false (no invent not-pointer soft-skip past hole).
// Special null/garbage/tbd have Type nil by design — complete not-pointer.
// Other Type-nil shells sticky (no invent not-pointer soft-skip past incomplete IR).
func (v *Variable) IsPointer() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if IsSpecialPtr(v) {
		return false
	}
	if v.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.Type.PtrType() != nil
}

// IsVirtual mirrors Variable::is_virtual.
// Variable.cpp:280–288 — field recurses; array is virtual when collective==0
// (parent array, not itemized member). Dummy null/garbage/tbd use IsSpecialPtr / Type==nil.
// Incomplete Variable sticky false (no invent not-virtual soft-skip past hole).
// IsArray without AsArray sticky false (no invent virtual-collective past broken shell).
func (v *Variable) IsVirtual() bool {
	// Variable always live; sticky incomplete no invent not-virtual soft-skip
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.FieldVarOf != nil {
		return v.FieldVarOf.IsVirtual()
	}
	if v.AsArray != nil {
		// ArrayVariable::collective == 0 → parent / non-itemized
		return v.AsArray.Collective == nil
	}
	// C++ isArray always implies ArrayVariable*; missing AsArray is broken IR sticky
	// (no invent virtual-collective soft-success past incomplete shell)
	if v.IsArray {
		SetError(ErrGeneric)
		return false
	}
	return false
}

// IsAggregate mirrors Variable::is_aggregate.
// Incomplete Variable sticky false (no invent not-aggregate soft-skip past hole).
// Special null/garbage/tbd have Type nil by design — complete not-aggregate.
// Other Type-nil shells sticky (no invent not-aggregate soft-skip past incomplete IR).
func (v *Variable) IsAggregate() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if IsSpecialPtr(v) {
		return false
	}
	if v.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.Type.IsAggregate()
}

// MakeDummyStaticVariable mirrors VariableSelector::make_dummy_static_variable.
// VariableSelector.cpp:1565–1568 — name only, type null.
func MakeDummyStaticVariable(name string) *Variable {
	return &Variable{Name: name, Type: nil}
}

// GetCollective mirrors Variable::get_collective.
// Variable.cpp:581–615 — itemized array → parent; array field maps onto collective fields.
// Incomplete FieldVars / missing parent / OOB field path fails closed sticky nil
// (no invent return self as collective / soft re-pick past broken field map).
func (v *Variable) GetCollective() *Variable {
	if v == nil {
		SetError(ErrGeneric)
		return nil
	}
	// special handling for array fields (Variable.cpp:583–612)
	if v.IsArrayField() {
		// find top-level array ancestor
		parent := v.FieldVarOf
		for parent != nil && !parent.IsArray && parent.AsArray == nil {
			parent = parent.FieldVarOf
		}
		// Variable.cpp:589 assert(parent) — incomplete ancestry sticky
		if parent == nil {
			SetError(ErrGeneric)
			return nil
		}
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent self-collective past broken array ancestor shell)
		if parent.IsArray && parent.AsArray == nil {
			SetError(ErrGeneric)
			return nil
		}
		// incomplete field IR on path or parent sticky (no invent soft self-collective)
		if !v.FieldVarsComplete() || !parent.FieldVarsComplete() {
			SetError(ErrGeneric)
			return nil
		}
		// if parent is already collective parent, this field is on collective
		pColl := parent.GetCollective()
		if pColl == nil {
			// child already sticky when ancestry/fields incomplete
			if !HasError() {
				SetError(ErrGeneric)
			}
			return nil
		}
		if pColl == parent {
			return v
		}
		// map field path onto coll parent's fields (Variable.cpp:596–611)
		coll := pColl
		// build field index path from array ancestor down to v
		var path []int
		for cur := v; cur != nil && cur != parent; cur = cur.FieldVarOf {
			fid := cur.GetFieldID()
			// incomplete FieldVars → GetFieldID -1 — sticky (no invent self)
			if fid < 0 {
				SetError(ErrGeneric)
				return nil
			}
			path = append([]int{fid}, path...)
		}
		for _, idx := range path {
			// Variable.cpp:608 assert(index < coll->field_vars.size()) sticky
			if coll == nil || !coll.FieldVarsComplete() || idx < 0 || idx >= len(coll.FieldVars) {
				SetError(ErrGeneric)
				return nil
			}
			coll = coll.FieldVars[idx]
		}
		return coll
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky
	// (no invent self as collective past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return nil
	}
	// non-field: itemized array member → collective parent
	if v.AsArray != nil && v.AsArray.Collective != nil {
		return &v.AsArray.Collective.Variable
	}
	return v
}

// GetSeqNum mirrors Variable::get_seq_num — digits after first '_'.
// Variable.cpp:261–265 — assert('_' present); sticky no invent 0 on missing separator.
func (v *Variable) GetSeqNum() int {
	// Variable always live with gensym name; sticky incomplete no invent 0
	if v == nil {
		SetError(ErrGeneric)
		return -1
	}
	idx := strings.IndexByte(v.Name, '_')
	// assert(index != npos) sticky — no invent seq 0 without separator
	if idx < 0 || idx+1 >= len(v.Name) {
		SetError(ErrGeneric)
		return -1
	}
	return Str2Int(v.Name[idx+1:])
}

// Match mirrors Variable::match — identity, or aggregate has field.
// Variable.cpp:254–258.
// Incomplete Variable* sticky false (no invent not-match / soft re-pick past holes).
// Special null/garbage/tbd have Type nil by design — complete not-match (unless identity).
// Other Type-nil shells sticky (no invent not-match soft-skip past incomplete IR).
func (v *Variable) Match(other *Variable) bool {
	// both Variable* always live; sticky incomplete no invent not-match
	if v == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	if v == other {
		return true
	}
	if IsSpecialPtr(v) || IsSpecialPtr(other) {
		return false
	}
	if v.Type == nil || other.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.Type.IsAggregate() {
		return v.HasFieldVar(other)
	}
	return false
}

// FieldVarsComplete reports nested FieldVars have no nil holes.
// Incomplete aggregates must not invent not-has-field / not-match past a hole
// for mark_dead_var and similar; callers fail closed when false.
// Nil Variable is incomplete (false) — no invent empty-complete fields for missing shell.
func (v *Variable) FieldVarsComplete() bool {
	// nil Variable is incomplete shell (not complete empty fields)
	if v == nil {
		return false
	}
	for _, f := range v.FieldVars {
		if f == nil || !f.FieldVarsComplete() {
			return false
		}
	}
	return true
}

// HasFieldVar mirrors Variable::has_field_var — other is this or nested field.
// Variable* always live in FieldVars; incomplete shell/hole sticky false
// (no invent skip hole and match a later field / soft re-pick past holes).
func (v *Variable) HasFieldVar(other *Variable) bool {
	// both Variable* always live; sticky incomplete no invent not-has-field
	if v == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	if v == other {
		return true
	}
	if !v.FieldVarsComplete() {
		// incomplete FieldVars sticky (no invent not-has-field past hole)
		SetError(ErrGeneric)
		return false
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
// GetContainerUnion walks FieldVarOf to the enclosing union variable.
// Variable always live; sticky nil (no invent no-container soft-skip past hole).
// Special null/garbage/tbd have Type nil by design — complete no-container.
// Other Type-nil ancestor sticky nil (no invent skip hole and miss a later union parent).
func (v *Variable) GetContainerUnion() *Variable {
	if v == nil {
		SetError(ErrGeneric)
		return nil
	}
	// specials: Type nil by design, never inside a union container
	if IsSpecialPtr(v) {
		return nil
	}
	for p := v; p != nil; p = p.FieldVarOf {
		if IsSpecialPtr(p) {
			return nil
		}
		if p.Type == nil {
			// incomplete Type on ancestry sticky (no invent no-container past hole)
			SetError(ErrGeneric)
			return nil
		}
		if p.Type.IsUnion() {
			return p
		}
	}
	return nil
}

// LooseMatch mirrors Variable::loose_match.
// Variable.cpp:239–250 — match collective, or same container union.
// Incomplete GetCollective fails closed sticky false (no invent match / soft re-pick).
func (v *Variable) LooseMatch(other *Variable) bool {
	// both Variable* always live; sticky incomplete no invent not-loose-match
	if v == nil || other == nil {
		SetError(ErrGeneric)
		return false
	}
	me := v.GetCollective()
	you := other.GetCollective()
	// GetCollective may already sticky incomplete; incomplete collective no match
	if me == nil || you == nil {
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if me.Match(you) {
		return true
	}
	meU := me.GetContainerUnion()
	youU := you.GetContainerUnion()
	return meU != nil && youU != nil && meU == youU
}

// IsUnionField mirrors Variable::is_union_field — direct field of a union.
// Variable always live; sticky false (no invent not-union-field soft-skip past hole).
// Field with live parent but Type-nil parent is incomplete IR sticky (no invent
// not-union-field soft-skip past hole). Non-field (FieldVarOf nil) is complete false.
func (v *Variable) IsUnionField() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.FieldVarOf == nil {
		return false
	}
	if v.FieldVarOf.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	return v.FieldVarOf.Type.IsUnion()
}

// IsInsideUnionField mirrors Variable::is_inside_union_field.
// Variable always live; sticky false for nil shell.
// Type-nil parent on chain sticky true (restrictive — no invent not-inside
// soft-skip past incomplete union-field IR; IsUnionField alone stickies false).
func (v *Variable) IsInsideUnionField() bool {
	if v == nil {
		SetError(ErrGeneric)
		return false
	}
	for p := v; p != nil; p = p.FieldVarOf {
		if p.IsUnionField() {
			return true
		}
		// IsUnionField stickies Type-nil parent as false; incomplete ancestry
		// must not invent not-inside after that soft false
		if p.FieldVarOf != nil && !IsSpecialPtr(p.FieldVarOf) && p.FieldVarOf.Type == nil {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return true
		}
	}
	return false
}

// GetFieldID mirrors Variable::get_field_id — index in parent FieldVars, or -1.
// Variable.cpp:323–333.
// Variable always live; sticky -1 (no invent not-field soft-skip past hole).
// Variable* always live in FieldVars; incomplete parent FieldVars sticky -1
// (no invent skip hole and match a later sibling index / soft re-pick).
func (v *Variable) GetFieldID() int {
	if v == nil {
		SetError(ErrGeneric)
		return -1
	}
	// non-field is complete -1 (not a field)
	if v.FieldVarOf == nil {
		return -1
	}
	if !v.FieldVarOf.FieldVarsComplete() {
		// incomplete parent FieldVars sticky (no invent field index past hole)
		SetError(ErrGeneric)
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
// Variable* always live in FieldVars; nil hole fails closed sticky IncompleteVariables
// (not bare nil invent empty-complete no-pointer-fields / soft re-pick past hole).
// Type* always live for non-special fields; Type-nil sticky IncompleteVariables
// (no invent soft-skip shell as neither pointer nor aggregate then complete empty/partial).
// Complete empty (no pointer fields) returns non-nil empty slice.
func (v *Variable) FindPointerFields() []*Variable {
	if v == nil {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	out := make([]*Variable, 0)
	for _, f := range v.FieldVars {
		if f == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		// Type* always live for non-special field subjects; Type-nil sticky
		// (IsPointer/IsAggregate residual ERROR still soft-skips as complete none)
		if f.Type == nil {
			if !IsSpecialPtr(f) {
				SetError(ErrGeneric)
				return IncompleteVariables()
			}
			continue
		}
		if f.IsPointer() {
			out = append(out, f)
		} else if f.IsAggregate() {
			nested := f.FindPointerFields()
			// incomplete nested FieldVars (empty complete is non-nil [])
			if !VariablesComplete(nested) {
				// nested already SetError sticky
				if !HasError() {
					SetError(ErrGeneric)
				}
				return IncompleteVariables()
			}
			out = append(out, nested...)
		}
	}
	return out
}

// CreateFieldVars mirrors Variable::create_field_vars for structs.
// Variable.cpp:337–370 — names name.f0, name.f1; OR parent const/vol into field qfer.
// Incomplete field Types / make_random fail closed: FieldVars → IncompleteVariables
// (not bare nil — FieldVarsComplete(nil)==true invents empty-complete zero fields
// when type still has Fields / half-built list was wiped).
func (v *Variable) CreateFieldVars() {
	// Variable.cpp:338 — assert(type->is_aggregate()) sticky for live non-aggregate
	// Variable always live; sticky no invent soft-skip create past missing shell
	if v == nil {
		SetError(ErrGeneric)
		return
	}
	if v.Type == nil || !v.Type.IsAggregate() {
		SetError(ErrGeneric)
		return
	}
	if len(v.FieldVars) > 0 {
		return
	}
	// Variable.cpp:340 — assert(fields.size() == qfers_.size()); Go fields embed qfer
	isVol := v.IsVolatile()
	isConst := v.IsConst()
	j := 0
	fail := func() {
		// sticky ERROR so soft re-pick cannot invent empty-complete zero fields past hole
		SetError(ErrGeneric)
		v.FieldVars = IncompleteVariables()
	}
	for _, f := range v.Type.Fields {
		if f.Type == nil {
			// incomplete type IR — fail closed sticky clear (no invent partial fields)
			fail()
			return
		}
		// Type::is_unamed_padding — zero-length bitfield skipped (Variable.cpp:351–352)
		if f.BitWidth == 0 {
			continue
		}
		fname := v.Name + ".f" + itoa(j)
		j++
		// CVQualifiers length = IndirectLevel+1 (pointer levels + storage).
		// Empty field qfer → all-false at each level (no invent 1-bool then SanityCheck fail
		// → IncompleteVariables soft shell without sticky ERROR_GUARD).
		need := f.Type.IndirectLevel() + 1
		if need < 1 {
			need = 1
		}
		consts := append([]bool(nil), f.Qfer.IsConsts...)
		vols := append([]bool(nil), f.Qfer.IsVolatiles...)
		if len(consts) == 0 {
			consts = make([]bool, need)
		} else if len(consts) != need {
			// incomplete field qfer depth — fail closed sticky
			fail()
			return
		}
		if len(vols) == 0 {
			vols = make([]bool, need)
		} else if len(vols) != need {
			fail()
			return
		}
		// quals.set_const(is_const_var || quals.is_const()) on storage (last)
		if isConst {
			consts[len(consts)-1] = true
		}
		if isVol {
			vols[len(vols)-1] = true
		}
		// Variable.cpp:360–363 / 395 — CreateVariable; init = Constant::make_random
		// unless outermost container is union.
		top := v
		for top.FieldVarOf != nil {
			top = top.FieldVarOf
		}
		// outermost container Type always live; Type-nil sticky fail (no invent
		// make_random init path as if non-union past incomplete top shell)
		if top.Type == nil {
			fail()
			return
		}
		var init *Constant
		if !top.Type.IsUnion() {
			// Variable.cpp:395 — Constant::make_random via process CGOptions +
			// Probabilities + DefaultRndNumGenerator; no invent NewProbabilities /
			// separate NewRng stream
			init = MakeRandom(f.Type, ProcessOptions(), ProcessProbabilities(), createVarRng())
			// Variable.cpp:397 — ERROR_GUARD_AND_DEL1 when make_random nullptr
			if HasError() || init == nil {
				fail()
				return
			}
		}
		// Variable.cpp:397 — ERROR_GUARD during field CreateVariable
		if HasError() {
			fail()
			return
		}
		qfer := NewCVQualifiers(consts, vols)
		// Variable.cpp:363 — assert(var->qfer.sanity_check(var->type))
		if !qfer.SanityCheck(f.Type) {
			// fail closed — no soft invent bad-qfer field var
			fail()
			return
		}
		fv := &Variable{
			Name:       fname,
			Type:       f.Type,
			Qfer:       qfer,
			FieldVarOf: v,
			// bitfields_length_[i] >= 0 → isBitfield (Type::is_bitfield)
			IsBitfield: f.BitWidth >= 0,
			Init:       init,
		}
		// recursive expand nested structs
		if f.Type.IsAggregate() {
			fv.CreateFieldVars()
		}
		if HasError() {
			fail()
			return
		}
		v.FieldVars = append(v.FieldVars, fv)
	}
}

// OutputValueDump mirrors Variable::output_value_dump.
// Variable.cpp:1173–1203 — printf checksum lines for simples; recurse aggregates;
// arrays expand all index combinations; unions only readable fields.
func (v *Variable) OutputValueDump(prefix string, indent int, unionFacts []*FactUnion) string {
	// Variable + Type always live at dump emit; sticky no invent empty dump shell
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky (no invent scalar
	// dump / soft-skip expand past broken array shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	// Variable.cpp:1175–1183 — is_virtual() → assert(!is_field_var()); expand array
	if v.IsVirtual() {
		// field of virtual array is broken IR sticky for this path
		if v.IsFieldVar() {
			SetError(ErrGeneric)
			return ""
		}
		if v.IsArray || (v.AsArray != nil && len(v.AsArray.Sizes) > 0) {
			return outputValueDumpArray(v, prefix, indent, unionFacts)
		}
	}
	if v.Type.IsSimple() {
		// Variable.cpp:1184–1188 — name + printf_directive always live sticky
		// no invent printf with empty name/directive
		name := v.GetActualName(false)
		dir := v.Type.PrintfDirective()
		if name == "" || dir == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		return OutputTab(indent) + "printf(\"" + prefix + name + " = " + dir + "\\n\", " + name + ");\n"
	}
	if v.Type.IsStruct() {
		// incomplete FieldVars sticky whole dump (no invent soft-skip hole)
		if !v.FieldVarsComplete() {
			SetError(ErrGeneric)
			return ""
		}
		var b strings.Builder
		for _, f := range v.FieldVars {
			part := f.OutputValueDump(prefix, indent, unionFacts)
			// residual ERROR sticky — no invent partial field dump past hard IR hole
			if HasError() {
				return ""
			}
			b.WriteString(part)
		}
		return b.String()
	}
	if v.Type.IsUnion() {
		// incomplete FieldVars sticky whole dump
		if !v.FieldVarsComplete() {
			SetError(ErrGeneric)
			return ""
		}
		var b strings.Builder
		for i, f := range v.FieldVars {
			// Variable.cpp:1195–1200 — FactUnion::is_field_readable (program end facts)
			if !IsFieldReadable(v, i, unionFacts) {
				// residual ERROR sticky — no invent soft-skip then partial dump past hole
				if HasError() {
					return ""
				}
				continue
			}
			// residual ERROR sticky — no invent soft-continue past IsFieldReadable hole
			if HasError() {
				return ""
			}
			part := f.OutputValueDump(prefix, indent, unionFacts)
			// residual ERROR sticky — no invent partial field dump past hard IR hole
			if HasError() {
				return ""
			}
			b.WriteString(part)
		}
		return b.String()
	}
	// pointers: dump as pointer directive
	if v.Type.IsPointerLike() {
		name := v.GetActualName(false)
		dir := v.Type.PrintfDirective()
		if name == "" || dir == "" {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return ""
		}
		return OutputTab(indent) + "printf(\"" + prefix + name + " = " + dir + "\\n\", " + name + ");\n"
	}
	return ""
}

// outputValueDumpArray expands array into all index combinations and dumps elements.
// outputValueDumpArray expands array into all index combinations and dumps elements.
// Variable always live; sticky empty (no invent soft-skip dump past hole).
// Zero-rank ArraySizes is complete empty (not incomplete IR).
func outputValueDumpArray(v *Variable, prefix string, indent int, unionFacts []*FactUnion) string {
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	if len(v.ArraySizes) == 0 {
		return ""
	}
	// base name always live sticky; no invent printf with bare "[0]" access
	base := v.GetActualName(false)
	if base == "" {
		SetError(ErrGeneric)
		return ""
	}
	// Type always live for dump dispatch; sticky no invent empty dump past Type-nil shell
	if v.Type == nil {
		SetError(ErrGeneric)
		return ""
	}
	all := expandWithinRanges(v.ArraySizes)
	var b strings.Builder
	for _, idx := range all {
		// build access name g_a[0][1]
		name := base
		for _, i := range idx {
			name += "[" + itoa(i) + "]"
		}
		if v.Type.IsSimple() {
			dir := v.Type.PrintfDirective()
			if dir == "" {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return ""
			}
			b.WriteString(OutputTab(indent) + "printf(\"" + prefix + name + " = " + dir + "\\n\", " + name + ");\n")
			continue
		}
		if v.Type.IsAggregate() && len(v.FieldVars) > 0 {
			// dump fields with indexed prefix path via synthetic names
			// Variable* always live in FieldVars; nil hole sticky whole dump
			for fi, f := range v.FieldVars {
				if f == nil || f.Type == nil {
					SetError(ErrGeneric)
					return ""
				}
				if v.Type.IsUnion() && !IsFieldReadable(v, fi, unionFacts) {
					// residual ERROR sticky — no invent soft-skip then partial dump past hole
					if HasError() {
						return ""
					}
					continue
				}
				// residual ERROR sticky — no invent soft-continue past IsFieldReadable hole
				if HasError() {
					return ""
				}
				if !f.Type.IsSimple() {
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
				if dir == "" {
					continue
				}
				b.WriteString(OutputTab(indent) + "printf(\"" + prefix + acc + " = " + dir + "\\n\", " + acc + ");\n")
			}
		}
	}
	return b.String()
}

// expandWithinRanges mirrors util.cpp expand_within_ranges — all index vectors
// in [0,size) for each dimension. util.cpp:111–137: product of sizes; a zero
// (or empty) dimension yields empty out. No soft invent size 1 for n < 1.
func expandWithinRanges(sizes []int) [][]int {
	if len(sizes) == 0 {
		return nil
	}
	// util.cpp: unsigned sizes; product 0 → empty expansion (no invent).
	for _, n := range sizes {
		if n < 1 {
			return nil
		}
	}
	// util.cpp: limits[dim-1] = in[dim-1]; limits[i] = limits[i+1]*in[i]
	dim := len(sizes)
	limits := make([]int, dim)
	limits[dim-1] = sizes[dim-1]
	for i := dim - 2; i >= 0; i-- {
		limits[i] = limits[i+1] * sizes[i]
	}
	out := make([][]int, 0, limits[0])
	for i := 0; i < limits[0]; i++ {
		tmp := make([]int, 0, dim)
		num := i
		for j := 0; j < dim-1; j++ {
			tmp = append(tmp, num/limits[j+1])
			num = num % limits[j+1]
		}
		tmp = append(tmp, num)
		out = append(out, tmp)
	}
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

// itoa formats n in decimal (including negative for sizes[i]-1 upper bounds).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if neg {
		d = append([]byte{'-'}, d...)
	}
	return string(d)
}

// CollectExpandable returns v plus all field_vars recursively (expand_struct_union_vars-ish).
// CollectExpandable walks field_vars for selectable members.
// Variable* always live in FieldVars; nil hole fails closed IncompleteVariables
// (not bare nil — VariablesComplete(nil)==true invents empty expand success).
// Special null/garbage/tbd have Type nil by design — complete expand as self only.
// Other Type-nil shells sticky IncompleteVariables (no invent expand pool past hole).
func (v *Variable) CollectExpandable() []*Variable {
	if v == nil {
		// incomplete subject fails closed sticky (no invent empty expand pool)
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	if !IsSpecialPtr(v) && v.Type == nil {
		SetError(ErrGeneric)
		return IncompleteVariables()
	}
	out := []*Variable{v}
	for _, f := range v.FieldVars {
		if f == nil {
			SetError(ErrGeneric)
			return IncompleteVariables()
		}
		nested := f.CollectExpandable()
		if !VariablesComplete(nested) {
			if !HasError() {
				SetError(ErrGeneric)
			}
			return IncompleteVariables()
		}
		out = append(out, nested...)
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
	// Variable + Type always live at hash emit; sticky incomplete no invent empty hash
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return ""
	}
	// C++ isArray always ArrayVariable*; missing AsArray sticky (no invent
	// hash-array expand from ArraySizes alone past broken shell)
	if v.IsArray && v.AsArray == nil {
		SetError(ErrGeneric)
		return ""
	}
	if v.IsArray && len(v.ArraySizes) > 0 {
		return hashArrayVariable(v, ctrl, unionFacts)
	}
	if v.Type.IsAggregate() {
		// incomplete FieldVars fails closed sticky whole hash (no invent soft-skip hole)
		if !v.FieldVarsComplete() {
			SetError(ErrGeneric)
			return ""
		}
		var b strings.Builder
		for i, f := range v.FieldVars {
			// Variable.cpp:893–898 — skip unreadable union fields
			if v.Type.IsUnion() && unionFacts != nil {
				if !IsFieldReadable(v, i, unionFacts) {
					// residual ERROR sticky — no invent soft-skip then partial hash past hole
					if HasError() {
						return ""
					}
					continue
				}
				// residual ERROR sticky — no invent soft-continue past IsFieldReadable hole
				if HasError() {
					return ""
				}
			}
			part := f.hashOutput(ctrl, unionFacts)
			// residual ERROR sticky — no invent partial field hash past hard IR hole
			if HasError() {
				return ""
			}
			b.WriteString(part)
		}
		return b.String()
	}
	if v.Type.IsSimple() {
		// Variable.cpp:900–920 — name always live sticky; no invent empty transparent_crc
		name := v.GetActualName(false)
		if name == "" || v.Name == "" {
			SetError(ErrGeneric)
			return ""
		}
		if v.Type.IsFloat() {
			return "    transparent_crc_bytes (&" + name + ", sizeof(" + name + "), \"" + v.Name + "\", print_hash_value);\n"
		}
		return "    transparent_crc(" + name + ", \"" + v.Name + "\", print_hash_value);\n"
	}
	// ePointer: no hash (Variable.cpp:921–922)
	return ""
}

// hashArrayHasPayload reports whether array hashing would emit any transparent_crc.
// Type* always live on Fields; nil hole sticky false (no invent has-payload / soft re-pick).
func hashArrayHasPayload(v *Variable) bool {
	// Variable + Type always live; sticky incomplete no invent no-payload soft-skip
	if v == nil || v.Type == nil {
		SetError(ErrGeneric)
		return false
	}
	if v.Type.IsSimple() {
		return true
	}
	if v.Type.IsAggregate() {
		j := 0
		for _, f := range v.Type.Fields {
			if f.Type == nil {
				// incomplete field Type sticky no-payload (restrictive — no invent payload)
				SetError(ErrGeneric)
				return false
			}
			if f.BitWidth == 0 {
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
// ArrayVariable.cpp:763 — get_last_ctrl_vars only (no letter-name soft-fallback).
// hashArrayVariable emits looped transparent_crc for array elements.
// Variable always live; sticky empty (no invent soft-skip hash past hole).
// Zero-rank / no payload is complete empty (not incomplete IR shell).
func hashArrayVariable(v *Variable, ctrl []*Variable, unionFacts []*FactUnion) string {
	if v == nil {
		SetError(ErrGeneric)
		return ""
	}
	if len(v.ArraySizes) == 0 || !hashArrayHasPayload(v) {
		return ""
	}
	if ctrl == nil {
		ctrl = GetLastCtrlVars()
	}
	if len(ctrl) < len(v.ArraySizes) {
		// C++ assumes last_ctrl_vars sized for dimension after OutputArrayInitializers sticky
		SetError(ErrGeneric)
		return ""
	}
	// array name always live sticky; no invent transparent_crc([i], …) / for ( = 0; …)
	access := v.GetActualName(false)
	if access == "" {
		SetError(ErrGeneric)
		return ""
	}
	names := make([]string, len(v.ArraySizes))
	for i := range v.ArraySizes {
		if ctrl[i] == nil {
			// incomplete ctrl IR sticky — no invent empty index id
			SetError(ErrGeneric)
			return ""
		}
		names[i] = ctrl[i].GetActualName(false)
		if names[i] == "" {
			// ctrl get_actual_name always live sticky; no invent empty index id
			SetError(ErrGeneric)
			return ""
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
	nameStr := access
	for _, iv := range names {
		access += "[" + iv + "]"
		nameStr += "[" + iv + "]"
	}
	if v.Type != nil && v.Type.IsAggregate() {
		j := 0
		for i, f := range v.Type.Fields {
			// Type* always live; nil hole fails closed sticky (no invent skip partial
			// hash / soft re-pick past incomplete field type as empty success)
			if f.Type == nil {
				SetError(ErrGeneric)
				return ""
			}
			if f.BitWidth == 0 {
				continue
			}
			// ArrayVariable.cpp:741–752 — skip unreadable union fields
			if v.Type.IsUnion() && unionFacts != nil {
				if !IsFieldReadable(v, i, unionFacts) {
					// residual ERROR sticky — no invent soft-skip then partial array hash past hole
					if HasError() {
						return ""
					}
					j++
					continue
				}
				// residual ERROR sticky — no invent soft-continue past IsFieldReadable hole
				if HasError() {
					return ""
				}
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
