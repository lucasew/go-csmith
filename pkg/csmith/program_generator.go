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
	// CGOptions process-wide state for Constant::make_random / choose_var / emit.
	SetProcessOptions(opts)
	seed := opts.Seed
	r := NewRng(seed)
	// C++ DefaultRndNumGenerator process instance — CreateVariable / create_field_vars
	// burn the same stream (no invent separate NewRng per CreateVariable).
	SetProcessRng(r)
	// C++ Probabilities is a process singleton — one session table for generator + VS
	// + CreateVariable/create_field_vars (no invent second NewProbabilities(opts))
	probs := NewProbabilities(opts)
	SetProcessProbabilities(probs)
	// Probabilities.cpp:573–578 — assignOpsTable_ + expr/param tables once
	InitSessionProbabilityTables(opts)
	// share session Probabilities with VS (no invent throwaway NewProbabilities then overwrite)
	vs := NewVariableSelectorProbs(opts, probs)
	stmtTab := NewStatementThresholdTable(opts)
	// C++ Statement static ProbabilityTable — one session table for Block::make_random
	// and nested GenerateBody via FunctionInvocation (no invent second table mid-call)
	SetProcessStmtTab(stmtTab)
	// VariableSelector::InitScopeTable — scopeTable_ once per generation
	InitScopeTable(opts)
	// Expression session tables (same instance as ProcessExprTables)
	exprTables := ProcessExprTables()
	if exprTables == nil {
		exprTables = NewExprTables(opts)
		SetProcessExprTables(exprTables)
	}
	g := &ProgramGenerator{
		Opts:     opts,
		Seed:     seed,
		Rng:      r,
		Probs:    probs,
		VS:       vs,
		Tables:   exprTables,
		StmtTab:  stmtTab,
		FactMgrs: NewFactMgrMap(),
	}
	// Share gensym + derived_types across selector and generator.
	vs.Types = &g.Types
	// Attribute generators for this generation (Initialize*Attributes)
	InitAttrGenerators(opts, probs)
	return g
}

// Initialize mirrors DefaultProgramGenerator::initialize (RNG already seeded).
func (g *ProgramGenerator) Initialize() {
	// Type::GenerateSimpleTypes is satisfied by GetSimpleType cache.
	// ExtensionMgr::CreateExtension — null default, nothing to do.
	// Finalization::doFinalization subset for a fresh generation.
	DoFinalization()
	// DoFinalization may clear process-wide session handles; re-install the
	// generator's live singletons (CGOptions / RNG / Probabilities / Statement table).
	// C++ statics survive between Finalization and the next draws of this run.
	if g != nil {
		SetProcessOptions(g.Opts)
		SetProcessRng(g.Rng)
		SetProcessProbabilities(g.Probs)
		SetProcessStmtTab(g.StmtTab)
		SetProcessExprTables(g.Tables)
		// re-init scope + assign ops from session opts (once-per-run tables)
		InitScopeTable(g.Opts)
		// assignOpsTable_ depends on CGOptions incr/compound flags
		SetProcessAssignOpsTable(NewAssignOpsTable(g.Opts))
	}
}

// GenerateAllTypes mirrors Type::GenerateAllTypes (random mode).
// Type.cpp:1179–1202 — simples + structs/unions via MoreTypesProbability.
func (g *ProgramGenerator) GenerateAllTypes() {
	GenerateAllTypesEnv(g.Rng, g.Opts, g.Probs, &g.Types)
}

