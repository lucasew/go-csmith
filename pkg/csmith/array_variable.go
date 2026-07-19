// Upstream: ArrayVariable.h / ArrayVariable.cpp (CreateArrayVariable).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// ArrayVariable mirrors ArrayVariable : Variable with dimension sizes.
type ArrayVariable struct {
	Variable
	// Sizes is the per-dimension lengths (ArrayVariable::sizes).
	Sizes []int
	// InitExprs mirrors ArrayVariable::init_values (Expression* alt inits).
	// Fact abstraction and emit use these; no invent Constant-from-string for
	// make_init_value results (e.g. &g_x).
	InitExprs []*Expression
	// InitValues are to_string() of InitExprs for brace emit / legacy tests.
	InitValues []string
	// Block is the owning block for locals (nil if global).
	Block *Block
	// Collective is non-nil for itemized members (points at parent array).
	Collective *ArrayVariable
	// Indices are constant index strings for itemized access (emit / name form).
	Indices []string
	// IndexExprs mirrors ArrayVariable::indices (Expression* per dimension).
	// Used by CGContext::read_indices / Lhs::visit_indices / modified-index DFA.
	IndexExprs []*Expression
}

// CreateArrayVariable mirrors ArrayVariable::CreateArrayVariable.
// ArrayVariable.cpp:123–193 — dimension distribution, size caps, alt init_values.
// probs is session Probabilities (C++ singleton); no invent NewProbabilities(opts).
// vs+cg are required for pointer alt-inits via make_init_value when
// !strict_const_arrays (ArrayVariable.cpp:179–184); nil → fail closed that branch
// (no invent Constant "0" as a soft stand-in for make_init_value).
func CreateArrayVariable(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	cg *CGContext,
	blk *Block,
	name string,
	elem *Type,
	init *Constant,
	qfer CVQualifiers,
) *ArrayVariable {
	// ArrayVariable.cpp:127–129 — assert(type); assert simple != eVoid
	if r == nil || elem == nil {
		return nil
	}
	if elem.IsSimple() && elem.Simple() == EVoid {
		return nil
	}
	// dimension: 1d 60%, 2d 30%, … via rnd_upto(99)+1 stepping
	// ArrayVariable.cpp:131–144
	num := int(r.RndUpto(99)) + 1
	// ArrayVariable.cpp:133 — ERROR_GUARD(nullptr)
	if HasError() {
		return nil
	}
	dimension := 0
	step := 100
	for num > 0 {
		dimension++
		step /= 2
		if step == 0 {
			step = 1
		}
		num -= step
	}
	// ArrayVariable.cpp:142–144 — clamp max only; no soft invent dimension=1 when max is 0
	if dimension > opts.MaxArrayDim {
		dimension = opts.MaxArrayDim
	}
	// ArrayVariable.cpp:146–158 — only push when dimen_size > 0 after clamp;
	// empty sizes allowed (no soft invent sizes=[1])
	sizes := make([]int, 0, dimension)
	total := 1
	for i := 0; i < dimension; i++ {
		dimen := int(r.RndUpto(uint32(opts.MaxArrayLenPerDim))) + 1
		// ArrayVariable.cpp:149–150 — rnd_upto(max_len_per_dim)+1; ERROR_GUARD
		if HasError() {
			return nil
		}
		if opts.MaxArrayLength > 0 && total*dimen > opts.MaxArrayLength {
			dimen = opts.MaxArrayLength / total
		}
		// ArrayVariable.cpp:154–157 — if (dimen_size) push; else skip (may leave sizes empty)
		if dimen < 1 {
			continue
		}
		total *= dimen
		sizes = append(sizes, dimen)
	}
	av := &ArrayVariable{
		Variable: Variable{
			Name:       name,
			Type:       elem, // element type
			Qfer:       qfer,
			IsArray:    true,
			Init:       init,
			ArraySizes: sizes,
		},
		Sizes: sizes,
		Block: blk,
	}
	// ArrayVariable.cpp:161 — ERROR_GUARD_AND_DEL1 after new ArrayVariable
	if HasError() {
		return nil
	}
	// self-link for ChooseOKVar itemize (VariableSelector.cpp:332–337)
	av.AsArray = av
	// ArrayVariable.cpp:161–163 — create_field_vars for aggregate element type
	if elem.IsAggregate() {
		av.CreateFieldVars()
	}
	if HasError() {
		return nil
	}
	// ArrayVariable.cpp:166 — pure_rnd_upto(total_size/2); pure_rnd_upto(0)==0
	// no soft invent half=1 when total_size/2 is 0
	half := uint32(total / 2)
	initNum := int(r.RndUpto(half))
	for i := 0; i < initNum; i++ {
		// ArrayVariable.cpp:177–185
		// if (!pointer || strict_const_arrays) Constant::make_random
		// else VariableSelector::make_init_value
		var e *Expression
		if !elem.IsPointerLike() || opts.StrictConstArrays {
			c := MakeRandom(elem, opts, probs, r)
			if HasError() {
				return nil
			}
			if c != nil {
				e = &Expression{Term: TermConstant, Con: c, ExprType: elem}
			}
		} else {
			// make_init_value needs live VS + CGContext (C++ always has both)
			if vs == nil || cg == nil {
				// no invent Constant "0" / null stand-in for missing make_init_value
				return nil
			}
			qf := qfer
			e = vs.MakeInitValue(AccessRead, *cg, elem, &qf, blk, r)
			if HasError() {
				return nil
			}
		}
		// ArrayVariable.cpp:185 — add_init_value(e); skip null (no invent shell)
		if e == nil {
			continue
		}
		av.InitExprs = append(av.InitExprs, e)
		// ArrayVariable.cpp:505–506 — init_values[i]->to_string() for brace emit
		if val := e.Output(); val != "" {
			av.InitValues = append(av.InitValues, val)
			av.ArrayInits = append(av.ArrayInits, val)
		}
	}
	// ArrayVariable.cpp:190–191 — blk? local_vars : GetGlobalVariables()
	// no soft invent skip registration (GenerateNewGlobal still pushes itemized member)
	if blk != nil {
		blk.LocalVars = append(blk.LocalVars, &av.Variable)
	} else if vs != nil {
		vs.GlobalList = append(vs.GlobalList, &av.Variable)
	}
	return av
}

