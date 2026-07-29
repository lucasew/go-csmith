// Upstream: RandomProgramGenerator.cpp CLI ↔ CGOptions.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
//
// Options holds the full generation surface. Drop-in parity uses only FieldCLI
// (CLIArgs + OptionsFromFuzzBlob). FieldLibrary/FieldGoOnly remain settable on
// the library path via Options + Generate; ForDropInParity forces them to
// Defaults for golden comparison.
package csmith

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// FieldKind classifies how an Options field participates in CLI / fuzz / parity.
type FieldKind int

const (
	// FieldCLI: has a golden-accepted CLI flag; parity via CLIArgs.
	FieldCLI FieldKind = iota
	// FieldLibrary: CGOptions field with no (or OMIT) CLI on pin; library-only.
	// Fuzz still randomizes; bodyparity skips when non-default (cannot drive UP).
	FieldLibrary
	// FieldGoOnly: not in CGOptions (MaxGlobals, Argv meta, explicit flags).
	FieldGoOnly
	// FieldMeta: not generation knobs (Argv header, OutputPath path for write).
	FieldMeta
)

// optionsField is one Options member in stable registry order.
type optionsField struct {
	Name string
	Kind FieldKind
	// CLI name without leading dashes, e.g. "jumps", "max-funcs". Empty if none.
	CLI string
	// CLIStyle how to emit when value differs from Defaults().
	CLIStyle cliStyle
	getBool  func(Options) bool
	setBool  func(*Options, bool)
	getInt   func(Options) int
	setInt   func(*Options, int)
	getStr   func(Options) string
	setStr   func(*Options, string)
	// Int fuzz: value = lo + (b % span); span 0 = leave default (not in int plane).
	intLo, intSpan int
	// strPresets for fuzz (index 0 always "").
	strPresets []string
}

type cliStyle int

const (
	cliNone        cliStyle = iota
	cliBoolPair             // --name / --no-name
	cliBoolOn               // only when true: --name
	cliBoolOff              // only when false: --name (e.g. --no-hash-value-printf)
	cliBoolNomain           // --nomain / --main
	cliBoolRetDead          // --no-return-dead-pointer / --return-dead-pointer
	cliBoolTakeUnion        // --take-union-field-addr / --take-no-union-field-addr
	cliInt                  // --flag N
	cliStr                  // --flag val
	cliSeed                 // -s N always
)

// optionsRegistry lists every Options field. Order of bool/int/str planes in the
// fuzz blob follows this registry (bools first, then ints, then strings).
// Adding fields only at end of each plane would require blob version bump; for
// now the blob is versioned (see fuzzBlobVersion).
var optionsRegistry = buildOptionsRegistry()

