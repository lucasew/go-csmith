// Upstream: Function.cpp initialize_builtin_functions / make_builtin_function.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// Builtin function catalog strings (Function.cpp:705–726).
// format: return_type; name; (params); kinds
var builtinFunctionStrings = []string{
	"UInt; __builtin_ia32_crc32qi; (UInt, UChar); x86",
	"Int; __builtin_clz; (UInt); x86",
	"Int; __builtin_clzl; (ULong); x86",
	"Int; __builtin_clzll; (ULonglong); x86",
	"Int; __builtin_ctz; (UInt); x86",
	"Int; __builtin_ctzl; (ULong); x86",
	"Int; __builtin_ctzll; (ULonglong); x86",
	"Int; __builtin_ffs; (Int); x86",
	"Int; __builtin_ffsl; (Long); x86",
	"Int; __builtin_ffsll; (Longlong); x86",
	"Int; __builtin_parity; (UInt); x86",
	"Int; __builtin_parityl; (ULong); x86",
	"Int; __builtin_parityll; (ULonglong); x86",
	"Int; __builtin_popcount; (UInt); x86",
	"Int; __builtin_popcountl; (ULong); x86",
	"Int; __builtin_popcountll; (ULonglong); x86",
	"UInt; __builtin_bswap32; (UInt); x86",
	"ULonglong; __builtin_bswap64; (ULonglong); x86",
	"Int; __builtin_ctzs; (UShort); clang",
	"Int; __builtin_clzs; (UShort); clang",
	"UShort; __builtin_bswap16; (UShort); ppc | clang",
}

// TypeFromString mirrors Type::get_type_from_string.
// Type.cpp:370–402 — assert(0 && "Unsupported type string!") on default.
func TypeFromString(s string) *Type {
	return TypeFromStringSess(nil, s)
}

// TypeFromStringSess is TypeFromString with explicit session residual sticky.
func TypeFromStringSess(s *Session, name string) *Type {
	name = strings.TrimSpace(name)
	switch name {
	case "Void":
		return GetSimpleType(EVoid)
	case "Char":
		return GetSimpleType(EChar)
	case "UChar":
		return GetSimpleType(EUChar)
	case "Short":
		return GetSimpleType(EShort)
	case "UShort":
		return GetSimpleType(EUShort)
	case "Int":
		return GetIntType()
	case "UInt":
		return GetSimpleType(EUInt)
	case "Long":
		return GetSimpleType(ELong)
	case "ULong":
		return GetSimpleType(EULong)
	case "Longlong":
		return GetSimpleType(ELongLong)
	case "ULonglong":
		return GetSimpleType(EULongLong)
	case "Float":
		return GetSimpleType(EFloat)
	case "Int128":
		return GetSimpleType(EInt128)
	case "UInt128":
		return GetSimpleType(EUInt128)
	default:
		// Type.cpp:401 assert(0); sticky — no soft invent GetIntType for unknown names
		sessNoteError(s, ErrGeneric)
		return nil
	}
}

// EnabledBuiltinKinds default map (CGOptions.cpp:211–212).
func defaultEnabledBuiltinKinds() map[string]bool {
	return map[string]bool{
		"generic": true,
		"x86":     true,
	}
}

// EnabledBuiltinKind reports whether a single kind is enabled.
// CGOptions.cpp:621–626.
func EnabledBuiltinKind(opts Options, kind string) bool {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return false
	}
	// Apply enable/disable strings if set
	m := defaultEnabledBuiltinKinds()
	if opts.DisableBuiltinKinds != "" {
		for _, k := range SplitStringAny(opts.DisableBuiltinKinds, "|, ") {
			if k != "" {
				m[k] = false
			}
		}
	}
	if opts.EnableBuiltinKinds != "" {
		for _, k := range SplitStringAny(opts.EnableBuiltinKinds, "|, ") {
			if k != "" {
				m[k] = true
			}
		}
	}
	// clang/ppc default off unless enabled
	if v, ok := m[kind]; ok {
		return v
	}
	return false
}

// EnabledBuiltin mirrors CGOptions::enabled_builtin — any kind in "a | b" list.
// CGOptions.cpp:628–637.
func EnabledBuiltin(opts Options, kinds string) bool {
	for _, k := range SplitStringAny(kinds, "|") {
		if EnabledBuiltinKind(opts, strings.TrimSpace(k)) {
			return true
		}
	}
	return false
}

// MakeDummyBlock mirrors Block::make_dummy_block without live CGContext.
// Prefer MakeDummyBlockCG when CGContext is available (fact_in + post_creation).
// Block.cpp:95–110 — empty block, stack push/pop, fact_in, post_creation_analysis.
func MakeDummyBlock(f *Function) *Block {
	return MakeDummyBlockSess(nil, f)
}

// MakeDummyBlockSess is MakeDummyBlock allocating StmID on bag s.
func MakeDummyBlockSess(s *Session, f *Function) *Block {
	// Block always has live Function; sticky no invent dummy body without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Library path without CGContext: still register block (no soft invent body stmts)
	b := &Block{Func: f, blockSize: 0, StmID: AllocStmIDSess(s)}
	f.Blocks = append(f.Blocks, b)
	return b
}

// GenerateParameterListFromString mirrors Function GenerateParameterListFromString.
// Function.cpp:345–363.
// Returns false on assert-path failure (empty list, mid Void, bad type, nil var).
// Fail wipes Param to IncompleteVariables sticky (not bare nil invent empty-complete
// void-param list after partial append / soft re-pick past VariablesComplete(nil)).
func GenerateParameterListFromString(f *Function, params string) bool {
	return GenerateParameterListFromStringSess(nil, f, params)
}