// Dimension returns get_dimension.
func (av *ArrayVariable) Dimension() int {
	if av == nil {
		return 0
	}
	return len(av.Sizes)
}

// TotalSize is product of sizes.
func (av *ArrayVariable) TotalSize() int {
	if av == nil {
		return 0
	}
	n := 1
	for _, s := range av.Sizes {
		n *= s
	}
	return n
}

// IsGlobal for arrays: name prefix g_ (same as Variable).
func (av *ArrayVariable) IsGlobal() bool {
	if av == nil {
		return false
	}
	return av.Variable.IsGlobal()
}

// CDeclType returns C type with dimensions, e.g. int x[2][3].
func (av *ArrayVariable) CDeclType() string {
	// ArrayVariable / Variable decl always has live type; no invent "int"
	if av == nil || av.Type == nil {
		return ""
	}
	cn := av.Type.CName()
	if cn == "" || av.Name == "" {
		return ""
	}
	var b strings.Builder
	if av.IsConst() {
		b.WriteString("const ")
	}
	if av.IsVolatile() {
		b.WriteString("volatile ")
	}
	b.WriteString(cn)
	b.WriteString(" ")
	b.WriteString(av.Name)
	for _, s := range av.Sizes {
		b.WriteString(fmt.Sprintf("[%d]", s))
	}
	return b.String()
}

// NoLoopInitializer mirrors ArrayVariable::no_loop_initializer.
// ArrayVariable.cpp:429–435 — struct/union, const, global, or multi init_values.
func (av *ArrayVariable) NoLoopInitializer() bool {
	if av == nil || av.Type == nil {
		return true
	}
	return av.Type.IsAggregate() || av.IsConst() || av.IsGlobal() || len(av.InitValues) > 0
}