func buildOptionsRegistry() []optionsField {
	// Helper constructors keep the table readable (one body each; style/kind differ).
	boolField := func(name, cli string, kind FieldKind, style cliStyle, get func(Options) bool, set func(*Options, bool)) optionsField {
		return optionsField{Name: name, Kind: kind, CLI: cli, CLIStyle: style, getBool: get, setBool: set}
	}
	bp := func(name, cli string, kind FieldKind, get func(Options) bool, set func(*Options, bool)) optionsField {
		return boolField(name, cli, kind, cliBoolPair, get, set)
	}
	bon := func(name, cli string, kind FieldKind, get func(Options) bool, set func(*Options, bool)) optionsField {
		return boolField(name, cli, kind, cliBoolOn, get, set)
	}
	boff := func(name, cli string, kind FieldKind, get func(Options) bool, set func(*Options, bool)) optionsField {
		return boolField(name, cli, kind, cliBoolOff, get, set)
	}
	kindB := func(name string, kind FieldKind, get func(Options) bool, set func(*Options, bool)) optionsField {
		return boolField(name, "", kind, cliNone, get, set)
	}
	libB := func(name string, get func(Options) bool, set func(*Options, bool)) optionsField {
		return kindB(name, FieldLibrary, get, set)
	}
	goB := func(name string, get func(Options) bool, set func(*Options, bool)) optionsField {
		return kindB(name, FieldGoOnly, get, set)
	}
	ii := func(name, cli string, kind FieldKind, lo, span int, get func(Options) int, set func(*Options, int)) optionsField {
		st := cliNone
		if cli != "" && kind == FieldCLI {
			st = cliInt
		}
		return optionsField{Name: name, Kind: kind, CLI: cli, CLIStyle: st, getInt: get, setInt: set, intLo: lo, intSpan: span}
	}
	ss := func(name, cli string, kind FieldKind, presets []string, get func(Options) string, set func(*Options, string)) optionsField {
		st := cliNone
		if cli != "" && kind == FieldCLI {
			st = cliStr
		}
		if presets == nil {
			presets = []string{""}
		}
		return optionsField{Name: name, Kind: kind, CLI: cli, CLIStyle: st, getStr: get, setStr: set, strPresets: presets}
	}

	r := make([]optionsField, 0, 140)

	// --- seed ---
	r = append(r, optionsField{
		Name: "Seed", Kind: FieldCLI, CLI: "s", CLIStyle: cliSeed,
	})

	// --- bools (generation features + modes) ---
	// Order is the fuzz bool-bit order.
	r = append(r,
		bp("AcceptArgc", "argc", FieldCLI, func(o Options) bool { return o.AcceptArgc }, func(o *Options, v bool) { o.AcceptArgc = v }),
		bp("Arrays", "arrays", FieldCLI, func(o Options) bool { return o.Arrays }, func(o *Options, v bool) { o.Arrays = v }),
		bp("Bitfields", "bitfields", FieldCLI, func(o Options) bool { return o.Bitfields }, func(o *Options, v bool) { o.Bitfields = v }),
		bp("ComputeHash", "checksum", FieldCLI, func(o Options) bool { return o.ComputeHash }, func(o *Options, v bool) { o.ComputeHash = v }),
		bp("CompoundAssignment", "compound-assignment", FieldCLI, func(o Options) bool { return o.CompoundAssignment }, func(o *Options, v bool) { o.CompoundAssignment = v }),
		bp("Consts", "consts", FieldCLI, func(o Options) bool { return o.Consts }, func(o *Options, v bool) { o.Consts = v }),
		bp("Divs", "divs", FieldCLI, func(o Options) bool { return o.Divs }, func(o *Options, v bool) { o.Divs = v }),
		bp("Muls", "muls", FieldCLI, func(o Options) bool { return o.Muls }, func(o *Options, v bool) { o.Muls = v }),
		bp("EmbeddedAssigns", "embedded-assigns", FieldCLI, func(o Options) bool { return o.EmbeddedAssigns }, func(o *Options, v bool) { o.EmbeddedAssigns = v }),
		bp("CommaOperators", "comma-operators", FieldCLI, func(o Options) bool { return o.CommaOperators }, func(o *Options, v bool) { o.CommaOperators = v }),
		bp("PreIncrOperator", "pre-incr-operator", FieldCLI, func(o Options) bool { return o.PreIncrOperator }, func(o *Options, v bool) { o.PreIncrOperator = v }),
		bp("PreDecrOperator", "pre-decr-operator", FieldCLI, func(o Options) bool { return o.PreDecrOperator }, func(o *Options, v bool) { o.PreDecrOperator = v }),
		bp("PostIncrOperator", "post-incr-operator", FieldCLI, func(o Options) bool { return o.PostIncrOperator }, func(o *Options, v bool) { o.PostIncrOperator = v }),
		bp("PostDecrOperator", "post-decr-operator", FieldCLI, func(o Options) bool { return o.PostDecrOperator }, func(o *Options, v bool) { o.PostDecrOperator = v }),
		bp("UnaryPlusOperator", "unary-plus-operator", FieldCLI, func(o Options) bool { return o.UnaryPlusOperator }, func(o *Options, v bool) { o.UnaryPlusOperator = v }),
		bp("Jumps", "jumps", FieldCLI, func(o Options) bool { return o.Jumps }, func(o *Options, v bool) { o.Jumps = v }),
		bp("LongLong", "longlong", FieldCLI, func(o Options) bool { return o.LongLong }, func(o *Options, v bool) { o.LongLong = v }),
		bp("Int8", "int8", FieldCLI, func(o Options) bool { return o.Int8 }, func(o *Options, v bool) { o.Int8 = v }),
		bp("UInt8", "uint8", FieldCLI, func(o Options) bool { return o.UInt8 }, func(o *Options, v bool) { o.UInt8 = v }),
		bp("EnableFloat", "float", FieldCLI, func(o Options) bool { return o.EnableFloat }, func(o *Options, v bool) { o.EnableFloat = v }),
		bp("Math64", "math64", FieldCLI, func(o Options) bool { return o.Math64 }, func(o *Options, v bool) { o.Math64 = v }),
		bp("InlineFunction", "inline-function", FieldCLI, func(o Options) bool { return o.InlineFunction }, func(o *Options, v bool) { o.InlineFunction = v }),
		bp("Pointers", "pointers", FieldCLI, func(o Options) bool { return o.Pointers }, func(o *Options, v bool) { o.Pointers = v }),
		bp("Structs", "structs", FieldCLI, func(o Options) bool { return o.Structs }, func(o *Options, v bool) { o.Structs = v }),
		bp("ReturnStructs", "return-structs", FieldCLI, func(o Options) bool { return o.ReturnStructs }, func(o *Options, v bool) { o.ReturnStructs = v }),
		bp("ArgStructs", "arg-structs", FieldCLI, func(o Options) bool { return o.ArgStructs }, func(o *Options, v bool) { o.ArgStructs = v }),
		bp("Unions", "unions", FieldCLI, func(o Options) bool { return o.Unions }, func(o *Options, v bool) { o.Unions = v }),
		bp("ReturnUnions", "return-unions", FieldCLI, func(o Options) bool { return o.ReturnUnions }, func(o *Options, v bool) { o.ReturnUnions = v }),
		bp("ArgUnions", "arg-unions", FieldCLI, func(o Options) bool { return o.ArgUnions }, func(o *Options, v bool) { o.ArgUnions = v }),
		// Upstream: --take-union-field-addr | --take-no-union-field-addr (not --no-take-…).
		optionsField{Name: "TakeUnionFieldAddr", Kind: FieldCLI, CLI: "take-union-field-addr", CLIStyle: cliBoolTakeUnion,
			getBool: func(o Options) bool { return o.TakeUnionFieldAddr }, setBool: func(o *Options, v bool) { o.TakeUnionFieldAddr = v }},
		bp("VolStructUnionFields", "vol-struct-union-fields", FieldCLI, func(o Options) bool { return o.VolStructUnionFields }, func(o *Options, v bool) { o.VolStructUnionFields = v }),
		bp("ConstStructUnionFields", "const-struct-union-fields", FieldCLI, func(o Options) bool { return o.ConstStructUnionFields }, func(o *Options, v bool) { o.ConstStructUnionFields = v }),
		bp("Volatiles", "volatiles", FieldCLI, func(o Options) bool { return o.Volatiles }, func(o *Options, v bool) { o.Volatiles = v }),
		bp("VolatilePointers", "volatile-pointers", FieldCLI, func(o Options) bool { return o.VolatilePointers }, func(o *Options, v bool) { o.VolatilePointers = v }),
		bp("ConstPointers", "const-pointers", FieldCLI, func(o Options) bool { return o.ConstPointers }, func(o *Options, v bool) { o.ConstPointers = v }),
		bp("GlobalVariables", "global-variables", FieldCLI, func(o Options) bool { return o.GlobalVariables }, func(o *Options, v bool) { o.GlobalVariables = v }),
		bp("AddrTakenOfLocals", "addr-taken-of-locals", FieldCLI, func(o Options) bool { return o.AddrTakenOfLocals }, func(o *Options, v bool) { o.AddrTakenOfLocals = v }),
		bp("StrictConstArrays", "strict-const-arrays", FieldCLI, func(o Options) bool { return o.StrictConstArrays }, func(o *Options, v bool) { o.StrictConstArrays = v }),
		bp("DanglingGlobalPointers", "dangling-global-pointers", FieldCLI, func(o Options) bool { return o.DanglingGlobalPointers }, func(o *Options, v bool) { o.DanglingGlobalPointers = v }),
		bp("Builtins", "builtins", FieldCLI, func(o Options) bool { return o.Builtins }, func(o *Options, v bool) { o.Builtins = v }),
		bp("ForceGlobalsStatic", "force-globals-static", FieldCLI, func(o Options) bool { return o.ForceGlobalsStatic }, func(o *Options, v bool) { o.ForceGlobalsStatic = v }),
		bp("ForceNonUniformArrayInit", "force-non-uniform-arrays", FieldCLI, func(o Options) bool { return o.ForceNonUniformArrayInit }, func(o *Options, v bool) { o.ForceNonUniformArrayInit = v }),
		bp("Int128", "int128", FieldCLI, func(o Options) bool { return o.Int128 }, func(o *Options, v bool) { o.Int128 = v }),
		bp("UInt128", "uint128", FieldCLI, func(o Options) bool { return o.UInt128 }, func(o *Options, v bool) { o.UInt128 = v }),
		bp("BinaryConstant", "binary-constant", FieldCLI, func(o Options) bool { return o.BinaryConstant }, func(o *Options, v bool) { o.BinaryConstant = v }),
		bp("MathNoTmp", "math-notmp", FieldCLI, func(o Options) bool { return o.MathNoTmp }, func(o *Options, v bool) { o.MathNoTmp = v }),
		bp("FunctionAttributes", "function-attributes", FieldCLI, func(o Options) bool { return o.FunctionAttributes }, func(o *Options, v bool) { o.FunctionAttributes = v }),
		bp("TypeAttributes", "type-attributes", FieldCLI, func(o Options) bool { return o.TypeAttributes }, func(o *Options, v bool) { o.TypeAttributes = v }),
		bp("LabelAttributes", "label-attributes", FieldCLI, func(o Options) bool { return o.LabelAttributes }, func(o *Options, v bool) { o.LabelAttributes = v }),
		bp("VariableAttributes", "variable-attributes", FieldCLI, func(o Options) bool { return o.VariableAttributes }, func(o *Options, v bool) { o.VariableAttributes = v }),
		bp("SafeMath", "safe-math", FieldCLI, func(o Options) bool { return o.SafeMath }, func(o *Options, v bool) { o.SafeMath = v }),
		bp("PackedStruct", "packed-struct", FieldCLI, func(o Options) bool { return o.PackedStruct }, func(o *Options, v bool) { o.PackedStruct = v }),
		bp("Paranoid", "paranoid", FieldCLI, func(o Options) bool { return o.Paranoid }, func(o *Options, v bool) { o.Paranoid = v }),
		bp("FixedStructFields", "fixed-struct-fields", FieldCLI, func(o Options) bool { return o.FixedStructFields }, func(o *Options, v bool) { o.FixedStructFields = v }),

		bon("ExpandStruct", "expand-struct", FieldCLI, func(o Options) bool { return o.ExpandStruct }, func(o *Options, v bool) { o.ExpandStruct = v }),
		bon("AccessOnce", "enable-access-once", FieldCLI, func(o Options) bool { return o.AccessOnce }, func(o *Options, v bool) { o.AccessOnce = v }),
		bon("StrictVolatileRule", "strict-volatile-rule", FieldCLI, func(o Options) bool { return o.StrictVolatileRule }, func(o *Options, v bool) { o.StrictVolatileRule = v }),
		bon("RandomRandom", "random-random", FieldCLI, func(o Options) bool { return o.RandomRandom }, func(o *Options, v bool) { o.RandomRandom = v }),
		bon("BlindCheckGlobal", "check-global", FieldCLI, func(o Options) bool { return o.BlindCheckGlobal }, func(o *Options, v bool) { o.BlindCheckGlobal = v }),
		bon("StepHashByStmt", "step-hash-by-stmt", FieldCLI, func(o Options) bool { return o.StepHashByStmt }, func(o *Options, v bool) { o.StepHashByStmt = v }),
		bon("ConstAsCondition", "const-as-condition", FieldCLI, func(o Options) bool { return o.ConstAsCondition }, func(o *Options, v bool) { o.ConstAsCondition = v }),
		bon("MatchExactQualifiers", "match-exact-qualifiers", FieldCLI, func(o Options) bool { return o.MatchExactQualifiers }, func(o *Options, v bool) { o.MatchExactQualifiers = v }),
		bon("FreshArrayCtrlVarNames", "fresh-array-ctrl-var-names", FieldCLI, func(o Options) bool { return o.FreshArrayCtrlVarNames }, func(o *Options, v bool) { o.FreshArrayCtrlVarNames = v }),
		bon("IdentifyWrappers", "identify-wrappers", FieldCLI, func(o Options) bool { return o.IdentifyWrappers }, func(o *Options, v bool) { o.IdentifyWrappers = v }),
		bon("MarkMutableConst", "mark-mutable-const", FieldCLI, func(o Options) bool { return o.MarkMutableConst }, func(o *Options, v bool) { o.MarkMutableConst = v }),
		bon("StrictFloat", "strict-float", FieldCLI, func(o Options) bool { return o.StrictFloat }, func(o *Options, v bool) { o.StrictFloat = v }),
		bon("CompactOutput", "compact-output", FieldCLI, func(o Options) bool { return o.CompactOutput }, func(o *Options, v bool) { o.CompactOutput = v }),
		bon("PrefixName", "prefix-name", FieldCLI, func(o Options) bool { return o.PrefixName }, func(o *Options, v bool) { o.PrefixName = v }),
		bon("SequenceNamePrefix", "sequence-name-prefix", FieldCLI, func(o Options) bool { return o.SequenceNamePrefix }, func(o *Options, v bool) { o.SequenceNamePrefix = v }),
		bon("CompatibleCheck", "compatible-check", FieldCLI, func(o Options) bool { return o.CompatibleCheck }, func(o *Options, v bool) { o.CompatibleCheck = v }),
		bon("Klee", "klee", FieldCLI, func(o Options) bool { return o.Klee }, func(o *Options, v bool) { o.Klee = v }),
		bon("Crest", "crest", FieldCLI, func(o Options) bool { return o.Crest }, func(o *Options, v bool) { o.Crest = v }),
		bon("CComp", "ccomp", FieldCLI, func(o Options) bool { return o.CComp }, func(o *Options, v bool) { o.CComp = v }),
		bon("CoverageTest", "coverage-test", FieldCLI, func(o Options) bool { return o.CoverageTest }, func(o *Options, v bool) { o.CoverageTest = v }),
		bon("NoDeltaReduction", "no-delta-reduction", FieldCLI, func(o Options) bool { return o.NoDeltaReduction }, func(o *Options, v bool) { o.NoDeltaReduction = v }),
		bon("Concise", "concise", FieldCLI, func(o Options) bool { return o.Concise }, func(o *Options, v bool) { o.Concise = v }),
		bon("Quiet", "quiet", FieldCLI, func(o Options) bool { return o.Quiet }, func(o *Options, v bool) { o.Quiet = v }),
		bon("LangCPP", "lang-cpp", FieldCLI, func(o Options) bool { return o.LangCPP }, func(o *Options, v bool) { o.LangCPP = v }),
		bon("CPP11", "cpp11", FieldCLI, func(o Options) bool { return o.CPP11 }, func(o *Options, v bool) { o.CPP11 = v }),
		bon("FastExecution", "fast-execution", FieldCLI, func(o Options) bool { return o.FastExecution }, func(o *Options, v bool) { o.FastExecution = v }),
		bon("DFSExhaustive", "dfs-exhaustive", FieldCLI, func(o Options) bool { return o.DFSExhaustive }, func(o *Options, v bool) { o.DFSExhaustive = v }),

		// Special bool emission
		optionsField{Name: "NoMain", Kind: FieldCLI, CLI: "nomain", CLIStyle: cliBoolNomain,
			getBool: func(o Options) bool { return o.NoMain }, setBool: func(o *Options, v bool) { o.NoMain = v }},
		optionsField{Name: "NoReturnDeadPointer", Kind: FieldCLI, CLI: "return-dead-pointer", CLIStyle: cliBoolRetDead,
			getBool: func(o Options) bool { return o.NoReturnDeadPointer }, setBool: func(o *Options, v bool) { o.NoReturnDeadPointer = v }},
		boff("HashValuePrintf", "no-hash-value-printf", FieldCLI,
			func(o Options) bool { return o.HashValuePrintf }, func(o *Options, v bool) { o.HashValuePrintf = v }),
		boff("SignedCharIndex", "no-signed-char-index", FieldCLI,
			func(o Options) bool { return o.SignedCharIndex }, func(o *Options, v bool) { o.SignedCharIndex = v }),

		// Library-only bools (CGOptions exists; CLI OMIT or absent on pin)
		libB("DepthProtect", func(o Options) bool { return o.DepthProtect }, func(o *Options, v bool) { o.DepthProtect = v }),
		libB("WrapVolatiles", func(o Options) bool { return o.WrapVolatiles }, func(o *Options, v bool) { o.WrapVolatiles = v }),
		libB("AllowConstVolatile", func(o Options) bool { return o.AllowConstVolatile }, func(o *Options, v bool) { o.AllowConstVolatile = v }),
		// RandomBased: flipped false by --dfs-exhaustive; no --random-based on golden.
		libB("RandomBased", func(o Options) bool { return o.RandomBased }, func(o *Options, v bool) { o.RandomBased = v }),

		// Go-only bools
		goB("IntSizeExplicit", func(o Options) bool { return o.IntSizeExplicit }, func(o *Options, v bool) { o.IntSizeExplicit = v }),
		goB("PointerExplicit", func(o Options) bool { return o.PointerExplicit }, func(o *Options, v bool) { o.PointerExplicit = v }),
	)

	// --- ints ---
	// Fuzz span keeps generation tractable (not full int range).
	r = append(r,
		ii("MaxFuncs", "max-funcs", FieldCLI, 1, 16, func(o Options) int { return o.MaxFuncs }, func(o *Options, v int) { o.MaxFuncs = v }),
		// max_params: CGOptions field, NO golden CLI (invalid option) → library
		ii("MaxParams", "", FieldLibrary, 1, 8, func(o Options) int { return o.MaxParams }, func(o *Options, v int) { o.MaxParams = v }),
		ii("Func1MaxParams", "func1_max_params", FieldCLI, 0, 8, func(o Options) int { return o.Func1MaxParams }, func(o *Options, v int) { o.Func1MaxParams = v }),
		ii("MaxBlockSize", "max-block-size", FieldCLI, 1, 8, func(o Options) int { return o.MaxBlockSize }, func(o *Options, v int) { o.MaxBlockSize = v }),
		ii("MaxBlockDepth", "max-block-depth", FieldCLI, 1, 8, func(o Options) int { return o.MaxBlockDepth }, func(o *Options, v int) { o.MaxBlockDepth = v }),
		ii("MaxExprComplexity", "max-expr-complexity", FieldCLI, 1, 16, func(o Options) int { return o.MaxExprComplexity }, func(o *Options, v int) { o.MaxExprComplexity = v }),
		ii("MaxStructFields", "max-struct-fields", FieldCLI, 1, 16, func(o Options) int { return o.MaxStructFields }, func(o *Options, v int) { o.MaxStructFields = v }),
		ii("MaxUnionFields", "max-union-fields", FieldCLI, 1, 8, func(o Options) int { return o.MaxUnionFields }, func(o *Options, v int) { o.MaxUnionFields = v }),
		ii("MaxNestedStructLevel", "max-nested-struct-level", FieldCLI, 1, 5, func(o Options) int { return o.MaxNestedStructLevel }, func(o *Options, v int) { o.MaxNestedStructLevel = v }),
		ii("MaxPointerDepth", "max-pointer-depth", FieldCLI, 1, 8, func(o Options) int { return o.MaxPointerDepth }, func(o *Options, v int) { o.MaxPointerDepth = v }),
		ii("MaxArrayDim", "max-array-dim", FieldCLI, 1, 5, func(o Options) int { return o.MaxArrayDim }, func(o *Options, v int) { o.MaxArrayDim = v }),
		ii("MaxArrayLenPerDim", "max-array-len-per-dim", FieldCLI, 1, 16, func(o Options) int { return o.MaxArrayLenPerDim }, func(o *Options, v int) { o.MaxArrayLenPerDim = v }),
		// max_array_length: no golden CLI
		ii("MaxArrayLength", "", FieldLibrary, 1, 64, func(o Options) int { return o.MaxArrayLength }, func(o *Options, v int) { o.MaxArrayLength = v }),
		ii("MaxArrayNumInLoop", "", FieldLibrary, 0, 8, func(o Options) int { return o.MaxArrayNumInLoop }, func(o *Options, v int) { o.MaxArrayNumInLoop = v }),
		ii("MaxExhaustiveDepth", "max-exhaustive-depth", FieldCLI, 1, 16, func(o Options) int { return o.MaxExhaustiveDepth }, func(o *Options, v int) { o.MaxExhaustiveDepth = v }),
		ii("InlineFunctionProb", "inline-function-prob", FieldCLI, 0, 101, func(o Options) int { return o.InlineFunctionProb }, func(o *Options, v int) { o.InlineFunctionProb = v }),
		ii("BuiltinFunctionProb", "builtin-function-prob", FieldCLI, 0, 101, func(o Options) int { return o.BuiltinFunctionProb }, func(o *Options, v int) { o.BuiltinFunctionProb = v }),
		ii("ArrayOOBProb", "array-oob-prob", FieldCLI, 0, 101, func(o Options) int { return o.ArrayOOBProb }, func(o *Options, v int) { o.ArrayOOBProb = v }),
		ii("NullPointerDerefProb", "null-ptr-deref-prob", FieldCLI, 0, 101, func(o Options) int { return o.NullPointerDerefProb }, func(o *Options, v int) { o.NullPointerDerefProb = v }),
		ii("DeadPointerDerefProb", "dangling-ptr-deref-prob", FieldCLI, 0, 101, func(o Options) int { return o.DeadPointerDerefProb }, func(o *Options, v int) { o.DeadPointerDerefProb = v }),
		ii("StopByStmt", "stop-by-stmt", FieldCLI, -1, 65, func(o Options) int { return o.StopByStmt }, func(o *Options, v int) { o.StopByStmt = v }), // -1..63 via scale
		ii("CoverageTestSize", "coverage-test-size", FieldCLI, 1, 64, func(o Options) int { return o.CoverageTestSize }, func(o *Options, v int) { o.CoverageTestSize = v }),
		ii("MaxSplitFiles", "max-split-files", FieldCLI, 0, 5, func(o Options) int { return o.MaxSplitFiles }, func(o *Options, v int) { o.MaxSplitFiles = v }),
		ii("IntSize", "int-size", FieldCLI, 1, 9, func(o Options) int { return o.IntSize }, func(o *Options, v int) { o.IntSize = v }),
		ii("PointerSize", "ptr-size", FieldCLI, 2, 7, func(o Options) int { return o.PointerSize }, func(o *Options, v int) { o.PointerSize = v }), // 2..8
		ii("InterestedFacts", "", FieldLibrary, 0, 8, func(o Options) int { return o.InterestedFacts }, func(o *Options, v int) { o.InterestedFacts = v }),
		ii("MaxGlobals", "", FieldGoOnly, 0, 33, func(o Options) int { return o.MaxGlobals }, func(o *Options, v int) { o.MaxGlobals = v }),
	)

	// --- strings (fuzz as small preset index; empty = default) ---
	// Keep empty for dump/delta so generation still produces a program body by default;
	// non-empty presets only for modes that still generate (partial-expand tokens etc.).
	r = append(r,
		ss("PlatformInfoPath", "", FieldLibrary, []string{"", defaultPlatformInfoPath},
			func(o Options) string { return o.PlatformInfoPath }, func(o *Options, v string) { o.PlatformInfoPath = v }),
		ss("SplitFilesDir", "split-files-dir", FieldCLI, []string{"", "./output"},
			func(o Options) string { return o.SplitFilesDir }, func(o *Options, v string) { o.SplitFilesDir = v }),
		ss("StructOutput", "struct-output", FieldCLI, []string{""},
			func(o Options) string { return o.StructOutput }, func(o *Options, v string) { o.StructOutput = v }),
		ss("DFSDebugSequence", "dfs-debug-sequence", FieldCLI, []string{""},
			func(o Options) string { return o.DFSDebugSequence }, func(o *Options, v string) { o.DFSDebugSequence = v }),
		ss("PartialExpand", "partial-expand", FieldCLI, []string{"", "0", "1"},
			func(o Options) string { return o.PartialExpand }, func(o *Options, v string) { o.PartialExpand = v }),
		// Delta/dump force empty in SanitizeForBodyParityFuzz; still registered.
		ss("DeltaMonitor", "delta-monitor", FieldCLI, []string{""},
			func(o Options) string { return o.DeltaMonitor }, func(o *Options, v string) { o.DeltaMonitor = v }),
		ss("DeltaOutput", "delta-output", FieldCLI, []string{""},
			func(o Options) string { return o.DeltaOutput }, func(o *Options, v string) { o.DeltaOutput = v }),
		ss("GoDelta", "go-delta", FieldCLI, []string{""},
			func(o Options) string { return o.GoDelta }, func(o *Options, v string) { o.GoDelta = v }),
		ss("DeltaInput", "delta-input", FieldCLI, []string{""},
			func(o Options) string { return o.DeltaInput }, func(o *Options, v string) { o.DeltaInput = v }),
		ss("ProbabilityConfiguration", "probability-configuration", FieldCLI, []string{""},
			func(o Options) string { return o.ProbabilityConfiguration }, func(o *Options, v string) { o.ProbabilityConfiguration = v }),
		ss("DumpDefaultProbabilities", "dump-default-probabilities", FieldCLI, []string{""},
			func(o Options) string { return o.DumpDefaultProbabilities }, func(o *Options, v string) { o.DumpDefaultProbabilities = v }),
		ss("DumpRandomProbabilities", "dump-random-probabilities", FieldCLI, []string{""},
			func(o Options) string { return o.DumpRandomProbabilities }, func(o *Options, v string) { o.DumpRandomProbabilities = v }),
		ss("SafeMathWrappers", "safe-math-wrappers", FieldCLI, []string{""},
			func(o Options) string { return o.SafeMathWrappers }, func(o *Options, v string) { o.SafeMathWrappers = v }),
		ss("MonitorFuncs", "monitor-funcs", FieldCLI, []string{""},
			func(o Options) string { return o.MonitorFuncs }, func(o *Options, v string) { o.MonitorFuncs = v }),
		ss("EnableBuiltinKinds", "enable-builtin-kinds", FieldCLI, []string{""},
			func(o Options) string { return o.EnableBuiltinKinds }, func(o *Options, v string) { o.EnableBuiltinKinds = v }),
		ss("DisableBuiltinKinds", "disable-builtin-kinds", FieldCLI, []string{""},
			func(o Options) string { return o.DisableBuiltinKinds }, func(o *Options, v string) { o.DisableBuiltinKinds = v }),
		ss("VolTestsMach", "", FieldLibrary, []string{"", "x86", "x86_64"},
			func(o Options) string { return o.VolTestsMach }, func(o *Options, v string) { o.VolTestsMach = v }),
		// Meta
		optionsField{Name: "OutputPath", Kind: FieldMeta, CLIStyle: cliNone,
			getStr: func(o Options) string { return o.OutputPath }, setStr: func(o *Options, v string) { o.OutputPath = v }, strPresets: []string{""}},
		optionsField{Name: "Argv", Kind: FieldMeta, CLIStyle: cliNone},
	)

	return r
}

