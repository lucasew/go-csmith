// Upstream: DefaultProgramGenerator.cpp / OutputMgr.cpp / Function.cpp GenerateFunctions.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// ProgramGenerator mirrors DefaultProgramGenerator state for one generation run.
type ProgramGenerator struct {
	Opts    Options
	Seed    uint64
	Rng     *Rng
	Probs   *Probabilities
	VS      *VariableSelector
	Tables  *ExprTables
	StmtTab *ThresholdTable
	Funcs   FunctionList
	// FactMgrs mirrors Function::FMList (per-function FactMgr).
	FactMgrs *FactMgrMap
	// Argv are CLI args for the Options: header line (excluding argv[0] typically).
	Argv []string
	// Types holds derived pointer types (Type::derived_types).
	Types TypeEnv
}

// NewProgramGenerator constructs a generator (DefaultProgramGenerator ctor + initialize subset).
// DefaultProgramGenerator::initialize — CreateInstance RNG; ExtensionMgr skipped (null).
func NewProgramGenerator(opts Options) *ProgramGenerator {
	seed := opts.Seed
	r := NewRng(seed)
	probs := NewProbabilities(opts)
	vs := NewVariableSelector(opts)
	g := &ProgramGenerator{
		Opts:     opts,
		Seed:     seed,
		Rng:      r,
		Probs:    probs,
		VS:       vs,
		Tables:   NewExprTables(opts),
		StmtTab:  NewStatementThresholdTable(opts),
		FactMgrs: NewFactMgrMap(),
	}
	// Share gensym + derived_types across selector and generator.
	vs.Types = &g.Types
	return g
}

// Initialize mirrors DefaultProgramGenerator::initialize (RNG already seeded).
func (g *ProgramGenerator) Initialize() {
	// Type::GenerateSimpleTypes is satisfied by GetSimpleType cache.
	// ExtensionMgr::CreateExtension — null default, nothing to do.
	// Finalization::doFinalization subset for a fresh generation.
	DoFinalization()
}

// GenerateAllTypes mirrors Type::GenerateAllTypes (random mode).
// Type.cpp:1179–1202 — simples + structs/unions via MoreTypesProbability.
func (g *ProgramGenerator) GenerateAllTypes() {
	GenerateAllTypesEnv(g.Rng, g.Opts, g.Probs, &g.Types)
}

// GenerateFunctions mirrors Function.cpp GenerateFunctions.
// Function.cpp:790–807 — make_first then GenerateBody; FactMgr per function.
func (g *ProgramGenerator) GenerateFunctions() {
	if g == nil {
		return
	}
	if g.FactMgrs == nil {
		g.FactMgrs = NewFactMgrMap()
	}
	g.Funcs.Types = &g.Types
	// Function.cpp:792–793 — initialize_builtin_functions when builtins on
	if g.Opts.Builtins {
		InitializeBuiltinFunctions(g.Opts, g.Probs, g.Rng, &g.Funcs, g.FactMgrs)
	}
	// Function::make_first — creates FactMgr for func_1
	_ = MakeFirst(g.Rng, g.Opts, g.Probs, g.VS, &g.VS.Sym, g.Tables, g.StmtTab, &g.Funcs, g.FactMgrs)
	// Create body of each function until no new unbuilt remain (Function.cpp:801–807).
	for i := 0; i < len(g.Funcs.Funcs); i++ {
		f := g.Funcs.Funcs[i]
		if f != nil && !f.IsBuilt {
			cg := EmptyCGContext().WithFuncList(&g.Funcs)
			cg.CurrentFunc = f
			cg.Types = &g.Types
			if fm := g.FactMgrs.ForFunc(f); fm != nil {
				// seed global pointer facts already known
				for _, gv := range g.VS.GlobalList {
					fm.AddNewVarFact(gv)
				}
				cg = cg.WithFactMgr(fm)
			}
			f.GenerateBody(g.Rng, g.Opts, g.Probs, g.VS, g.Tables, g.StmtTab, cg)
		}
	}
	// Function.cpp:808 — FactPointTo::aggregate_all_pointto_sets
	AggregateAllPointToSets(g.Funcs.Funcs, g.FactMgrs)
}