// CountExprKeyVar mirrors count_expr_key_var (ArrayVariable.cpp:66–90).
// Variable: 1; Constant: 0; user call: 2; unary: recurse; binary: sum.
func CountExprKeyVar(e *Expression) int {
	if e == nil {
		// ArrayVariable.cpp:89 assert(0) for unexpected; nil is broken IR
		return 0
	}
	switch e.Term {
	case TermVariable:
		return 1
	case TermConstant:
		return 0
	case TermFunction:
		if e.Invoke == nil {
			return 0
		}
		// user call → 2 (ArrayVariable.cpp:76–77)
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			return 2
		}
		n := len(e.Invoke.Args)
		if n == 1 {
			return CountExprKeyVar(e.Invoke.Args[0])
		}
		// ArrayVariable.cpp:84 — assert(param_value.size() == 2) for binary
		if n == 2 {
			return CountExprKeyVar(e.Invoke.Args[0]) + CountExprKeyVar(e.Invoke.Args[1])
		}
		// wrong arity — fail closed 0 (no invent sum of first two)
		return 0
	default:
		// ArrayVariable.cpp:89 assert(0)
		return 0
	}
}

// FindExprKeyVar mirrors find_expr_key_var (ArrayVariable.cpp:98–119).
// Sole key variable of an index expression, or nil if none / ambiguous.
func FindExprKeyVar(e *Expression) *Variable {
	if e == nil {
		return nil
	}
	switch e.Term {
	case TermVariable:
		return e.Var
	case TermFunction:
		if e.Invoke == nil || (e.Invoke.User != nil && !e.Invoke.IsStd) {
			return nil
		}
		n := len(e.Invoke.Args)
		if n == 1 {
			return FindExprKeyVar(e.Invoke.Args[0])
		}
		// ArrayVariable.cpp:110 — assert(param_value.size() == 2)
		if n == 2 {
			v0 := FindExprKeyVar(e.Invoke.Args[0])
			v1 := FindExprKeyVar(e.Invoke.Args[1])
			if v0 == nil && v1 != nil {
				return v1
			}
			if v0 != nil && v1 == nil {
				return v0
			}
			// two vars or none → nil
			return nil
		}
	}
	return nil
}

// IsVariant mirrors ArrayVariable::is_variant.
// ArrayVariable.cpp:394–412 — same collective; each dim has exactly one key var
// and those key vars match across the two itemized members.
func (av *ArrayVariable) IsVariant(other *Variable) bool {
	if av == nil || other == nil || !other.IsArray {
		return false
	}
	ov := other.AsArray
	if ov == nil {
		return false
	}
	// both must be itemized (collective non-nil) under the same collective
	if av.Collective == nil || ov.Collective == nil {
		return false
	}
	if av.Collective != ov.Collective {
		return false
	}
	// prefer IndexExprs (Expression*); fall back to Indices arity
	n := len(av.IndexExprs)
	if n == 0 {
		n = len(av.Indices)
	}
	on := len(ov.IndexExprs)
	if on == 0 {
		on = len(ov.Indices)
	}
	if n == 0 || n != on {
		return false
	}
	for i := 0; i < n; i++ {
		var e, oe *Expression
		if i < len(av.IndexExprs) {
			e = av.IndexExprs[i]
		}
		if i < len(ov.IndexExprs) {
			oe = ov.IndexExprs[i]
		}
		if e != nil && oe != nil {
			// ArrayVariable.cpp:403–405 — Expression path
			if CountExprKeyVar(e) != 1 || CountExprKeyVar(oe) != 1 {
				return false
			}
			if FindExprKeyVar(e) != FindExprKeyVar(oe) {
				return false
			}
			continue
		}
		// string-only Indices: equal strings share key identity (no Variable* handle)
		si, sj := "", ""
		if i < len(av.Indices) {
			si = av.Indices[i]
		}
		if i < len(ov.Indices) {
			sj = ov.Indices[i]
		}
		if si != sj {
			return false
		}
	}
	return true
}

