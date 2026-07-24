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
	// ArrayVariable.cpp:127–129 — assert(type); assert simple != eVoid sticky
	// name always live from gensym; sticky no invent empty-name array shell
	if r == nil || elem == nil || name == "" {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	simple := elem.IsSimple()
	// residual ERROR sticky — no invent soft-array past IsSimple residual
	if sessHasError(nil) {
		return nil
	}
	if simple && elem.Simple() == EVoid {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// incomplete ambient / facts fail closed sticky when CG is live (make_init_value path)
	// (no invent array create / soft re-pick past holes under incomplete shells)
	if cg != nil {
		if !EffectComplete(cg.EffectContext()) ||
			(cg.EffectAccum != nil && !EffectComplete(*cg.EffectAccum)) ||
			!EffectComplete(cg.EffectStm) {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		if cg.FM != nil && !FactsComplete(cg.FM.GlobalFacts) {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
	}
	// dimension: 1d 60%, 2d 30%, … via rnd_upto(99)+1 stepping
	// ArrayVariable.cpp:131–144
	num := int(r.RndUpto(99)) + 1
	// ArrayVariable.cpp:133 — ERROR_GUARD(nullptr)
	if sessHasError(nil) {
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
		if sessHasError(nil) {
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
			Qfer:       qfer.Clone(), // own vectors (C++ Variable value-copy)
			IsArray:    true,
			Init:       init,
			ArraySizes: sizes,
		},
		Sizes: sizes,
		Block: blk,
	}
	// ArrayVariable.cpp:161 — ERROR_GUARD_AND_DEL1 after new ArrayVariable
	if sessHasError(nil) {
		return nil
	}
	// self-link for ChooseOKVar itemize (VariableSelector.cpp:332–337)
	av.AsArray = av
	// ArrayVariable.cpp:161–163 — create_field_vars for aggregate element type
	if elem.IsAggregate() {
		av.CreateFieldVars()
	}
	if sessHasError(nil) {
		return nil
	}
	// ArrayVariable.cpp:165–186 — pure_rnd_upto(total_size/2) then alt inits.
	// pure_rnd_upto(0)==0 with no draw; use PureRndUpto so ProcessRng matches C++
	// RandomNumber singleton when r is the session process RNG.
	half := uint32(total / 2)
	// Prefer process pure path when r is the live process generator (C++ pure_rnd_*)
	var initNum int
	if pr := ProcessRng(); pr != nil && pr == r {
		initNum = int(PureRndUpto(half, nil))
	} else {
		initNum = int(r.RndUpto(half))
	}
	for i := 0; i < initNum; i++ {
		// ArrayVariable.cpp:177–185
		// if (!pointer || strict_const_arrays) Constant::make_random
		// else VariableSelector::make_init_value
		var e *Expression
		ptrLike := elem.IsPointerLike()
		if sessHasError(nil) {
			return nil
		}
		if !ptrLike || opts.StrictConstArrays {
			c := MakeRandom(elem, opts, probs, r)
			if sessHasError(nil) {
				return nil
			}
			if c != nil {
				e = &Expression{Term: TermConstant, Con: c, ExprType: elem}
			}
		} else {
			if vs == nil || cg == nil {
				sessNoteError(nil, ErrGeneric)
				return nil
			}
			qf := qfer
			e = vs.MakeInitValue(AccessRead, *cg, elem, &qf, blk, r)
			if sessHasError(nil) {
				return nil
			}
		}
		// ArrayVariable.cpp:185 — add_init_value(e); to_string only at OutputDef
		if e == nil {
			return nil
		}
		av.InitExprs = append(av.InitExprs, e)
		// Keep InitValues for legacy tests / Fact paths that read strings
		val := e.Output()
		if sessHasError(nil) {
			return nil
		}
		if val != "" {
			av.InitValues = append(av.InitValues, val)
			av.ArrayInits = append(av.ArrayInits, val)
		} else {
			sessNoteError(nil, ErrGeneric)
			return nil
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
// ArrayVariable always live at get_dimension; sticky 0 (no invent dim soft-skip past hole).
func (av *ArrayVariable) Dimension() int {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	return len(av.Sizes)
}

// TotalSize is product of sizes.
// ArrayVariable always live; sticky 0 (no invent empty-size soft-skip past hole).
func (av *ArrayVariable) TotalSize() int {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	n := 1
	for _, s := range av.Sizes {
		n *= s
	}
	return n
}

// IsGlobal for arrays: name prefix g_ (same as Variable).
// ArrayVariable always live; sticky incomplete no invent not-global soft-skip.
func (av *ArrayVariable) IsGlobal() bool {
	// residual sticky via Variable.IsGlobal
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	ok := av.Variable.IsGlobal()
	// residual ERROR sticky — no invent soft-global past Variable.IsGlobal residual
	if sessHasError(nil) {
		return false
	}
	return ok
}

// CDeclType returns C type with dimensions, e.g. int x[2][3].
// ArrayVariable.cpp:512–521 OutputDecl — output_qualified_type + name + [sizes].
// Do not invent bare Type.CName + storage IsVolatile (misplaces pointer vol).
func (av *ArrayVariable) CDeclType() string {
	// ArrayVariable / Variable decl always has live type; sticky (no invent "int")
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if av.Type == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp:517 — output_qualified_type(out)
	ty := av.Qfer.OutputQualifiedType(av.Type)
	// residual ERROR sticky — no invent soft-empty decl past OutputQualifiedType residual
	if sessHasError(nil) {
		return ""
	}
	if ty == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp:518 — get_actual_name(); no invent space (type already spaced)
	name := av.GetActualName(false)
	// residual ERROR sticky — no invent soft-continue decl past GetActualName residual
	if sessHasError(nil) {
		return ""
	}
	if name == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(ty)
	b.WriteString(name)
	for _, s := range av.Sizes {
		b.WriteString(fmt.Sprintf("[%d]", s))
	}
	return b.String()
}

// NoLoopInitializer mirrors ArrayVariable::no_loop_initializer.
// ArrayVariable.cpp:429–435 — struct/union, const, global, or multi init_values.
// Incomplete ArrayVariable/Type sticky true (no invent loop-init eligibility past holes).
func (av *ArrayVariable) NoLoopInitializer() bool {
	// ArrayVariable always live with Type; sticky incomplete no invent loop-init OK
	if av == nil || av.Type == nil {
		sessNoteError(nil, ErrGeneric)
		return true
	}
	if av.Type.IsAggregate() {
		return true
	}
	if av.IsConst() {
		// residual ERROR sticky — no invent no-loop true past IsConst residual hole
		if sessHasError(nil) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue no-loop past IsConst residual false
	if sessHasError(nil) {
		return true
	}
	if av.IsGlobal() {
		// residual ERROR sticky — no invent no-loop true past IsGlobal residual hole
		if sessHasError(nil) {
			return true
		}
		return true
	}
	// residual ERROR sticky — no invent soft-continue no-loop past IsGlobal residual false
	if sessHasError(nil) {
		return true
	}
	return len(av.InitValues) > 0
}

// CountExprKeyVar mirrors count_expr_key_var (ArrayVariable.cpp:66–90).
// Variable: 1; Constant: 0; user call: 2; unary: recurse; binary: sum.
// Incomplete IR fails closed sticky as -1 (no invent constant-0 / var-1 / soft re-pick
// variant match past broken shells).
func CountExprKeyVar(e *Expression) int {
	if e == nil {
		// ArrayVariable.cpp:89 assert(0) for unexpected; nil is broken IR sticky
		sessNoteError(nil, ErrGeneric)
		return -1
	}
	switch e.Term {
	case TermVariable:
		// ExpressionVariable always has live Variable*
		if e.Var == nil {
			sessNoteError(nil, ErrGeneric)
			return -1
		}
		return 1
	case TermConstant:
		// live Constant* Type+Value; incomplete shell fails closed sticky
		// (no invent key-count 0 for Type-nil / empty-value soft-miss)
		if e.Con == nil || e.Con.Type == nil || e.Con.Value == "" {
			sessNoteError(nil, ErrGeneric)
			return -1
		}
		return 0
	case TermFunction:
		if e.Invoke == nil {
			sessNoteError(nil, ErrGeneric)
			return -1
		}
		// user call → 2 (ArrayVariable.cpp:76–77)
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			return 2
		}
		n := len(e.Invoke.Args)
		if n == 1 {
			// param always live; nil arg fails closed sticky
			if e.Invoke.Args[0] == nil {
				sessNoteError(nil, ErrGeneric)
				return -1
			}
			c := CountExprKeyVar(e.Invoke.Args[0])
			// residual ERROR sticky — no invent key-count past nested residual hole
			if sessHasError(nil) {
				return -1
			}
			return c
		}
		// ArrayVariable.cpp:84 — assert(param_value.size() == 2) for binary
		if n == 2 {
			if e.Invoke.Args[0] == nil || e.Invoke.Args[1] == nil {
				sessNoteError(nil, ErrGeneric)
				return -1
			}
			a := CountExprKeyVar(e.Invoke.Args[0])
			// residual ERROR sticky — no invent soft-sum past left residual hole
			if sessHasError(nil) {
				return -1
			}
			b := CountExprKeyVar(e.Invoke.Args[1])
			// residual ERROR sticky — no invent soft-sum past right residual hole
			if sessHasError(nil) {
				return -1
			}
			if a < 0 || b < 0 {
				if !sessHasError(nil) {
					sessNoteError(nil, ErrGeneric)
				}
				return -1
			}
			return a + b
		}
		// wrong arity — fail closed sticky -1 (no invent sum of first two)
		sessNoteError(nil, ErrGeneric)
		return -1
	default:
		// ArrayVariable.cpp:89 assert(0)
		sessNoteError(nil, ErrGeneric)
		return -1
	}
}

// FindExprKeyVar mirrors find_expr_key_var (ArrayVariable.cpp:98–119).
// Sole key variable of an index expression, or nil if none / ambiguous.
// Incomplete IR (nil Invoke/args) fails closed sticky nil (no invent key from partial).
func FindExprKeyVar(e *Expression) *Variable {
	if e == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	switch e.Term {
	case TermVariable:
		// incomplete Variable* shell sticky → nil
		// Type-nil non-special sticky (no invent key var soft-success past type hole)
		if e.Var == nil {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		if e.Var.Type == nil && !IsSpecialPtr(e.Var) {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		return e.Var
	case TermFunction:
		if e.Invoke == nil {
			sessNoteError(nil, ErrGeneric)
			return nil
		}
		if e.Invoke.User != nil && !e.Invoke.IsStd {
			return nil // user call: no single key var (complete empty)
		}
		n := len(e.Invoke.Args)
		if n == 1 {
			if e.Invoke.Args[0] == nil {
				sessNoteError(nil, ErrGeneric)
				return nil
			}
			kv := FindExprKeyVar(e.Invoke.Args[0])
			// residual ERROR sticky — no invent key past nested residual hole
			if sessHasError(nil) {
				return nil
			}
			return kv
		}
		// ArrayVariable.cpp:110 — assert(param_value.size() == 2)
		if n == 2 {
			if e.Invoke.Args[0] == nil || e.Invoke.Args[1] == nil {
				sessNoteError(nil, ErrGeneric)
				return nil
			}
			v0 := FindExprKeyVar(e.Invoke.Args[0])
			// residual ERROR sticky — no invent soft-pick right past left residual hole
			if sessHasError(nil) {
				return nil
			}
			v1 := FindExprKeyVar(e.Invoke.Args[1])
			// residual ERROR sticky — no invent soft-pick left past right residual hole
			if sessHasError(nil) {
				return nil
			}
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
// Incomplete IndexExprs fails closed sticky false (no invent soft-skip nil hole then
// string Indices match as complete variant; no invent mixed expr/string dual path).
func (av *ArrayVariable) IsVariant(other *Variable) bool {
	// both ArrayVariable shells always live; sticky incomplete no invent not-variant
	if av == nil || other == nil {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	if !other.IsArray {
		return false
	}
	ov := other.AsArray
	if ov == nil {
		// incomplete array flag without AsArray sticky
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// both must be itemized (collective non-nil) under the same collective
	if av.Collective == nil || ov.Collective == nil {
		return false
	}
	if av.Collective != ov.Collective {
		return false
	}
	// incomplete IndexExprs holes — sticky fail closed before string soft-fallback
	if !ExpressionsComplete(av.IndexExprs) || !ExpressionsComplete(ov.IndexExprs) {
		sessNoteError(nil, ErrGeneric)
		return false
	}
	// Expression* path when either side has IndexExprs — both must use same complete list
	// (no invent half IndexExprs + half Indices string match past a lag hole)
	if len(av.IndexExprs) > 0 || len(ov.IndexExprs) > 0 {
		if len(av.IndexExprs) != len(ov.IndexExprs) {
			return false
		}
		for i := range av.IndexExprs {
			e, oe := av.IndexExprs[i], ov.IndexExprs[i]
			// ArrayVariable.cpp:403–405 — Expression path; live Expression* only
			ce := CountExprKeyVar(e)
			// residual ERROR sticky — no invent soft-continue count past residual hole
			if sessHasError(nil) {
				return false
			}
			coe := CountExprKeyVar(oe)
			// residual ERROR sticky — no invent soft-continue count past oe residual hole
			if sessHasError(nil) {
				return false
			}
			if ce != 1 || coe != 1 {
				return false
			}
			ke := FindExprKeyVar(e)
			// residual ERROR sticky — no invent soft-continue key match past residual hole
			if sessHasError(nil) {
				return false
			}
			koe := FindExprKeyVar(oe)
			// residual ERROR sticky — no invent soft-continue key match past oe residual
			if sessHasError(nil) {
				return false
			}
			// both residual-nil soft invent was invent variant-true (nil==nil)
			if ke == nil || koe == nil || ke != koe {
				return false
			}
		}
		return true
	}
	// string-only Indices: equal strings share key identity (no Variable* handle)
	n := len(av.Indices)
	if n == 0 || n != len(ov.Indices) {
		return false
	}
	for i := 0; i < n; i++ {
		if av.Indices[i] != ov.Indices[i] {
			return false
		}
	}
	return true
}

// ItemizeConstIndices mirrors ArrayVariable::itemize(const vector<int>&).
// ArrayVariable.cpp:280–295 — fixed const indices; create_field_vars for aggregates.
// ArrayVariable always live; sticky nil (no invent itemize soft-skip past hole).
// Itemized member (Collective set) is complete soft miss (not incomplete IR).
func (av *ArrayVariable) ItemizeConstIndices(constIndices []int, vs *VariableSelector) *ArrayVariable {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if av.Collective != nil {
		return nil
	}
	if len(constIndices) != len(av.Sizes) {
		return nil
	}
	item := &ArrayVariable{
		Variable: Variable{
			Name:       av.Name,
			Type:       av.Type,
			Qfer:       av.Qfer.Clone(), // do not share collective qfer vectors
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
	// ArrayVariable.cpp:288–291 — type always live; type->is_aggregate()
	// sticky no invent itemize soft-success past Type-nil shell (skip field expand)
	if item.Type == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if item.Type.IsAggregate() {
		item.CreateFieldVars()
		// residual ERROR sticky — no invent itemize shell past CreateFieldVars residual
		if sessHasError(nil) {
			return nil
		}
	}
	if vs != nil {
		vs.AllVars = append(vs.AllVars, &item.Variable)
	}
	return item
}

// SetIndex mirrors ArrayVariable::set_index (string form for emit).
// ArrayVariable.cpp:229–231 — indices[index] = e; vector already sized for dim.
// sticky no invent pad empty holes or empty Constant "0" stand-ins
// ArrayVariable always live; sticky (no invent soft-skip index set past hole).
func (av *ArrayVariable) SetIndex(index int, expr string) {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if index < 0 || expr == "" {
		sessNoteError(nil, ErrGeneric)
		return
	}
	// C++ indices[index] — sticky no grow past end with empty pad slots
	if index > len(av.Indices) {
		sessNoteError(nil, ErrGeneric)
		return
	}
	con := &Expression{
		Term: TermConstant, Con: &Constant{Value: expr, Type: GetIntType()}, ExprType: GetIntType(),
	}
	if index == len(av.Indices) {
		av.Indices = append(av.Indices, expr)
		av.IndexExprs = append(av.IndexExprs, con)
		return
	}
	av.Indices[index] = expr
	// keep IndexExprs in sync without inventing nil pad slots when lists lag
	// (incomplete dual-list IR sticky fail closed — no soft invent empty expr holes)
	if index < len(av.IndexExprs) {
		av.IndexExprs[index] = con
	} else if index == len(av.IndexExprs) {
		av.IndexExprs = append(av.IndexExprs, con)
	} else {
		// index > len(IndexExprs): lag hole sticky — leave IndexExprs unchanged
		sessNoteError(nil, ErrGeneric)
	}
}

// SetIndexExpr mirrors ArrayVariable::set_index(size_t, const Expression*).
// ArrayVariable.cpp:229–231 — stores Expression*; emit string from Output only.
// ArrayVariable always live; sticky (no invent soft-skip index set past hole).
func (av *ArrayVariable) SetIndexExpr(index int, e *Expression) {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if index < 0 || e == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	// sticky no soft invent "0" for nil/empty Output (C++ uses Expression* directly)
	s := e.Output()
	// residual ERROR sticky — no invent soft-continue set-index past Output residual
	if sessHasError(nil) {
		return
	}
	if s == "" {
		sessNoteError(nil, ErrGeneric)
		return
	}
	// C++ indices[index] — sticky no invent empty pad holes past end
	if index > len(av.IndexExprs) {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if index == len(av.IndexExprs) {
		av.IndexExprs = append(av.IndexExprs, e)
		av.Indices = append(av.Indices, s)
		return
	}
	av.IndexExprs[index] = e
	for len(av.Indices) <= index {
		if len(av.Indices) >= len(av.IndexExprs) {
			break
		}
		av.Indices = append(av.Indices, "")
	}
	if index < len(av.Indices) {
		av.Indices[index] = s
	} else if index == len(av.Indices) {
		av.Indices = append(av.Indices, s)
	}
}

// AddIndex mirrors ArrayVariable::add_index (string helper).
// ArrayVariable always live; sticky (no invent soft-skip add past hole).
func (av *ArrayVariable) AddIndex(expr string) {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if expr == "" {
		// empty index string is broken IR sticky; no invent empty Constant shell
		sessNoteError(nil, ErrGeneric)
		return
	}
	av.Indices = append(av.Indices, expr)
	av.IndexExprs = append(av.IndexExprs, &Expression{
		Term: TermConstant, Con: &Constant{Value: expr, Type: GetIntType()}, ExprType: GetIntType(),
	})
}

// AddIndexExpr appends an index Expression (ArrayVariable::add_index).
// ArrayVariable.cpp:227 — indices.push_back(e); sticky no soft invent "0".
// ArrayVariable always live; sticky (no invent soft-skip add past hole).
func (av *ArrayVariable) AddIndexExpr(e *Expression) {
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	if e == nil {
		sessNoteError(nil, ErrGeneric)
		return
	}
	s := e.Output()
	// residual ERROR sticky — no invent soft-continue add-index past Output residual
	if sessHasError(nil) {
		return
	}
	// incomplete index IR sticky — no invent empty bracket token
	if s == "" {
		sessNoteError(nil, ErrGeneric)
		return
	}
	av.IndexExprs = append(av.IndexExprs, e)
	av.Indices = append(av.Indices, s)
}

// currentSession().ArrayInitSeed mirrors ArrayVariable.cpp:429 — static unsigned seed = 0xABCDEF
// inside build_init_recursive. Process-wide: advances across every array OutputDef
// (do not reset per array).

// ResetArrayInitSeed restores the C++ static seed (tests / Finalization).
func ResetArrayInitSeed() { currentSession().ArrayInitSeed = 0xABCDEF }

// buildInitRecursive mirrors ArrayVariable::build_init_recursive.
// ArrayVariable.cpp:426–446 — nested braces; process-static seed pick; "," (no space).
func (av *ArrayVariable) buildInitRecursive(dimen int, initStrings []string) string {
	// C++ assert(dimen < dim) and % init_strings.size(); empty list is broken IR sticky
	if av == nil || dimen >= len(av.Sizes) || len(initStrings) == 0 {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString("{")
	for i := 0; i < av.Sizes[dimen]; i++ {
		if dimen == len(av.Sizes)-1 {
			// ArrayVariable.cpp:433–437 —
			//   size_t rnd_index = ((seed * seed + (i + 7) * (i + 13)) * 52369) % n;
			// seed is unsigned: seed*seed wraps at 32 bits, then promotes to size_t
			// for the rest of the expression (64-bit on LP64). All-uint32 Go mul was wrong.
			s := currentSession().ArrayInitSeed
			ss := uint64(s * s) // uint32 mul wrap, then widen
			prod := uint64(i+7) * uint64(i+13)
			rnd := (ss + prod) * 52369
			idx := int(rnd % uint64(len(initStrings)))
			part := initStrings[idx]
			if part == "" {
				sessNoteError(nil, ErrGeneric)
				return ""
			}
			b.WriteString(part)
			currentSession().ArrayInitSeed = s + 1
		} else {
			part := av.buildInitRecursive(dimen+1, initStrings)
			if part == "" {
				if !sessHasError(nil) {
					sessNoteError(nil, ErrGeneric)
				}
				return ""
			}
			b.WriteString(part)
		}
		// ArrayVariable.cpp:441–442 — comma only between elements, no trailing, no space
		if i != av.Sizes[dimen]-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("}")
	return b.String()
}

// buildInitializerStr mirrors ArrayVariable::build_initializer_str.
// ArrayVariable.cpp:450–474 — force_non_uniform → recursive seed path; else nested dims.
func (av *ArrayVariable) buildInitializerStr(initStrings []string) string {
	if av == nil || len(initStrings) == 0 {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// Process options match CGOptions::force_non_uniform_array_init (default true)
	if ProcessOptions().ForceNonUniformArrayInit {
		// ArrayVariable.cpp:429 / 452–453 — static seed continues across calls
		return av.buildInitRecursive(0, initStrings)
	}
	// ArrayVariable.cpp:456–473 — build from last dimension outward
	str := ""
	for i := len(av.Sizes) - 1; i >= 0; i-- {
		lenI := av.Sizes[i]
		var dim strings.Builder
		dim.WriteString("{")
		for j := 0; j < lenI; j++ {
			if i == len(av.Sizes)-1 {
				// ArrayVariable.cpp:462–463 — (i+(j+7)*(j+13))*52369 % n as size_t width
				rnd := (uint64(i) + uint64(j+7)*uint64(j+13)) * 52369
				idx := int(rnd % uint64(len(initStrings)))
				part := initStrings[idx]
				if part == "" {
					sessNoteError(nil, ErrGeneric)
					return ""
				}
				dim.WriteString(part)
			} else {
				dim.WriteString(str)
			}
			if j < lenI-1 {
				dim.WriteString(", ")
			}
		}
		dim.WriteString("}")
		str = dim.String()
	}
	return str
}

// OutputDef emits a definition with brace initializer when no_loop_initializer.
// ArrayVariable.cpp:491–520 — brace for globals/const/multi; bare decl for loop-init locals.
func (av *ArrayVariable) OutputDef() string {
	// ArrayVariable always live at OutputDef; sticky no invent empty def shell
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp:493 — only collective (parent) arrays emit def
	if av.Collective != nil {
		return ""
	}
	// ArrayVariable.cpp:494–507 — OutputDecl always live; sticky no invent bare ";" / " = …"
	decl := av.CDeclType()
	// residual ERROR sticky — no invent soft-empty def past CDeclType residual
	if sessHasError(nil) {
		return ""
	}
	if decl == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	if !av.NoLoopInitializer() {
		// residual ERROR sticky — no invent decl-only path past NoLoopInitializer residual
		if sessHasError(nil) {
			return ""
		}
		// ArrayVariable.cpp:494–498 — OutputDecl + ";" (loop fills body)
		b.WriteString(decl)
		b.WriteString(";")
		return b.String()
	}
	// residual ERROR sticky — no invent brace init past NoLoopInitializer residual true
	if sessHasError(nil) {
		return ""
	}
	// ArrayVariable.cpp:488–493 — init_strings from init->to_string() then each
	// init_values[i]->to_string() (Expression::Output at emit time, not cached Value).
	vals := make([]string, 0, 1+len(av.InitExprs))
	if av.InitExpr != nil {
		s := av.InitExpr.Output()
		if sessHasError(nil) {
			return ""
		}
		if s != "" {
			vals = append(vals, s)
		}
	} else if av.Init != nil {
		s := av.Init.Output()
		if sessHasError(nil) {
			return ""
		}
		if s != "" {
			vals = append(vals, s)
		}
	}
	for _, e := range av.InitExprs {
		if e == nil {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		s := e.Output()
		if sessHasError(nil) {
			return ""
		}
		if s == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
		vals = append(vals, s)
	}
	// Fallback: legacy InitValues if no Expression* pool (tests)
	if len(vals) == 0 && len(av.InitValues) > 0 {
		vals = append(vals, av.InitValues...)
	}
	// assert(init) — sticky no soft invent "0" brace list
	if len(vals) == 0 {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	for _, v := range vals {
		if v == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
	}
	b.WriteString(decl)
	// ArrayVariable.cpp:506 — always build_initializer_str (full nested braces).
	// Do not invent size caps (old tot>64 → emit 8 was not C++).
	init := av.buildInitializerStr(vals)
	if init == "" {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	b.WriteString(" = ")
	b.WriteString(init)
	b.WriteString(";")
	// ArrayVariable.cpp:506–507 — "= init;" then outputln only.
	// Do not invent Variable::OutputDef VOLATILE GLOBAL comment (arrays omit it).
	return b.String()
}

// OutputLowerBound mirrors ArrayVariable::OutputLowerBound — name[0][0]….
// ArrayVariable.cpp:694–700.
func (av *ArrayVariable) OutputLowerBound() string {
	// ArrayVariable always live at bound emit; sticky no invent empty bounds
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp: lower bound uses name + [0]…; name always live sticky
	name := av.GetActualName(false)
	// residual ERROR sticky — no invent soft-empty lower past GetActualName residual
	if sessHasError(nil) {
		return ""
	}
	if name == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	s := name
	for range av.Sizes {
		s += "[0]"
	}
	return s
}

// OutputWithIndices mirrors ArrayVariable::output_with_indices.
// ArrayVariable.cpp:703–711 — cvs[i]->Output only (no letter-name invent).
func (av *ArrayVariable) OutputWithIndices(ctrl []string) string {
	// ArrayVariable always live at index emit; sticky no invent empty access
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	name := av.GetActualName(false)
	// residual ERROR sticky — no invent soft-empty access past GetActualName residual
	if sessHasError(nil) {
		return ""
	}
	if name == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// C++ cvs sized for get_dimension(); sticky no invent empty "[]" when ctrl short/empty
	if len(ctrl) < len(av.Sizes) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	for i := range av.Sizes {
		if ctrl[i] == "" {
			sessNoteError(nil, ErrGeneric)
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
	// ArrayVariable always live at init emit; sticky no invent empty loop-init without it
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// no_loop_initializer: soft empty (brace def path used instead)
	if av.NoLoopInitializer() {
		// residual ERROR sticky — no invent empty soft-success past NoLoopInitializer residual
		if sessHasError(nil) {
			return ""
		}
		return ""
	}
	// residual ERROR sticky — no invent loop-init past NoLoopInitializer residual false
	if sessHasError(nil) {
		return ""
	}
	// ArrayVariable.cpp:622–623 — collective itemized members skip output_init
	if av.Collective != nil {
		return ""
	}
	// C++ requires cvs sized for get_dimension(); undersized sticky → no invent i/j/k
	if len(ctrl) < len(av.Sizes) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	for i := range av.Sizes {
		if ctrl[i] == "" {
			sessNoteError(nil, ErrGeneric)
			return ""
		}
	}
	// ArrayVariable.cpp:649 — init->Output; always live Expression* sticky (no invent "0")
	// Expression.cpp:120–123 to_string → Output (Constant.cpp:532–536 paren negatives).
	// Soft invent Init.Value skipped Output → " = -3L;" vs " = (-3L);" (seed-353 l_52).
	var initVal string
	if av.InitExpr != nil {
		initVal = av.InitExpr.Output()
		// residual ERROR sticky — no invent loop-init past Output residual hole
		if sessHasError(nil) {
			return ""
		}
	} else if av.Init != nil {
		initVal = av.Init.Output()
		// residual ERROR sticky — no invent loop-init past Constant Output residual hole
		if sessHasError(nil) {
			return ""
		}
	}
	if initVal == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp:649 + output_with_indices — access always live
	// sticky no invent for-loops + " = init;" without LHS
	access := av.OutputWithIndices(ctrl)
	if access == "" {
		if !sessHasError(nil) {
			sessNoteError(nil, ErrGeneric)
		}
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
	b.WriteString(pad + "    " + access + " = " + initVal + ";\n")
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
		// nil receiver/RNG incomplete sticky (no invent itemize without live array/rng)
		if av != nil && r == nil {
			sessNoteError(nil, ErrGeneric)
		}
		return nil
	}
	// ArrayVariable.cpp:250 — assert(collective == 0); sticky no soft invent re-itemize self
	if av.Collective != nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	item := &ArrayVariable{
		Variable: Variable{
			Name:       av.Name,
			Type:       av.Type,
			Qfer:       av.Qfer.Clone(), // do not share collective qfer vectors
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
	// ArrayVariable.cpp:261–264 — type always live; only expand aggregate itemized
	// sticky no invent itemize soft-success past Type-nil shell (skip field expand)
	if item.Type == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if item.Type.IsAggregate() {
		item.CreateFieldVars()
		// residual ERROR sticky — no invent itemize shell past CreateFieldVars residual
		if sessHasError(nil) {
			return nil
		}
	}
	if vs != nil {
		vs.AllVars = append(vs.AllVars, &item.Variable)
	}
	return item
}

// SizeInBytesArray mirrors ArrayVariable::size_in_bytes.
// ArrayVariable.cpp:241–247 — elem size × product of dimensions.
// ArrayVariable/Type always live; sticky 0 (no invent zero-size soft-skip past hole).
func (av *ArrayVariable) SizeInBytesArray() int {
	if av == nil || av.Type == nil {
		sessNoteError(nil, ErrGeneric)
		return 0
	}
	n := av.Type.SizeInBytes()
	// residual ERROR sticky — no invent soft-zero size past SizeInBytes residual
	if sessHasError(nil) {
		return 0
	}
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
	// ArrayVariable always live at Output; sticky no invent empty access shell
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	name := av.GetActualName(false)
	// residual ERROR sticky — no invent soft-empty access past GetActualName residual
	if sessHasError(nil) {
		return ""
	}
	if name == "" {
		// name always live; sticky no invent bare indices without identifier
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if av.Collective == nil {
		return name
	}
	// ArrayVariable.cpp:544–545 — assert(!indices.empty())
	// IndexExprs preferred (Expression::Output); Indices is const-itemize string form.
	if len(av.IndexExprs) == 0 && len(av.Indices) == 0 {
		// sticky fail closed — no soft invent bare collective name for broken itemized IR
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// incomplete IndexExprs fails closed sticky whole access (no invent soft-skip hole mid indices)
	if !ExpressionsComplete(av.IndexExprs) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(name)
	if len(av.IndexExprs) > 0 {
		for _, e := range av.IndexExprs {
			// ArrayVariable.cpp:548–552 — indices[i]->Output always live Expression*
			// sticky no invent empty brackets "[]" for empty index Output
			idx := e.Output()
			// residual ERROR sticky — no invent soft-continue later indices past Output residual
			if sessHasError(nil) {
				return ""
			}
			if idx == "" {
				sessNoteError(nil, ErrGeneric)
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
			sessNoteError(nil, ErrGeneric)
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
	// ArrayVariable always live at bound emit; sticky no invent empty upper bound
	if av == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	name := av.GetActualName(false)
	// residual ERROR sticky — no invent soft-empty upper past GetActualName residual
	if sessHasError(nil) {
		return ""
	}
	if name == "" {
		// name always live; sticky no invent bare "[n]" bounds
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if len(av.Sizes) == 0 {
		// sticky no invent bare name without dimensions
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(name)
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
	// dead path; C++ would dereference indices[i] — sticky no soft invent "0"
	if av == nil || idx == nil {
		if av != nil && idx == nil {
			sessNoteError(nil, ErrGeneric)
		}
		return ""
	}
	size := 1
	if i >= 0 && i < len(av.Sizes) && av.Sizes[i] > 0 {
		size = av.Sizes[i]
	}
	body := idx.Output()
	// residual ERROR sticky — no invent soft-empty modulo past Output residual hole
	if sessHasError(nil) {
		return ""
	}
	// index Output always live; sticky no invent "(( % n)" empty shell
	if body == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// index Type* always live for signed cast path; Type-nil sticky empty
	// (no invent bare modulo soft-success past incomplete index type shell)
	t := idx.GetType()
	// residual ERROR sticky — no invent soft-empty modulo past GetType residual hole
	if sessHasError(nil) {
		return ""
	}
	if t == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// cast signed index type to unsigned before %
	if t.IsSigned() {
		// residual ERROR sticky — no invent cast path past IsSigned residual hole
		if sessHasError(nil) {
			return ""
		}
		u := t.ToUnsigned()
		if u == nil {
			// incomplete to_unsigned sticky — no invent bare modulo past hole
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		// residual ERROR sticky — no invent cast path past ToUnsigned residual hole
		if sessHasError(nil) {
			return ""
		}
		cn := u.CName()
		if cn == "" {
			// incomplete unsigned CName sticky — no invent bare cast shell
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		return fmt.Sprintf("((%s)(%s) %% %d)", cn, body, size)
	}
	// residual ERROR sticky — no invent bare modulo past IsSigned residual false path
	if sessHasError(nil) {
		return ""
	}
	return fmt.Sprintf("((%s) %% %d)", body, size)
}

// RndMutate mirrors ArrayVariable::rnd_mutate.
// ArrayVariable.cpp:336–337 — assert(0 && "invalid call to rnd_mutate"); dead API.
// Fail closed sticky: always nil (no invent variant/offset mutation / soft re-pick).
func (av *ArrayVariable) RndMutate(r *Rng) *ArrayVariable {
	_ = r
	sessNoteError(nil, ErrGeneric)
	return nil
}

// CreateMutatedArrayVar mirrors VariableSelector::create_mutated_array_var.
// VariableSelector.cpp:1552–1554 — assert(0 && "invalid call…"); dead API.
// Fail closed sticky: always nil (no invent new itemized member from index rewrite).
func CreateMutatedArrayVar(av *ArrayVariable, newIndices []*Expression) *ArrayVariable {
	_ = av
	_ = newIndices
	sessNoteError(nil, ErrGeneric)
	return nil
}