// GenerateParameterListFromStringSess is GenerateParameterListFromString with
// explicit session residual sticky.
func GenerateParameterListFromStringSess(s *Session, f *Function, params string) bool {
	// Function always live; sticky no invent param list without it
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	vs := SplitString(params, ',')
	fail := func() {
		f.Param = IncompleteVariables()
		sessNoteError(s, ErrGeneric)
	}
	// Function.cpp:350 — assert(params_cnt > 0) sticky (no invent empty-complete Param)
	if len(vs) == 0 {
		fail()
		return false
	}
	// Function.cpp:351–352 — sole "Void" → no params
	if len(vs) == 1 && strings.TrimSpace(vs[0]) == "Void" {
		return true
	}
	for i, ts := range vs {
		ts = strings.TrimSpace(ts)
		// Function.cpp:355 — assert(vs[i] != "Void"); no soft invent skip
		if ts == "Void" {
			fail()
			return false
		}
		ty := TypeFromStringSess(s, ts)
		if ty == nil {
			// unsupported type string — assert path
			fail()
			return false
		}
		q := NewCVQualifiers([]bool{false}, []bool{false})
		name := "p_" + itoa(i+1)
		// Function.cpp:359–360 — GenerateParameterVariable; assert(v)
		v := CreateVariableQferSess(s, name, ty, q)
		if v == nil {
			fail()
			return false
		}
		f.Param = append(f.Param, v)
	}
	return true
}

// MakeBuiltinFunction mirrors Function::make_builtin_function.
// Function.cpp:734–771.
func MakeBuiltinFunction(opts Options, probs *Probabilities, r *Rng, list *FunctionList, fmMap *FactMgrMap, line string) *Function {
	return MakeBuiltinFunctionSess(nil, opts, probs, r, list, fmMap, line)
}

// MakeBuiltinFunctionSess is MakeBuiltinFunction with StmID / sticky on bag s.
func MakeBuiltinFunctionSess(s *Session, opts Options, probs *Probabilities, r *Rng, list *FunctionList, fmMap *FactMgrMap, line string) *Function {
	if s == nil && fmMap != nil {
		s = fmMap.Sess
	}
	parts := SplitString(line, ';')
	// trim each
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) == 4 {
		if !EnabledBuiltin(opts, parts[3]) {
			return nil
		}
	} else if len(parts) == 3 {
		if !EnabledBuiltin(opts, "generic") {
			return nil
		}
	} else {
		// Function.cpp:744 — assert(0 && "Invalid builtin function format!") sticky
		sessNoteError(s, ErrGeneric)
		return nil
	}
	ty := TypeFromStringSess(s, parts[0])
	if ty == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	name := parts[1]
	// Function.cpp always has live name token; sticky (no invent empty-name builtin shell)
	if name == "" {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Function.cpp:752 — CVQualifiers::random_qualifiers always has process RNG
	// sticky — no invent fixed non-const RV / NewProbabilities when session missing
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	f := &Function{
		Name:       name,
		ReturnType: ty,
		IsBuiltin:  true,
		BuildState: BuildBuilding,
	}
	// return dummy variable — Probabilities singleton always live; nil probs → 0% quals
	retQ := RandomQualifiersNoContextNoVolatileSess(s, ty, opts, probs, r)
	f.RV = CreateVariableQferSess(s, name+"_rv", ty, retQ)
	if f.RV == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// params from ( ... )
	paramStr := GetSubstring(parts[2], '(', ')')
	// Function.cpp:345+ — assert-path on bad param list; sticky no soft invent empty params
	if !GenerateParameterListFromStringSess(s, f, paramStr) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		return nil
	}
	// Function.cpp:757–758 — FMList.push_back(new FactMgr(f)) at builtin create
	fm := f.ensurePairedFactMgrSess(s)
	if fmMap != nil {
		_ = fmMap.ForFuncSess(s, f)
	}
	_ = fm
	// dummy body (no random generation for builtins)
	// Block.cpp:97 assert(curr_func) — f is live
	f.Body = MakeDummyBlockSess(s, f)
	f.ComputeSummarySess(s, EmptyEffect())
	f.BuildState = BuildBuilt
	f.IsBuilt = true
	if list != nil {
		list.Funcs = append(list.Funcs, f)
	}
	return f
}

// InitializeBuiltinFunctions mirrors Function::initialize_builtin_functions.
// Function.cpp:700–732.
func InitializeBuiltinFunctions(opts Options, probs *Probabilities, r *Rng, list *FunctionList, fmMap *FactMgrMap) int {
	return InitializeBuiltinFunctionsSess(nil, opts, probs, r, list, fmMap)
}

// InitializeBuiltinFunctionsSess is InitializeBuiltinFunctions on bag s.
func InitializeBuiltinFunctionsSess(s *Session, opts Options, probs *Probabilities, r *Rng, list *FunctionList, fmMap *FactMgrMap) int {
	if s == nil && fmMap != nil {
		s = fmMap.Sess
	}
	if !opts.Builtins {
		return 0
	}
	n := 0
	for _, line := range builtinFunctionStrings {
		if MakeBuiltinFunctionSess(s, opts, probs, r, list, fmMap, line) != nil {
			n++
		}
	}
	return n
}