// CLIArgs returns argv[1..] for golden / our CLI. Only FieldCLI fields are
// emitted, and only when they differ from Defaults(). Always includes -s.
//
// Does NOT emit library-only knobs (max-params, depth-protect, wrap-volatiles,
// …) — golden rejects them as "invalid option".
func (o Options) CLIArgs() []string {
	d := Defaults()
	args := make([]string, 0, 48)
	args = append(args, "-s", strconv.FormatUint(o.Seed, 10))

	for _, f := range optionsRegistry {
		if f.Kind != FieldCLI {
			continue
		}
		switch f.CLIStyle {
		case cliSeed:
			// already emitted
		case cliBoolPair:
			got, def := f.getBool(o), f.getBool(d)
			if got == def {
				continue
			}
			if got {
				args = append(args, "--"+f.CLI)
			} else {
				args = append(args, "--no-"+f.CLI)
			}
		case cliBoolOn:
			if f.getBool(o) != f.getBool(d) && f.getBool(o) {
				args = append(args, "--"+f.CLI)
			}
		case cliBoolOff:
			// emit flag when value is false and default was true (or vice versa for rare cases)
			if f.getBool(o) != f.getBool(d) && !f.getBool(o) {
				args = append(args, "--"+f.CLI)
			}
		case cliBoolNomain:
			if o.NoMain != d.NoMain {
				if o.NoMain {
					args = append(args, "--nomain")
				} else {
					args = append(args, "--main")
				}
			}
		case cliBoolRetDead:
			if o.NoReturnDeadPointer != d.NoReturnDeadPointer {
				if o.NoReturnDeadPointer {
					args = append(args, "--no-return-dead-pointer")
				} else {
					args = append(args, "--return-dead-pointer")
				}
			}
		case cliBoolTakeUnion:
			// RandomProgramGenerator.cpp — --take-union-field-addr | --take-no-union-field-addr
			if o.TakeUnionFieldAddr != d.TakeUnionFieldAddr {
				if o.TakeUnionFieldAddr {
					args = append(args, "--take-union-field-addr")
				} else {
					args = append(args, "--take-no-union-field-addr")
				}
			}
		case cliInt:
			if f.getInt(o) != f.getInt(d) {
				args = append(args, "--"+f.CLI, strconv.Itoa(f.getInt(o)))
			}
		case cliStr:
			got := f.getStr(o)
			def := f.getStr(d)
			if got != def && got != "" {
				args = append(args, "--"+f.CLI, got)
			}
		}
	}
	// When dfs-exhaustive, golden sets random_based false; no separate flag.
	return args
}

