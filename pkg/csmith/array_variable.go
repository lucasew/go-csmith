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
	// InitValues are alternative initializers (simplified: string constants).
	InitValues []string
	// Block is the owning block for locals (nil if global).
	Block *Block
	// Collective is non-nil for itemized members (points at parent array).
	Collective *ArrayVariable
	// Indices are constant index strings for itemized access.
	Indices []string
}

// CreateArrayVariable mirrors ArrayVariable::CreateArrayVariable.
// ArrayVariable.cpp:123–180 — dimension distribution and size caps.
func CreateArrayVariable(
	r *Rng,
	opts Options,
	blk *Block,
	name string,
	elem *Type,
	init *Constant,
	qfer CVQualifiers,
) *ArrayVariable {
	if r == nil || elem == nil {
		return nil
	}
	if elem.IsSimple() && elem.Simple() == EVoid {
		return nil
	}
	// dimension: 1d 60%, 2d 30%, … via rnd_upto(99)+1 stepping
	// ArrayVariable.cpp:131–144
	num := int(r.RndUpto(99)) + 1
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
	if dimension > opts.MaxArrayDim {
		dimension = opts.MaxArrayDim
	}
	if dimension < 1 {
		dimension = 1
	}
	sizes := make([]int, 0, dimension)
	total := 1
	for i := 0; i < dimension; i++ {
		dimen := int(r.RndUpto(uint32(opts.MaxArrayLenPerDim))) + 1
		if opts.MaxArrayLength > 0 && total*dimen > opts.MaxArrayLength {
			dimen = opts.MaxArrayLength / total
		}
		if dimen < 1 {
			// stop adding dims if cannot fit
			break
		}
		total *= dimen
		sizes = append(sizes, dimen)
	}
	if len(sizes) == 0 {
		sizes = []int{1}
		total = 1
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
	// self-link for ChooseOKVar itemize (VariableSelector.cpp:332–337)
	av.AsArray = av
	// init_num = pure_rnd_upto(total_size/2); alt constants
	// ArrayVariable.cpp:166
	half := total / 2
	if half < 1 {
		half = 1
	}
	initNum := int(r.RndUpto(uint32(half)))
	for i := 0; i < initNum; i++ {
		c := MakeRandom(elem, opts, r)
		if c != nil {
			av.InitValues = append(av.InitValues, c.Value)
			av.ArrayInits = append(av.ArrayInits, c.Value)
		}
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
	if av == nil || av.Type == nil {
		return "int"
	}
	var b strings.Builder
	if av.IsConst() {
		b.WriteString("const ")
	}
	if av.IsVolatile() {
		b.WriteString("volatile ")
	}
	b.WriteString(av.Type.CName())
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

// buildInitRecursive mirrors ArrayVariable::build_init_recursive.
// ArrayVariable.cpp:439–461 — nested braces for multi-dim; pick from init_strings.
func (av *ArrayVariable) buildInitRecursive(dimen int, initStrings []string, seed *uint32) string {
	if av == nil || dimen >= len(av.Sizes) || len(initStrings) == 0 {
		return "0"
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
// ArrayVariable.cpp OutputDef path — brace for globals/const/multi; bare decl for loop-init locals.
func (av *ArrayVariable) OutputDef() string {
	if av == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(av.CDeclType())
	if av.NoLoopInitializer() && (av.Init != nil || len(av.InitValues) > 0) {
		vals := make([]string, 0, 1+len(av.InitValues))
		if av.Init != nil && av.Init.Value != "" {
			vals = append(vals, av.Init.Value)
		}
		vals = append(vals, av.InitValues...)
		if len(vals) == 0 {
			vals = []string{"0"}
		}
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
// ArrayVariable.cpp:703–711.
func (av *ArrayVariable) OutputWithIndices(ctrl []string) string {
	if av == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(av.GetActualName(false))
	for i := range av.Sizes {
		b.WriteString("[")
		if i < len(ctrl) && ctrl[i] != "" {
			b.WriteString(ctrl[i])
		} else {
			b.WriteString(string([]byte{byte('i' + i)}))
		}
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
func (av *ArrayVariable) OutputInitOpts(indent string, ctrl []string, postIncr bool) string {
	if av == nil || av.NoLoopInitializer() {
		return ""
	}
	initVal := "0"
	if av.Init != nil && av.Init.Value != "" {
		initVal = av.Init.Value
	}
	var b strings.Builder
	// nested fors for each dimension
	pad := indent
	for i, sz := range av.Sizes {
		iv := string([]byte{byte('i' + i)})
		if i < len(ctrl) && ctrl[i] != "" {
			iv = ctrl[i]
		}
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

// Indices holds itemized index expressions (constant strings for now).
// Collective is the parent array when this is an itemized member.
func (av *ArrayVariable) Itemize(r *Rng) *ArrayVariable {
	// ArrayVariable::itemize (void) — ArrayVariable.cpp:249–278
	if av == nil || r == nil {
		return nil
	}
	if av.Collective != nil {
		// already itemized
		return av
	}
	item := &ArrayVariable{
		Variable: Variable{
			Name:       av.Name,
			Type:       av.Type,
			Qfer:       av.Qfer,
			IsArray:    true,
			Init:       av.Init,
			ArraySizes: av.Sizes,
			ArrayInits: av.ArrayInits,
		},
		Sizes:      av.Sizes,
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
		item.Indices = append(item.Indices, fmt.Sprintf("%d", idx))
	}
	return item
}

// OutputAccess emits name[i0][i1]… for itemized, or bare name for collective.
func (av *ArrayVariable) OutputAccess() string {
	if av == nil {
		return ""
	}
	if av.Collective == nil || len(av.Indices) == 0 {
		return av.Name
	}
	var b strings.Builder
	b.WriteString(av.Name)
	for _, ix := range av.Indices {
		b.WriteString("[")
		b.WriteString(ix)
		b.WriteString("]")
	}
	return b.String()
}