// ItemizeConstIndices mirrors ArrayVariable::itemize(const vector<int>&).
// ArrayVariable.cpp:280–295 — fixed const indices; create_field_vars for aggregates.
func (av *ArrayVariable) ItemizeConstIndices(constIndices []int, vs *VariableSelector) *ArrayVariable {
	if av == nil || av.Collective != nil {
		return nil
	}
	if len(constIndices) != len(av.Sizes) {
		return nil
	}
	item := &ArrayVariable{
		Variable: Variable{
			Name:       av.Name,
			Type:       av.Type,
			Qfer:       av.Qfer,
			IsArray:    true,
			Init:       av.Init,
			InitExpr:   av.InitExpr,
			ArraySizes: av.Sizes,
			ArrayInits: av.ArrayInits,
		},
		Sizes:      append([]int(nil), av.Sizes...),
		InitExprs:  append([]*Expression(nil), av.InitExprs...),
		InitValues: av.InitValues,
		Block:      av.Block,
		Collective: av,
	}
	item.AsArray = item
	for _, idx := range constIndices {
		s := fmt.Sprintf("%d", idx)
		item.Indices = append(item.Indices, s)
		item.IndexExprs = append(item.IndexExprs, &Expression{
			Term: TermConstant, Con: MakeInt(idx), ExprType: GetIntType(),
		})
	}
	if item.Type != nil && item.Type.IsAggregate() {
		item.CreateFieldVars()
	}
	if vs != nil {
		vs.AllVars = append(vs.AllVars, &item.Variable)
	}
	return item
}

// SetIndex mirrors ArrayVariable::set_index (string form for emit).
// ArrayVariable.cpp:229–231 — indices[index] = e; no soft invent "0" padding.
func (av *ArrayVariable) SetIndex(index int, expr string) {
	if av == nil || index < 0 {
		return
	}
	// grow with empty slots (C++ assumes index already valid; no invent constants)
	for len(av.Indices) <= index {
		av.Indices = append(av.Indices, "")
	}
	av.Indices[index] = expr
	// keep IndexExprs in sync: constant string → TermConstant
	for len(av.IndexExprs) <= index {
		av.IndexExprs = append(av.IndexExprs, nil)
	}
	av.IndexExprs[index] = &Expression{
		Term: TermConstant, Con: &Constant{Value: expr, Type: GetIntType()}, ExprType: GetIntType(),
	}
}

// SetIndexExpr mirrors ArrayVariable::set_index(size_t, const Expression*).
// ArrayVariable.cpp:229–231 — stores Expression*; emit string from Output only.
func (av *ArrayVariable) SetIndexExpr(index int, e *Expression) {
	if av == nil || index < 0 {
		return
	}
	for len(av.IndexExprs) <= index {
		av.IndexExprs = append(av.IndexExprs, nil)
	}
	av.IndexExprs[index] = e
	// no soft invent "0" for nil/empty Output (C++ uses Expression* directly)
	s := ""
	if e != nil {
		s = e.Output()
	}
	for len(av.Indices) <= index {
		av.Indices = append(av.Indices, "")
	}
	av.Indices[index] = s
}

// AddIndex mirrors ArrayVariable::add_index (string helper).
func (av *ArrayVariable) AddIndex(expr string) {
	if av == nil {
		return
	}
	av.Indices = append(av.Indices, expr)
	av.IndexExprs = append(av.IndexExprs, &Expression{
		Term: TermConstant, Con: &Constant{Value: expr, Type: GetIntType()}, ExprType: GetIntType(),
	})
}

// AddIndexExpr appends an index Expression (ArrayVariable::add_index).
// ArrayVariable.cpp:227 — indices.push_back(e); no soft invent "0".
func (av *ArrayVariable) AddIndexExpr(e *Expression) {
	if av == nil {
		return
	}
	av.IndexExprs = append(av.IndexExprs, e)
	s := ""
	if e != nil {
		s = e.Output()
	}
	av.Indices = append(av.Indices, s)
}

// buildInitRecursive mirrors ArrayVariable::build_init_recursive.
// ArrayVariable.cpp:439–461 — nested braces for multi-dim; pick from init_strings.
// C++ assert(dimen < dim) and % init_strings.size(); empty list is broken IR.
func (av *ArrayVariable) buildInitRecursive(dimen int, initStrings []string, seed *uint32) string {
	if av == nil || dimen >= len(av.Sizes) || len(initStrings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < av.Sizes[dimen]; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		if dimen == len(av.Sizes)-1 {
			// magic index pick (ArrayVariable.cpp:448–452)
			s := *seed
			rnd := ((s*s + uint32(i+7)*uint32(i+13)) * 52369) % uint32(len(initStrings))
			b.WriteString(initStrings[rnd])
			*seed = s + 1
		} else {
			b.WriteString(av.buildInitRecursive(dimen+1, initStrings, seed))
		}
	}
	b.WriteString("}")
	return b.String()
}