// UpstreamParityGaps lists field names that differ from Defaults() but cannot
// be expressed on golden CLI (library / go-only). Prefer ForDropInParity so
// bodyparity never needs this; kept for diagnostics.
func (o Options) UpstreamParityGaps() []string {
	d := Defaults()
	var gaps []string
	for _, f := range optionsRegistry {
		if f.Kind != FieldLibrary && f.Kind != FieldGoOnly {
			continue
		}
		switch {
		case f.getBool != nil:
			if f.getBool(o) != f.getBool(d) {
				gaps = append(gaps, f.Name)
			}
		case f.getInt != nil:
			if f.getInt(o) != f.getInt(d) {
				gaps = append(gaps, f.Name)
			}
		case f.getStr != nil:
			if f.getStr(o) != f.getStr(d) {
				gaps = append(gaps, f.Name)
			}
		}
	}
	return gaps
}

// ForDropInParity returns a copy safe for golden body comparison:
// library / go-only / meta fields forced to Defaults(); dump/delta/split cleared.
// Seed and all FieldCLI values are preserved. Call before Generate + CLIArgs
// when the promise is drop-in vs csmith(1).
func (o Options) ForDropInParity() Options {
	d := Defaults()
	out := o
	for _, f := range optionsRegistry {
		if f.Kind == FieldCLI {
			continue
		}
		switch {
		case f.setBool != nil && f.getBool != nil:
			f.setBool(&out, f.getBool(d))
		case f.setInt != nil && f.getInt != nil:
			f.setInt(&out, f.getInt(d))
		case f.setStr != nil && f.getStr != nil:
			f.setStr(&out, f.getStr(d))
		}
	}
	// DFS CLI forces random_based false (upstream parser); RandomBased is library-kind.
	if out.DFSExhaustive {
		out.RandomBased = false
	} else {
		out.RandomBased = true
	}
	return SanitizeForBodyParityFuzz(out)
}