// GenerateFunctions mirrors Function.cpp GenerateFunctions.
// Function.cpp:790–809 — interested facts; builtins; make_first; generate unbuilt;
// aggregate_all_pointto_sets.
func (g *ProgramGenerator) GenerateFunctions() {
	if g == nil {
		return
	}
	// Function::FMList is session state from NewProgramGenerator; no invent mid-run
	if g.FactMgrs == nil {
		return
	}
	g.Funcs.Types = &g.Types
	// Function.cpp:791 — FactMgr::add_interested_facts(CGOptions::interested_facts())
	interests := g.Opts.InterestedFacts
	if interests == 0 {
		interests = DefaultInterestedFacts
	}
	AddInterestedFacts(interests)
	// Function.cpp:792–793 — initialize_builtin_functions when builtins on
	if g.Opts.Builtins {
		InitializeBuiltinFunctions(g.Opts, g.Probs, g.Rng, &g.Funcs, g.FactMgrs)
	}
	// Function.cpp:796–797 — make_first; ERROR_RETURN
	first := MakeFirst(g.Rng, g.Opts, g.Probs, g.VS, &g.VS.Sym, g.Tables, g.StmtTab, &g.Funcs, g.FactMgrs)
	if first == nil || HasError() {
		// sticky error / failed first function — stop generation (no soft invent continue)
		return
	}
	// Function.cpp:801–807 — create body of each unbuilt function (list may grow)
	for i := 0; i < len(g.Funcs.Funcs); i++ {
		f := g.Funcs.Funcs[i]
		if f == nil || f.IsBuilt || f.BuildState == BuildBuilt {
			continue
		}
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
		// Function.cpp:805 ERROR_RETURN after each GenerateBody
		if HasError() {
			return
		}
	}
	// Function.cpp:808 — FactPointTo::aggregate_all_pointto_sets
	AggregateAllPointToSets(g.Funcs.Funcs, g.FactMgrs)
	// Function.cpp:809 — ExtensionMgr::GenerateValues (null extension → no-op)
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
	var structAttr, unionAttr *AttributeGenerator
	if g.Opts.TypeAttributes {
		structAttr = EnsureStructTypeAttrGenerator()
		unionAttr = EnsureUnionTypeAttrGenerator()
	}
	for _, st := range g.Types.StructTypes {
		if st != nil {
			if structAttr != nil {
				b.WriteString(st.OutputStructDeclOpts(g.Rng, structAttr))
			} else {
				b.WriteString(st.OutputStructDecl())
			}
			b.WriteString("\n")
		}
	}
	for _, ut := range g.Types.UnionTypes {
		if ut != nil {
			if unionAttr != nil {
				b.WriteString(ut.OutputUnionDeclOpts(g.Rng, unionAttr))
			} else {
				b.WriteString(ut.OutputUnionDecl())
			}
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
	// C++ GlobalList may hold collective + itemized member; emit array def once
	emittedArray := map[string]bool{}
	for _, v := range g.VS.GlobalList {
		if v == nil || v.Type == nil {
			continue
		}
		if av := arrayByName[v.Name]; av != nil {
			if emittedArray[v.Name] {
				continue
			}
			emittedArray[v.Name] = true
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
// Function.cpp:812–830 — optional FORWARD ALIAS DECLARATIONS when func_attr_flag.
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
	// Function.cpp:820–826 — alias decls when FunctionAttributes
	if g.Opts.FunctionAttributes {
		b.WriteString("\n\n/* --- FORWARD ALIAS DECLARATIONS --- */\n")
		for _, f := range g.Funcs.Funcs {
			if f == nil || f.IsBuiltin {
				continue
			}
			b.WriteString(f.OutputForwardDeclAlias(g.Opts.ForceGlobalsStatic))
			b.WriteString("\n")
		}
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
	return HashGlobalVariablesWithUnionFacts(vs, nil)
}

// HashGlobalVariablesWithUnionFacts hashes globals with FactUnion field filtering.
// Variable::hash uses FactUnion::is_field_readable when FactMgr global_facts present.
func HashGlobalVariablesWithUnionFacts(vs *VariableSelector, unionFacts []*FactUnion) string {
	if vs == nil {
		return ""
	}
	ctrl := GetLastCtrlVars()
	var b strings.Builder
	for _, v := range vs.GlobalList {
		if v == nil {
			continue
		}
		b.WriteString(v.hashOutput(ctrl, unionFacts))
	}
	return b.String()
}

// hashGlobals is HashGlobalVariables using first function's UnionFacts when available.
func (g *ProgramGenerator) hashGlobals() string {
	if g == nil {
		return ""
	}
	var uf []*FactUnion
	if g.FactMgrs != nil && len(g.Funcs.Funcs) > 0 && g.Funcs.Funcs[0] != nil {
		if fm := g.FactMgrs.ForFunc(g.Funcs.Funcs[0]); fm != nil {
			uf = fm.UnionFacts
		}
	}
	return HashGlobalVariablesWithUnionFacts(g.VS, uf)
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

	// first-function invocation builder (shared by blind_check and normal path)
	var firstInv string
	var f0 *Function
	if len(g.Funcs.Funcs) > 0 && g.Funcs.Funcs[0] != nil {
		f0 = g.Funcs.Funcs[0]
		cg := EmptyCGContext().WithFuncList(&g.Funcs)
		cg.Types = &g.Types
		// OutputMgr.cpp:97 — ExtensionMgr::MakeFuncInvocation always live invoke
		// no soft invent name()+"()" when build fails
		inv := BuildUserInvocation(g.Rng, g.Opts, g.Probs, g.VS, g.Tables, &cg, &g.Funcs, f0)
		if inv != nil && !inv.Failed {
			firstInv = inv.Output()
		}
	}

	// OutputMgr.cpp:106–111 — blind_check_global: invoke + output_value_dump per global
	if g.Opts.BlindCheckGlobal {
		if firstInv != "" {
			b.WriteString("    " + firstInv + ";\n")
		}
		// program end facts for union readability (FactMgr::get_program_end_facts)
		var endUnion []*FactUnion
		if f0 != nil && g.FactMgrs != nil {
			if fm := g.FactMgrs.ForFunc(f0); fm != nil {
				endUnion = fm.UnionFacts
			}
		}
		for _, v := range g.VS.GlobalList {
			b.WriteString(v.OutputValueDump("checksum ", 1, endUnion))
		}
		b.WriteString("    return 0;\n")
		b.WriteString("}\n")
		return b.String()
	}

	b.WriteString("    int print_hash_value = 0;\n")
	if g.Opts.AcceptArgc {
		b.WriteString("    if (argc == 2 && strcmp(argv[1], \"1\") == 0) print_hash_value = 1;\n")
	}
	b.WriteString("    platform_main_begin();\n")
	if g.Opts.ComputeHash {
		b.WriteString("    crc32_gentab();\n")
	}
	// ExtensionMgr::OutputFirstFunInvocation
	// OutputMgr.cpp:127–136
	if firstInv != "" {
		b.WriteString("    " + firstInv + ";\n")
		// OutputMgr.cpp:136–140 — OutputPtrResets when !dangling_global_ptrs
		if f0 != nil && !g.Opts.DanglingGlobalPointers {
			b.WriteString(OutputPtrResets(f0.DeadGlobals, g.Opts))
		}
	}
	// OutputMgr.cpp:141–145 — step_hash path invokes csmith_compute_hash; else inline HashGlobalVariables
	// (guarded by compute_hash so crc_* are defined)
	if g.Opts.ComputeHash {
		if g.Opts.StepHashByStmt {
			b.WriteString("    csmith_compute_hash();\n")
		} else {
			// OutputMgr.cpp:142 — HashGlobalVariables after OutputArrayInitializers
			// (ctrl vars from that path via get_last_ctrl_vars; no soft GetNew here)
			b.WriteString(g.hashGlobals())
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
	b.WriteString(g.hashGlobals())
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
// OutputMgr.cpp:326–340 — scalar = 0; arrays use get_last_ctrl_vars + output_init(&zero).
func OutputPtrResets(ptrs []*Variable, opts Options) string {
	if len(ptrs) == 0 {
		return ""
	}
	// OutputMgr.cpp:332 — Variable::get_last_ctrl_vars() (no GetNew soft-fallback)
	ctrl := GetLastCtrlVars()
	names := CtrlVarNames(ctrl)
	// OutputMgr.cpp:331 — Constant zero(get_int_type(), "0"); always live Expression*
	zero := MakeInt(0)
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
			if len(names) == 0 {
				// C++ assumes get_last_ctrl_vars() non-empty after OutputArrayInitializers
				continue
			}
			// ArrayVariable::output_init(out, &zero, ctrl_vars, 1) — always loop form
			if av == nil {
				av = &ArrayVariable{
					Variable: *v,
					Sizes:    sizes,
				}
			}
			// force loop even if NoLoopInitializer (globals); pass live zero, not invent
			savedInit, savedExpr := av.Init, av.InitExpr
			av.Init = zero
			av.InitExpr = nil
			b.WriteString(outputArrayInitForced(av, "    ", names, opts.PostIncrOperator))
			av.Init, av.InitExpr = savedInit, savedExpr
			continue
		}
		b.WriteString("    " + v.OutputC() + " = 0;\n")
	}
	return b.String()
}

// outputArrayInitForced is OutputInit without NoLoopInitializer early-out.
// Used by OutputPtrResets (upstream always loops for array dead_globals).
// ArrayVariable.cpp:619–655 — init->Output only; no invent "0" when init missing.
func outputArrayInitForced(av *ArrayVariable, indent string, ctrl []string, postIncr bool) string {
	if av == nil {
		return ""
	}
	// ArrayVariable.cpp:622–623 — collective itemized members skip
	if av.Collective != nil {
		return ""
	}
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
	// ArrayVariable::output_with_indices
	access := av.OutputWithIndices(ctrl)
	b.WriteString(pad + "    " + access + " = " + initVal + ";\n")
	for i := len(av.Sizes) - 1; i >= 1; i-- {
		pad = pad[:len(pad)-4]
		b.WriteString(pad + "}\n")
	}
	return b.String()
}

// GoGenerator mirrors DefaultProgramGenerator::goGenerator / DefaultOutputMgr::Output.
// DefaultProgramGenerator.cpp:67–80; DefaultOutputMgr.cpp:175–195.
// Returns empty string when sticky ERROR_RETURN aborts generation (no soft invent
// of partial program as success).
func (g *ProgramGenerator) GoGenerator() string {
	g.Initialize()
	var b strings.Builder
	b.WriteString(g.OutputHeader())
	g.GenerateAllTypes()
	if HasError() {
		return ""
	}
	b.WriteString(g.OutputStructTypes())
	g.GenerateFunctions()
	// Function.cpp:797/805 ERROR_RETURN — stop output when generation failed
	if HasError() {
		return ""
	}
	b.WriteString(g.OutputGlobals())
	b.WriteString(g.OutputFunctions())
	b.WriteString(g.OutputHashFuncDef())
	b.WriteString(g.OutputMain())
	// DefaultOutputMgr.cpp:194 — OutputTail after main (statistics comment)
	b.WriteString(OutputTail(g.Funcs.Funcs, g.Opts))
	// DefaultProgramGenerator.cpp:73–77 — identify_wrappers writes wrapper.h
	// Library-first: append N_WRAP definition as a trailing section for consumers.
	if g.Opts.IdentifyWrappers {
		b.WriteString("\n/* --- wrapper.h (identify_wrappers) ---\n")
		b.WriteString(OutputWrapperH())
		b.WriteString("--- end wrapper.h --- */\n")
	}
	return b.String()
}

// WrapperHeader returns wrapper.h content when identify_wrappers is set.
// DefaultProgramGenerator.cpp:73–77.
func (g *ProgramGenerator) WrapperHeader() string {
	if g == nil || !g.Opts.IdentifyWrappers {
		return ""
	}
	return OutputWrapperH()
}