// OutputDef emits a definition with brace initializer when no_loop_initializer.
// ArrayVariable.cpp:491–520 — brace for globals/const/multi; bare decl for loop-init locals.
func (av *ArrayVariable) OutputDef() string {
	if av == nil {
		return ""
	}
	// ArrayVariable.cpp:493 — only collective (parent) arrays emit def
	if av.Collective != nil {
		return ""
	}
	// ArrayVariable.cpp:494–507 — OutputDecl always live; no invent bare ";" / " = …"
	decl := av.CDeclType()
	if decl == "" {
		return ""
	}
	var b strings.Builder
	if !av.NoLoopInitializer() {
		// ArrayVariable.cpp:494–498 — OutputDecl + ";" (loop fills body)
		b.WriteString(decl)
		b.WriteString(";")
		return b.String()
	}
	// ArrayVariable.cpp:500–507 — string initializer; assert(init)
	vals := make([]string, 0, 1+len(av.InitValues))
	if av.Init != nil && av.Init.Value != "" {
		vals = append(vals, av.Init.Value)
	} else if av.InitExpr != nil {
		if s := av.InitExpr.Output(); s != "" {
			vals = append(vals, s)
		}
	}
	vals = append(vals, av.InitValues...)
	// assert(init) — no soft invent "0" brace list
	if len(vals) == 0 {
		return ""
	}
	b.WriteString(decl)
	// multi-dim or multi-value: recursive full initializer when total size small
	if av.TotalSize() <= 64 && (len(av.Sizes) > 1 || len(vals) > 1) {
		seed := uint32(0xABCDEF)
		b.WriteString(" = ")
		b.WriteString(av.buildInitRecursive(0, vals, &seed))
	} else {
		b.WriteString(" = {")
		maxEmit := av.TotalSize()
		if maxEmit > 8 {
			maxEmit = 8
		}
		for i := 0; i < maxEmit && i < len(vals); i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(vals[i])
		}
		b.WriteString("}")
	}
	b.WriteString(";")
	// Variable.cpp:658–661 — ArrayVariable inherits OutputDef comment path for volatile globals
	if av.IsGlobal() && av.IsVolatile() {
		b.WriteString(" /* VOLATILE GLOBAL ")
		b.WriteString(av.GetActualName(false))
		b.WriteString(" */")
	}
	return b.String()
}

// OutputLowerBound mirrors ArrayVariable::OutputLowerBound — name[0][0]….
// ArrayVariable.cpp:694–700.
func (av *ArrayVariable) OutputLowerBound() string {
	if av == nil {
		return ""
	}
	s := av.Name
	for range av.Sizes {
		s += "[0]"
	}
	return s
}

// OutputWithIndices mirrors ArrayVariable::output_with_indices.
// ArrayVariable.cpp:703–711 — cvs[i]->Output only (no letter-name invent).
func (av *ArrayVariable) OutputWithIndices(ctrl []string) string {
	if av == nil {
		return ""
	}
	name := av.GetActualName(false)
	if name == "" {
		return ""
	}
	// C++ cvs sized for get_dimension(); no invent empty "[]" when ctrl short/empty
	if len(ctrl) < len(av.Sizes) {
		return ""
	}
	for i := range av.Sizes {
		if ctrl[i] == "" {
			return ""
		}
	}
	var b strings.Builder
	b.WriteString(name)
	for i := range av.Sizes {
		// ArrayVariable.cpp:708–709 — cvs[i]->Output(out); always live
		b.WriteString("[")
		b.WriteString(ctrl[i])
		b.WriteString("]")
	}
	return b.String()
}

// OutputInit mirrors ArrayVariable::output_init — nested for loops assigning init.
// ArrayVariable.cpp:619–655. ctrl names from new_ctrl_vars (i,j,k…).
// postIncr: CGOptions::post_incr_operator — "i++" vs "i = i + 1".
func (av *ArrayVariable) OutputInit(indent string, ctrl []string) string {
	return av.OutputInitOpts(indent, ctrl, true)
}