// fuzzBlobMagic marks a drop-in multi-field blob (not a raw LE seed).
const (
	fuzzBlobMagic   = byte(0xFF)
	fuzzBlobVersion = byte(2)
	// fuzzIntKeepDefault: int-plane byte meaning "leave Defaults()".
	// Needed when Defaults() sit outside the compact fuzz span (e.g.
	// MaxExhaustiveDepth=-1, CoverageTestSize=500) so FuzzBlobFromOptions
	// → OptionsFromFuzzBlob is a true round-trip for drop-in roots.
	fuzzIntKeepDefault = byte(0xFF)
)

// Drop-in fuzz blob v2 (bodyparity / flag-surface loops):
//
//	[0]      magic 0xFF
//	[1]      version = 2
//	[2:10]   seed uint64 LE
//	[10:…]   bool bits — FieldCLI bools only
//	[…]      uint8 knobs — FieldCLI ints only
//	[…]      uint8 presets — FieldCLI strings only
//
// Legacy: any blob without magic → first 8 bytes = seed, Defaults() flags.
// Library-only CGOptions are never randomized here (always Defaults) so every
// case is expressible to golden via CLIArgs.
func isFuzzBlobV2(b []byte) bool {
	return len(b) >= 10 && b[0] == fuzzBlobMagic && b[1] == fuzzBlobVersion
}

