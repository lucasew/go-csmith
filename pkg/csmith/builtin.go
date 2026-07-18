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
// Type.cpp:370–402.
func TypeFromString(s string) *Type {
	s = strings.TrimSpace(s)
	switch s {
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

// MakeDummyBlock mirrors Block::make_dummy_block — empty body for builtins.
// Block.cpp:95–110.
func MakeDummyBlock(f *Function) *Block {
	if f == nil {
		return nil
	}
	b := &Block{Func: f, blockSize: 0}
	f.Blocks = append(f.Blocks, b)
	return b
}

// GenerateParameterListFromString mirrors Function GenerateParameterListFromString.
// Function.cpp:345–363.
func GenerateParameterListFromString(f *Function, params string) {
	if f == nil {
		return
	}
	vs := SplitString(params, ',')
	if len(vs) == 0 {
		return
	}
	if len(vs) == 1 && strings.TrimSpace(vs[0]) == "Void" {
		return
	}
	for i, ts := range vs {
		ts = strings.TrimSpace(ts)
		if ts == "Void" {
			continue
		}
		ty := TypeFromString(ts)
		if ty == nil {
			continue
		}
		q := NewCVQualifiers([]bool{false}, []bool{false})
		name := "p_" + itoa(i+1)
		// builtin params use p_N names (gensym-style simplified)
		v := CreateVariableQfer(name, ty, q)
		f.Param = append(f.Param, v)
	}
}

// MakeBuiltinFunction mirrors Function::make_builtin_function.
// Function.cpp:734–771.
func MakeBuiltinFunction(opts Options, probs *Probabilities, r *Rng, list *FunctionList, fmMap *FactMgrMap, line string) *Function {
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
		return nil
	}
	ty := TypeFromString(parts[0])
	if ty == nil {
		return nil
	}
	name := parts[1]
	f := &Function{
		Name:       name,
		ReturnType: ty,
		IsBuiltin:  true,
		BuildState: BuildBuilding,
	}
	// return dummy variable
	retQ := NewCVQualifiers([]bool{false}, []bool{false})
	if probs != nil && r != nil {
		retQ = RandomQualifiersNoContextNoVolatile(ty, opts, probs, r)
	}
	f.RV = CreateVariableQfer(name+"_rv", ty, retQ)
	// params from ( ... )
	paramStr := GetSubstring(parts[2], '(', ')')
	GenerateParameterListFromString(f, paramStr)
	// FactMgr
	var fm *FactMgr
	if fmMap != nil {
		fm = fmMap.ForFunc(f)
	} else {
		fm = NewFactMgr(f)
	}
	_ = fm
	// dummy body (no random generation for builtins)
	f.Body = MakeDummyBlock(f)
	f.ComputeSummary(EmptyEffect())
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
	if !opts.Builtins {
		return 0
	}
	n := 0
	for _, line := range builtinFunctionStrings {
		if MakeBuiltinFunction(opts, probs, r, list, fmMap, line) != nil {
			n++
		}
	}
	return n
}