// OutputInitOpts is OutputInit with post_incr_operator control.
// ArrayVariable.cpp:619–655 — cvs[i] names only (no letter-name soft invent).
func (av *ArrayVariable) OutputInitOpts(indent string, ctrl []string, postIncr bool) string {
	if av == nil || av.NoLoopInitializer() {
		return ""
	}
	// ArrayVariable.cpp:622–623 — collective itemized members skip output_init
	if av.Collective != nil {
		return ""
	}
	// C++ requires cvs sized for get_dimension(); undersized → no invent i/j/k
	if len(ctrl) < len(av.Sizes) {
		return ""
	}
	for i := range av.Sizes {
		if ctrl[i] == "" {
			return ""
		}
	}
	// ArrayVariable.cpp:649 — init->Output; always live Expression* (no invent "0")
	var initVal string
	if av.InitExpr != nil {
		initVal = av.InitExpr.Output()
	} else if av.Init != nil {
		initVal = av.Init.Value
	}
	if initVal == "" {
		return ""
	}
	var b strings.Builder
	// nested fors for each dimension
	pad := indent
	for i, sz := range av.Sizes {
		iv := ctrl[i]
		incr := iv + "++"
		if !postIncr {
			incr = iv + " = " + iv + " + 1"
		}
		b.WriteString(pad + "for (" + iv + " = 0; " + iv + " < " + itoa(sz) + "; " + incr + ")\n")
		if i+1 < len(av.Sizes) {
			b.WriteString(pad + "{\n")
			pad += "    "
		}
	}
	// a[i][j] = init;
	b.WriteString(pad + "    " + av.OutputWithIndices(ctrl) + " = " + initVal + ";\n")
	for i := len(av.Sizes) - 1; i >= 1; i-- {
		pad = pad[:len(pad)-4]
		b.WriteString(pad + "}\n")
	}
	return b.String()
}

// Itemize mirrors ArrayVariable::itemize(void).
// ArrayVariable.cpp:249–278 — random const indices; AllVars; create_field_vars for aggregates.
func (av *ArrayVariable) Itemize(r *Rng) *ArrayVariable {
	return av.ItemizeInto(r, nil)
}

// ItemizeInto is Itemize with optional VariableSelector for AllVars registration.
// ArrayVariable.cpp:251–252 — VariableSelector::AllVars.push_back(av).
func (av *ArrayVariable) ItemizeInto(r *Rng, vs *VariableSelector) *ArrayVariable {
	// ArrayVariable::itemize (void) — ArrayVariable.cpp:249–278
	if av == nil || r == nil {
		return nil
	}
	// ArrayVariable.cpp:250 — assert(collective == 0); no soft invent re-itemize self
	if av.Collective != nil {
		return nil
	}
	item := &ArrayVariable{
		Variable: Variable{
			Name:       av.Name,
			Type:       av.Type,
			Qfer:       av.Qfer,
			IsArray:    true,
			Init:       av.Init,
			InitExpr:   av.InitExpr,
			ArraySizes: av.Sizes,
			ArrayInits: av.ArrayInits,
		},
		Sizes:      av.Sizes,
		InitExprs:  append([]*Expression(nil), av.InitExprs...),
		InitValues: av.InitValues,
		Block:      av.Block,
		Collective: av,
	}
	item.AsArray = item
	for _, sz := range av.Sizes {
		idx := 0
		if sz > 0 {
			idx = int(r.RndUpto(uint32(sz)))
		}
		s := fmt.Sprintf("%d", idx)
		item.Indices = append(item.Indices, s)
		// ArrayVariable.cpp:257–258 — Constant(get_int_type(), int2str(index))
		item.IndexExprs = append(item.IndexExprs, &Expression{
			Term: TermConstant, Con: MakeInt(idx), ExprType: GetIntType(),
		})
	}
	// ArrayVariable.cpp:261–264 — only expand struct/union for itemized member
	if item.Type != nil && item.Type.IsAggregate() {
		item.CreateFieldVars()
	}
	if vs != nil {
		vs.AllVars = append(vs.AllVars, &item.Variable)
	}
	return item
}