// OptionsFromFuzzBlob decodes a drop-in fuzz payload into Options.
// Only FieldCLI fields are applied; then ForDropInParity sanitizes.
func OptionsFromFuzzBlob(b []byte) Options {
	o := Defaults()
	if len(b) == 0 {
		return o.ForDropInParity()
	}
	if !isFuzzBlobV2(b) {
		var seedBuf [8]byte
		copy(seedBuf[:], b)
		o.Seed = binary.LittleEndian.Uint64(seedBuf[:])
		return o.ForDropInParity()
	}
	o.Seed = binary.LittleEndian.Uint64(b[2:10])
	off := 10

	bools, ints, strs := dropInPlanes()
	nb := (len(bools) + 7) / 8
	if len(b) >= off+nb {
		bits := b[off : off+nb]
		for i, f := range bools {
			if f.setBool == nil {
				continue
			}
			on := bits[i/8]&(1<<uint(i%8)) != 0
			f.setBool(&o, on)
		}
		off += nb
	}
	if len(b) >= off+len(ints) {
		for i, f := range ints {
			if f.setInt == nil || f.intSpan <= 0 {
				continue
			}
			// 0xFF = keep Defaults() for this int (see FuzzBlobFromOptions).
			if b[off+i] == fuzzIntKeepDefault {
				continue
			}
			f.setInt(&o, f.intLo+int(b[off+i])%f.intSpan)
		}
		off += len(ints)
	}
	if len(b) >= off+len(strs) {
		for i, f := range strs {
			if f.setStr == nil || len(f.strPresets) == 0 {
				continue
			}
			idx := int(b[off+i]) % len(f.strPresets)
			f.setStr(&o, f.strPresets[idx])
		}
	}

	if o.Func1MaxParams > o.MaxParams {
		// MaxParams is library-only (always default in drop-in); still clamp.
		o.Func1MaxParams = o.MaxParams
	}
	if o.MaxFuncs < 1 {
		o.MaxFuncs = 1
	}
	if o.MaxBlockSize < 1 {
		o.MaxBlockSize = 1
	}
	if o.MaxBlockDepth < 1 {
		o.MaxBlockDepth = 1
	}
	if o.DFSExhaustive {
		o.RandomBased = false
		if o.MaxExhaustiveDepth <= 0 {
			o.MaxExhaustiveDepth = 1
		}
	}
	return o.ForDropInParity()
}