// OutputHeader mirrors OutputMgr::OutputHeader (non-concise path).
// OutputMgr.cpp:235–311.
func (g *ProgramGenerator) OutputHeader() string {
	var b strings.Builder
	if g.Opts.Concise {
		b.WriteString("// Options: ")
		if len(g.Argv) == 0 {
			b.WriteString(" (none)")
		} else {
			for _, a := range g.Argv {
				b.WriteString(" ")
				b.WriteString(a)
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString("/*\n")
		b.WriteString(" * This is a RANDOMLY GENERATED PROGRAM.\n")
		b.WriteString(" *\n")
		// PACKAGE_STRING / git_version — fair rewrite identifies as go-csmith port
		b.WriteString(" * Generator: go-csmith (fair rewrite)\n")
		b.WriteString(" * Git version: fair-rewrite\n")
		b.WriteString(" * Options:  ")
		if len(g.Argv) == 0 {
			b.WriteString(" (none)")
		} else {
			for _, a := range g.Argv {
				b.WriteString(" ")
				b.WriteString(a)
			}
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf(" * Seed:      %d\n", g.Seed))
		b.WriteString(" */\n\n")
	}
	if !g.Opts.LongLong {
		b.WriteString("\n#define NO_LONGLONG\n\n")
	}
	if g.Opts.EnableFloat {
		b.WriteString("#include <float.h>\n#include <math.h>\n")
	}
	b.WriteString("#include \"csmith.h\"\n\n")
	if !g.Opts.ComputeHash {
		if g.Opts.AllowInt64() {
			b.WriteString("volatile uint64_t csmith_sink_ = 0;\n\n")
		} else {
			b.WriteString("volatile uint32_t csmith_sink_ = 0;\n\n")
		}
	} else {
		b.WriteString("\n")
	}
	b.WriteString("static long __undefined;\n\n")
	if g.Opts.DepthProtect {
		b.WriteString("#define MAX_DEPTH (5)\nint32_t DEPTH = 0;\n\n")
	}
	// OutputMgr.cpp:301–305 — optional wrappers
	if g.Opts.WrapVolatiles {
		b.WriteString("/* To use wrapper functions, compile this program with -DWRAP_VOLATILES=1. */\n")
		b.WriteString("#include \"volatile_runtime.h\"\n\n")
	}
	if g.Opts.AccessOnce {
		// OutputMgr.cpp:64–67 access_once_macro
		b.WriteString("#ifndef ACCESS_ONCE\n")
		b.WriteString("#define ACCESS_ONCE(v) (*(volatile typeof(v) *)&(v))\n")
		b.WriteString("#endif\n\n")
	}
	if g.Opts.StepHashByStmt {
		// OutputMgr::OutputHashFuncDecl / OutputStepHashFuncDecl
		b.WriteString("void csmith_compute_hash(void);\n")
		b.WriteString("void step_hash(int stmt_id);\n\n")
	}
	return b.String()
}

// OutputStructTypes emits struct/union definitions (before globals/functions).
func (g *ProgramGenerator) OutputStructTypes() string {
	if g == nil {
		return ""
	}
	if len(g.Types.StructTypes) == 0 && len(g.Types.UnionTypes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("/* --- STRUCT/UNION TYPES --- */\n\n")
	for _, st := range g.Types.StructTypes {
		if st != nil {
			b.WriteString(st.OutputStructDecl())
			b.WriteString("\n")
		}
	}
	for _, ut := range g.Types.UnionTypes {
		if ut != nil {
			b.WriteString(ut.OutputUnionDecl())
			b.WriteString("\n")
		}
	}
	return b.String()
}

// OutputGlobals emits GlobalList declarations.
func (g *ProgramGenerator) OutputGlobals() string {
	if g == nil || g.VS == nil || len(g.VS.GlobalList) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("/* --- GLOBAL VARIABLES --- */\n\n")
	arrayByName := map[string]*ArrayVariable{}
	for _, av := range g.VS.Arrays {
		if av != nil {
			arrayByName[av.Name] = av
		}
	}
	for _, v := range g.VS.GlobalList {
		if v == nil || v.Type == nil {
			continue
		}
		if av := arrayByName[v.Name]; av != nil {
			if g.Opts.ForceGlobalsStatic {
				b.WriteString("static ")
			}
			b.WriteString(av.OutputDef())
			b.WriteString("\n")
			continue
		}
		// Variable::OutputDef with force_globals_static + prefix_name + optional attrs
		b.WriteString(v.OutputDefFull(g.Opts.ForceGlobalsStatic, g.Opts.PrefixName, g.Opts.VariableAttributes, g.Rng))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// OutputFunctions mirrors OutputForwardDeclarations + OutputFunctions.
func (g *ProgramGenerator) OutputFunctions() string {
	var b strings.Builder
	b.WriteString("\n\n/* --- FORWARD DECLARATIONS --- */\n")
	for _, f := range g.Funcs.Funcs {
		if f == nil || f.IsBuiltin {
			continue
		}
		b.WriteString(f.OutputForwardDeclOpts(g.Opts.ForceGlobalsStatic, g.Rng, g.Opts.FunctionAttributes))
		b.WriteString("\n")
	}
	b.WriteString("\n\n/* --- FUNCTIONS --- */\n")
	for _, f := range g.Funcs.Funcs {
		if f == nil || f.IsBuiltin {
			continue
		}
		b.WriteString(f.OutputOpts(g.Opts.ForceGlobalsStatic, g.Opts.FunctionAttributes, g.Rng))
		b.WriteString("\n")
	}
	return b.String()
}

// HashGlobalVariables mirrors VariableSelector::HashGlobalVariables.
// VariableSelector.cpp:1613–1615 — MapVariableList(GlobalList, HashVariable).
// Uses GetLastCtrlVars for array index names (caller declares via OutputArrayCtrlVars).
func HashGlobalVariables(vs *VariableSelector) string {
	if vs == nil {
		return ""
	}
	ctrl := GetLastCtrlVars()
	var b strings.Builder
	for _, v := range vs.GlobalList {
		if v == nil {
			continue
		}
		b.WriteString(v.hashOutput(ctrl))
	}
	return b.String()
}

// OutputMain mirrors OutputMgr::OutputMain (no extension).
// OutputMgr.cpp:92–153.
func (g *ProgramGenerator) OutputMain() string {
	if g.Opts.NoMain {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n")
	// OutputMgr.cpp:99 — output_comment_line("----------------------------------------")
	b.WriteString(OutputCommentLine("----------------------------------------", g.Opts.Quiet, g.Opts.Concise))
	if g.Opts.AcceptArgc {
		b.WriteString("int main (int argc, char* argv[])\n{\n")
	} else {
		b.WriteString("int main (void)\n{\n")
	}
	// OutputMgr.cpp:104 — OutputArrayInitializers for global arrays (ctrl vars + loop inits)
	b.WriteString(OutputArrayInitializers(g.VS.GlobalList, g.Opts, "    "))
	b.WriteString("    int print_hash_value = 0;\n")
	if g.Opts.AcceptArgc {
		b.WriteString("    if (argc == 2 && strcmp(argv[1], \"1\") == 0) print_hash_value = 1;\n")
	}
	b.WriteString("    platform_main_begin();\n")
	if g.Opts.ComputeHash {
		b.WriteString("    crc32_gentab();\n")
	}
	// ExtensionMgr::OutputFirstFunInvocation — FunctionInvocation::make_random(first)
	// OutputMgr.cpp:127–136 / FunctionInvocation.cpp:128–135
	if len(g.Funcs.Funcs) > 0 && g.Funcs.Funcs[0] != nil {
		f0 := g.Funcs.Funcs[0]
		cg := EmptyCGContext().WithFuncList(&g.Funcs)
		cg.Types = &g.Types
		// build_invocation for target — args via make_random_param (empty if no params)
		inv := BuildUserInvocation(g.Rng, g.Opts, g.Probs, g.VS, g.Tables, cg, &g.Funcs, f0)
		if inv == nil || inv.Failed {
			b.WriteString("    " + f0.Name + "();\n")
		} else {
			b.WriteString("    " + inv.Output() + ";\n")
		}
		// OutputMgr.cpp:136–140 — OutputPtrResets when !dangling_global_ptrs
		if !g.Opts.DanglingGlobalPointers {
			b.WriteString(OutputPtrResets(f0.DeadGlobals, g.Opts))
		}
	}
	if g.Opts.ComputeHash {
		if g.Opts.StepHashByStmt {
			// OutputMgr.cpp:139–140 — call hash function instead of inline
			b.WriteString("    csmith_compute_hash();\n")
		} else {
			// ensure ctrl vars exist when only arrays with brace init (no prior set)
			if GetLastCtrlVars() == nil && GetMaxArrayDimension(g.VS.GlobalList) > 0 {
				ctrl := GetNewCtrlVars(g.Opts)
				b.WriteString(OutputArrayCtrlVars(ctrl, GetMaxArrayDimension(g.VS.GlobalList), "    "))
			}
			// HashGlobalVariables inline
			b.WriteString(HashGlobalVariables(g.VS))
		}
		b.WriteString("    platform_main_end(crc32_context ^ 0xFFFFFFFFUL, print_hash_value);\n")
	} else {
		b.WriteString("    platform_main_end(0,0);\n")
	}
	// ExtensionMgr::OutputTail — return 0 when extension null
	// ExtensionMgr.cpp:109–111
	b.WriteString("    return 0;\n")
	b.WriteString("}\n")
	return b.String()
}

// OutputHashFuncDef mirrors OutputMgr::OutputHashFuncDef.
// OutputMgr.cpp:209–220 — declare ctrl vars then HashGlobalVariables.
func (g *ProgramGenerator) OutputHashFuncDef() string {
	if g == nil || !g.Opts.StepHashByStmt || !g.Opts.ComputeHash {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nvoid csmith_compute_hash(void)\n{\n")
	// OutputMgr.cpp:213–218 — GetMaxArrayDimension + get_new_ctrl_vars + OutputArrayCtrlVars
	dimen := GetMaxArrayDimension(g.VS.GlobalList)
	if dimen > 0 {
		ctrl := GetNewCtrlVars(g.Opts)
		b.WriteString(OutputArrayCtrlVars(ctrl, dimen, "    "))
	}
	b.WriteString(HashGlobalVariables(g.VS))
	b.WriteString("}\n")
	// OutputMgr::OutputStepHashFuncDef — OutputMgr.cpp:170–201
	b.WriteString("\nvoid step_hash(int stmt_id)\n{\n")
	b.WriteString("    int i = 0;\n")
	b.WriteString("    csmith_compute_hash();\n")
	b.WriteString("    printf(\"before stmt(%d): checksum = %X\\n\", stmt_id, crc32_context ^ 0xFFFFFFFFUL);\n")
	b.WriteString("    crc32_context = 0xFFFFFFFFUL;\n")
	b.WriteString("    for (i = 0; i < 256; i++) {\n")
	b.WriteString("        crc32_tab[i] = 0;\n")
	b.WriteString("    }\n")
	b.WriteString("    crc32_gentab();\n")
	b.WriteString("}\n")
	return b.String()
}

// OutputPtrResets mirrors OutputMgr::OutputPtrResets.
// OutputMgr.cpp:326–340 — scalar = 0; arrays use get_last_ctrl_vars + output_init zero.
func OutputPtrResets(ptrs []*Variable, opts Options) string {
	if len(ptrs) == 0 {
		return ""
	}
	ctrl := GetLastCtrlVars()
	if ctrl == nil {
		// generation-time fallback when no prior OutputArrayInitializers
		ctrl = GetNewCtrlVars(opts)
	}
	names := CtrlVarNames(ctrl)
	var b strings.Builder
	for _, v := range ptrs {
		if v == nil {
			continue
		}
		sizes := v.ArraySizes
		av := v.AsArray
		if av != nil && len(av.Sizes) > 0 {
			sizes = av.Sizes
		}
		if v.IsArray && len(sizes) > 0 {
			// ArrayVariable::output_init with Constant zero (always loop form)
			if av == nil {
				av = &ArrayVariable{
					Variable: *v,
					Sizes:    sizes,
				}
			}
			// force loop even if NoLoopInitializer (globals)
			saved := av.Init
			av.Init = MakeInt(0)
			// temporary: override NoLoopInitializer by calling force form
			b.WriteString(outputArrayInitForced(av, "    ", names))
			av.Init = saved
			continue
		}
		b.WriteString("    " + v.OutputC() + " = 0;\n")
	}
	return b.String()
}

// outputArrayInitForced is OutputInit without NoLoopInitializer early-out.
// Used by OutputPtrResets (upstream always loops for array dead_globals).
func outputArrayInitForced(av *ArrayVariable, indent string, ctrl []string) string {
	if av == nil {
		return ""
	}
	initVal := "0"
	if av.Init != nil && av.Init.Value != "" {
		initVal = av.Init.Value
	}
	var b strings.Builder
	pad := indent
	for i, sz := range av.Sizes {
		iv := string([]byte{byte('i' + i)})
		if i < len(ctrl) && ctrl[i] != "" {
			iv = ctrl[i]
		}
		b.WriteString(pad + "for (" + iv + " = 0; " + iv + " < " + itoa(sz) + "; " + iv + "++)\n")
		if i+1 < len(av.Sizes) {
			b.WriteString(pad + "{\n")
			pad += "    "
		}
	}
	access := av.GetActualName(false)
	for i := range av.Sizes {
		iv := string([]byte{byte('i' + i)})
		if i < len(ctrl) && ctrl[i] != "" {
			iv = ctrl[i]
		}
		access += "[" + iv + "]"
	}
	b.WriteString(pad + "    " + access + " = " + initVal + ";\n")
	for i := len(av.Sizes) - 1; i >= 1; i-- {
		pad = pad[:len(pad)-4]
		b.WriteString(pad + "}\n")
	}
	return b.String()
}

// GoGenerator mirrors DefaultProgramGenerator::goGenerator / DefaultOutputMgr::Output.
// DefaultProgramGenerator.cpp:67–72; DefaultOutputMgr.cpp:175–195.
func (g *ProgramGenerator) GoGenerator() string {
	g.Initialize()
	var b strings.Builder
	b.WriteString(g.OutputHeader())
	g.GenerateAllTypes()
	b.WriteString(g.OutputStructTypes())
	g.GenerateFunctions()
	b.WriteString(g.OutputGlobals())
	b.WriteString(g.OutputFunctions())
	b.WriteString(g.OutputHashFuncDef())
	b.WriteString(g.OutputMain())
	// DefaultOutputMgr.cpp:194 — OutputTail after main (statistics comment)
	b.WriteString(OutputTail(g.Funcs.Funcs, g.Opts))
	return b.String()
}