// SizeInBytesArray mirrors ArrayVariable::size_in_bytes.
// ArrayVariable.cpp:241–247 — elem size × product of dimensions.
func (av *ArrayVariable) SizeInBytesArray() int {
	if av == nil || av.Type == nil {
		return 0
	}
	n := av.Type.SizeInBytes()
	for _, sz := range av.Sizes {
		if sz > 0 {
			n *= sz
		}
	}
	return n
}

// OutputAccess mirrors ArrayVariable::Output for itemized / collective emit.
// ArrayVariable.cpp:539–571 — collective bare name; itemized name[index]…
// C++ assert(!indices.empty()) then indices[i]->Output; always `if (1)` path
// (not signed-cast % size). No sizes-only invent and no soft "[0]".
func (av *ArrayVariable) OutputAccess() string {
	if av == nil {
		return ""
	}
	name := av.GetActualName(false)
	if name == "" {
		// name always live; no invent bare indices without identifier
		return ""
	}
	if av.Collective == nil {
		return name
	}
	// ArrayVariable.cpp:544–545 — assert(!indices.empty())
	// IndexExprs preferred (Expression::Output); Indices is const-itemize string form.
	if len(av.IndexExprs) == 0 && len(av.Indices) == 0 {
		// fail closed — no soft invent bare collective name for broken itemized IR
		return ""
	}
	var b strings.Builder
	b.WriteString(name)
	if len(av.IndexExprs) > 0 {
		for _, e := range av.IndexExprs {
			// ArrayVariable.cpp:548–552 — indices[i]->Output always live Expression*
			// no invent empty brackets "[]" for nil/empty index Output
			if e == nil {
				return ""
			}
			idx := e.Output()
			if idx == "" {
				return ""
			}
			b.WriteString("[")
			b.WriteString(idx)
			b.WriteString("]")
		}
		return b.String()
	}
	for _, s := range av.Indices {
		// const-itemize string indices always non-empty in C++
		if s == "" {
			return ""
		}
		b.WriteString("[")
		b.WriteString(s)
		b.WriteString("]")
	}
	return b.String()
}

// OutputUpperBoundArray mirrors ArrayVariable::OutputUpperBound — name[size-1]….
// ArrayVariable.cpp:572–577 — always (sizes[i] - 1); no soft invent "0" for empty dims.
func (av *ArrayVariable) OutputUpperBoundArray() string {
	if av == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(av.GetActualName(false))
	for _, sz := range av.Sizes {
		// ArrayVariable.cpp:575 — out << "[" << (sizes[i] - 1) << "]"
		b.WriteString("[" + itoa(sz-1) + "]")
	}
	return b.String()
}

// OutputIndexModulo is the dead-code branch of ArrayVariable::Output when
// index may be out of range (signed cast + % size). Kept for completeness;
// live C++ path uses `if (1)` always. ArrayVariable.cpp:553–568.
func (av *ArrayVariable) OutputIndexModulo(i int, idx *Expression) string {
	// dead path; C++ would dereference indices[i] — no soft invent "0"
	if av == nil || idx == nil {
		return ""
	}
	size := 1
	if i >= 0 && i < len(av.Sizes) && av.Sizes[i] > 0 {
		size = av.Sizes[i]
	}
	body := idx.Output()
	// cast signed index type to unsigned before %
	if t := idx.GetType(); t != nil && t.IsSigned() {
		if u := t.ToUnsigned(); u != nil {
			return fmt.Sprintf("((%s)(%s) %% %d)", u.CName(), body, size)
		}
	}
	return fmt.Sprintf("((%s) %% %d)", body, size)
}

// RndMutate mirrors ArrayVariable::rnd_mutate.
// ArrayVariable.cpp:336–337 — assert(0 && "invalid call to rnd_mutate"); dead API.
// Fail closed: always nil (no invent variant/offset mutation).
func (av *ArrayVariable) RndMutate(r *Rng) *ArrayVariable {
	_ = r
	return nil
}

// CreateMutatedArrayVar mirrors VariableSelector::create_mutated_array_var.
// VariableSelector.cpp:1552–1554 — assert(0 && "invalid call…"); dead API.
// Fail closed: always nil (no invent new itemized member from index rewrite).
func CreateMutatedArrayVar(av *ArrayVariable, newIndices []*Expression) *ArrayVariable {
	_ = av
	_ = newIndices
	return nil
}