// FuzzBlobFromOptions encodes a drop-in v2 blob (FieldCLI planes only).
func FuzzBlobFromOptions(o Options) []byte {
	o = o.ForDropInParity()
	bools, ints, strs := dropInPlanes()
	nb := (len(bools) + 7) / 8
	b := make([]byte, 2+8+nb+len(ints)+len(strs))
	b[0] = fuzzBlobMagic
	b[1] = fuzzBlobVersion
	binary.LittleEndian.PutUint64(b[2:10], o.Seed)
	off := 10
	for i, f := range bools {
		if f.getBool != nil && f.getBool(o) {
			b[off+i/8] |= 1 << uint(i%8)
		}
	}
	off += nb
	d := Defaults()
	for i, f := range ints {
		if f.getInt == nil || f.intSpan <= 0 {
			continue
		}
		// Preserve Defaults exactly when the field matches (even if the
		// default is outside [intLo, intLo+intSpan), e.g. MaxExhaustiveDepth).
		if f.getInt(o) == f.getInt(d) {
			b[off+i] = fuzzIntKeepDefault
			continue
		}
		x := f.getInt(o) - f.intLo
		if x < 0 {
			x = 0
		}
		if f.intSpan > 0 && x >= f.intSpan {
			x = f.intSpan - 1
		}
		// Reserve 0xFF as keep-default sentinel.
		if x > 254 {
			x = 254
		}
		b[off+i] = byte(x)
	}
	off += len(ints)
	for i, f := range strs {
		if f.getStr == nil || len(f.strPresets) == 0 {
			continue
		}
		got := f.getStr(o)
		idx := 0
		for j, p := range f.strPresets {
			if p == got {
				idx = j
				break
			}
		}
		b[off+i] = byte(idx)
	}
	return b
}

