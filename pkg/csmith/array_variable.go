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

// OutputDef emits a definition with optional brace initializer (first + alts simplified).
func (av *ArrayVariable) OutputDef() string {
	if av == nil {
		return ""
	}
	var b strings.Builder
	if av.IsGlobal() {
		// force_globals_static handled by caller for consistency
	}
	b.WriteString(av.CDeclType())
	if av.Init != nil || len(av.InitValues) > 0 {
		b.WriteString(" = {")
		vals := make([]string, 0, 1+len(av.InitValues))
		if av.Init != nil && av.Init.Value != "" {
			vals = append(vals, av.Init.Value)
		}
		vals = append(vals, av.InitValues...)
		// cap emit length
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
	return b.String()
}