// fieldPlanes splits the registry into bool/int/str planes.
// cliOnly keeps FieldCLI only (drop-in / bodyparity fuzz surface).
func fieldPlanes(cliOnly bool) (bools, ints, strs []optionsField) {
	for _, f := range optionsRegistry {
		if cliOnly && f.Kind != FieldCLI {
			continue
		}
		switch {
		case f.getBool != nil && f.setBool != nil:
			bools = append(bools, f)
		case f.getInt != nil && f.setInt != nil:
			ints = append(ints, f)
		case f.getStr != nil && f.setStr != nil:
			strs = append(strs, f)
		}
	}
	return
}

func registryPlanes() (bools, ints, strs []optionsField) { return fieldPlanes(false) }

// dropInPlanes is the bodyparity / drop-in fuzz surface (golden CLI only).
func dropInPlanes() (bools, ints, strs []optionsField) { return fieldPlanes(true) }

// SanitizeForBodyParityFuzz clears modes that cannot produce a single-stdout
// program body for comparison (dump-and-exit, delta tools, multi-file split).
// Also collapses golden-CLI mutual exclusions so drop-in fuzz spends less time
// on shared skip cases (upstream prints "error: options conflict" then exits).
func SanitizeForBodyParityFuzz(o Options) Options {
	o.DumpDefaultProbabilities = ""
	o.DumpRandomProbabilities = ""
	o.ProbabilityConfiguration = ""
	o.DeltaMonitor = ""
	o.DeltaOutput = ""
	o.GoDelta = ""
	o.DeltaInput = ""
	o.StructOutput = ""
	o.OutputPath = ""
	o.Argv = nil
	o.MaxSplitFiles = 0
	o.SplitFilesDir = ""
	if o.PlatformInfoPath == "" {
		o.PlatformInfoPath = defaultPlatformInfoPath
	}
	// Upstream: --cpp11 only valid with --lang-cpp.
	if o.CPP11 {
		o.LangCPP = true
	}
	// Upstream: only one of --klee / --crest / --coverage-test.
	nExt := 0
	if o.Klee {
		nExt++
	}
	if o.Crest {
		nExt++
	}
	if o.CoverageTest {
		nExt++
	}
	if nExt > 1 {
		// Keep the first in priority order; drop the rest.
		if o.Klee {
			o.Crest, o.CoverageTest = false, false
		} else if o.Crest {
			o.CoverageTest = false
		}
	}
	// Upstream: exhaustive mode doesn't support klee|crest|coverage-test.
	if o.DFSExhaustive {
		o.Klee, o.Crest, o.CoverageTest = false, false, false
		// Sequence prefix is DFS-only.
	} else {
		o.SequenceNamePrefix = false
	}
	// Cap DFS so drop-in fuzz stays finite (full exhaustive can hang for minutes+
	// and golden may segfault; depth>2 routinely burns the whole campaign budget).
	if o.DFSExhaustive {
		if o.MaxExhaustiveDepth < 0 {
			o.MaxExhaustiveDepth = 0
		}
		if o.MaxExhaustiveDepth > 2 {
			o.MaxExhaustiveDepth = 2
		}
		if o.MaxFuncs > 3 {
			o.MaxFuncs = 3
		}
		if o.MaxBlockSize > 2 {
			o.MaxBlockSize = 2
		}
		if o.MaxBlockDepth > 3 {
			o.MaxBlockDepth = 3
		}
	}
	// Platform sizes: only 2/4/8 (int) or 4/8 (ptr); else leave Defaults (4).
	switch o.IntSize {
	case 2, 4, 8:
		// ok
	default:
		o.IntSize = Defaults().IntSize
		o.IntSizeExplicit = false
	}
	switch o.PointerSize {
	case 4, 8:
		// ok
	default:
		o.PointerSize = Defaults().PointerSize
	}
	// Partial-expand free-form strings often conflict; bodyparity drop-in leaves it off.
	o.PartialExpand = ""
	return o
}

// OptionsRegistryNames returns all registered field names (for tests).
func OptionsRegistryNames() []string {
	out := make([]string, len(optionsRegistry))
	for i, f := range optionsRegistry {
		out[i] = f.Name
	}
	return out
}

func planeLens(cliOnly bool) (nBool, nInt, nStr int) {
	b, i, s := fieldPlanes(cliOnly)
	return len(b), len(i), len(s)
}

// OptionsFieldCount returns full registry plane sizes (all kinds).
func OptionsFieldCount() (nBool, nInt, nStr int) { return planeLens(false) }

// DropInFieldCount returns drop-in (FieldCLI) plane sizes for bodyparity fuzz.
func DropInFieldCount() (nBool, nInt, nStr int) { return planeLens(true) }

// FormatOptionsShort is a compact log of non-default fields.
func FormatOptionsShort(o Options) string {
	d := Defaults()
	var parts []string
	parts = append(parts, fmt.Sprintf("seed=%d", o.Seed))
	for _, f := range optionsRegistry {
		switch {
		case f.getBool != nil:
			if f.getBool(o) != f.getBool(d) {
				parts = append(parts, fmt.Sprintf("%s=%v", f.Name, f.getBool(o)))
			}
		case f.getInt != nil:
			if f.getInt(o) != f.getInt(d) {
				parts = append(parts, fmt.Sprintf("%s=%d", f.Name, f.getInt(o)))
			}
		case f.getStr != nil:
			if f.getStr(o) != f.getStr(d) && f.getStr(o) != "" {
				parts = append(parts, fmt.Sprintf("%s=%q", f.Name, f.getStr(o)))
			}
		}
	}
	return strings.Join(parts, " ")
}
