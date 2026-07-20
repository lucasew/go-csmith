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
	// CGOptions::monitored_funcs → OutputMgr::monitored_funcs_
	opts.ApplyMonitoredFuncs()
	seed := opts.Seed
	// RandomNumber::CreateInstance(rDefaultRndNumGenerator, seed)
	// RandomNumber.cpp:63–74 — process singleton + DefaultRndNumGenerator.
	CreateRandomNumberInstance(RngKindDefault, seed)
	r := ProcessRng()
	if r == nil {
		// Create failed (should not on Default); fail closed sticky already set.
		r = NewRng(seed)
		SetProcessRng(r)
	}
	// C++ Probabilities is a process singleton — one session table for generator + VS
	// + CreateVariable/create_field_vars (no invent second NewProbabilities(opts))
	probs := NewProbabilities(opts)
	SetProcessProbabilities(probs)
	// Probabilities.cpp:573–578 — assignOpsTable_ + expr/param tables once
	InitSessionProbabilityTables(opts)
	// share session Probabilities with VS (no invent throwaway NewProbabilities then overwrite)
	vs := NewVariableSelectorProbs(opts, probs)
	// Statement.cpp:133–139 — InitProbabilityTable from pStatementProb (session probs)
	// no invent second NewStatementThresholdTable independent of Probabilities
	stmtTab := probs.StatementThresholdTable()
	SetProcessStmtTab(stmtTab)
	// VariableSelector::InitScopeTable — scopeTable_ once per generation
	InitScopeTable(opts)
	// Expression session tables from InitSessionProbabilityTables (no invent mid-ctor)
	exprTables := ProcessExprTables()
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
// C++ initialize only CreateInstance+OutputMgr; Go also runs Finalization subset
// so library multi-Generate starts from a clean process pool (dtor-like).
func (g *ProgramGenerator) Initialize() {
	// Type::GenerateSimpleTypes is satisfied by GetSimpleType cache.
	// ExtensionMgr::CreateExtension — null default, nothing to do.
	// Finalization::doFinalization subset for a fresh generation
	// (includes RandomNumber::doFinalization).
	DoFinalization()
	// DoFinalization clears process-wide session handles; re-install the
	// generator's live singletons (CGOptions / RNG / Probabilities / Statement table).
	if g != nil {
		SetProcessOptions(g.Opts)
		// RandomNumber::CreateInstance after Finalization (DefaultProgramGenerator.cpp:55)
		CreateRandomNumberInstance(RngKindDefault, g.Seed)
		if r := ProcessRng(); r != nil {
			g.Rng = r
		} else {
			SetProcessRng(g.Rng)
		}
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
	// ProgramGenerator always live for GenerateFunctions; sticky incomplete no invent no-op
	if g == nil {
		SetError(ErrGeneric)
		return
	}
	// Function::FMList is session state from NewProgramGenerator; sticky no invent mid-run miss
	if g.FactMgrs == nil {
		SetError(ErrGeneric)
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
		// Function* always live on Funcs; nil hole fails closed sticky (no invent skip mid generation)
		if f == nil {
			SetError(ErrGeneric)
			return
		}
		if f.IsBuilt || f.BuildState == BuildBuilt {
			continue
		}
		cg := EmptyCGContext().WithFuncList(&g.Funcs)
		cg.CurrentFunc = f
		cg.Types = &g.Types
		if fm := g.FactMgrs.ForFunc(f); fm != nil {
			// seed global pointer facts already known
			// Variable* always live on GlobalList; nil hole / incomplete AddNewVarFact
			// fails closed sticky (no invent soft-continue GenerateBody past holes)
			if g.VS != nil {
				if !VariablesComplete(g.VS.GlobalList) {
					SetError(ErrGeneric)
					return
				}
				for _, gv := range g.VS.GlobalList {
					fm.AddNewVarFact(gv)
					// incomplete PT/union abstract sticky or wipe must abort (no invent body past holes)
					if HasError() || !FactsComplete(fm.GlobalFacts) {
						if !HasError() {
							SetError(ErrGeneric)
						}
						return
					}
				}
			}
			cg = cg.WithFactMgr(fm)
		}
		f.GenerateBody(g.Rng, g.Opts, g.Probs, g.VS, g.Tables, g.StmtTab, cg)
		// Function.cpp:805 ERROR_RETURN after each GenerateBody
		if HasError() {
			return
		}
		// no invent continue past unbuilt/null-body (C++ would crash on body->)
		if f.Body == nil || f.BuildState != BuildBuilt {
			return
		}
	}
	// Function.cpp:808 — FactPointTo::aggregate_all_pointto_sets
	AggregateAllPointToSets(g.Funcs.Funcs, g.FactMgrs)
	// Function.cpp:809 — ExtensionMgr::GenerateValues (null extension → no-op)
}

// OutputHeader mirrors OutputMgr::OutputHeader (non-concise path).
// OutputMgr.cpp:235–311.
// Incomplete ProgramGenerator sticky empty (no invent header soft-skip past hole).
func (g *ProgramGenerator) OutputHeader() string {
	// ProgramGenerator always live at emit; sticky incomplete no invent empty header
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
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
	// OutputMgr.cpp:308–311 — decls when step_hash_by_stmt
	// Go: only with ComputeHash so decls match OutputHashFuncDef / main call
	// (no invent forward decls without live defs)
	if g.hashHelpersEnabled() {
		// OutputMgr::OutputHashFuncDecl / OutputStepHashFuncDecl
		b.WriteString(OutputHashFuncDecl())
		b.WriteString(OutputStepHashFuncDecl())
	}
	return b.String()
}

// hashHelpersEnabled is step_hash_by_stmt && compute_hash.
// OutputHashFuncDef / header decls / main invocation share this gate so we never
// invent a call or forward decl without a matching definition body.
func (g *ProgramGenerator) hashHelpersEnabled() bool {
	return g != nil && g.Opts.StepHashByStmt && g.Opts.ComputeHash
}

// hashFuncDefReady is true when OutputHashFuncDef can emit a live body.
// Variable.cpp:802 assert(dimen <= ctrl_vars.size()) — fail closed when MaxArrayDim
// cannot cover global array rank (no invent empty ctrl "int ;" shell inside hash).
func (g *ProgramGenerator) hashFuncDefReady() bool {
	if !g.hashHelpersEnabled() {
		return false
	}
	// incomplete GlobalList sticky not-ready (GetMaxArrayDimension -1 must not
	// invent ready via dimen<=0 / soft re-pick hash-func shell past holes)
	if g.VS == nil || !VariablesComplete(g.VS.GlobalList) {
		SetError(ErrGeneric)
		return false
	}
	dimen := GetMaxArrayDimension(g.VS.GlobalList)
	if dimen < 0 {
		// incomplete array sizes sticky
		if !HasError() {
			SetError(ErrGeneric)
		}
		return false
	}
	if dimen == 0 {
		return true
	}
	return g.Opts.MaxArrayDim >= dimen
}

// OutputStructTypes emits struct/union definitions (before globals/functions).
// Incomplete StructTypes/UnionTypes fails closed sticky (no invent empty-section past holes).
func (g *ProgramGenerator) OutputStructTypes() string {
	// ProgramGenerator always live at emit; sticky incomplete no invent empty types section
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
	if len(g.Types.StructTypes) == 0 && len(g.Types.UnionTypes) == 0 {
		return ""
	}
	// incomplete type pools fail closed sticky (no invent partial section / empty shell)
	if !typesComplete(g.Types.StructTypes) || !typesComplete(g.Types.UnionTypes) {
		SetError(ErrGeneric)
		return ""
	}
	var structAttr, unionAttr *AttributeGenerator
	if g.Opts.TypeAttributes {
		structAttr = EnsureStructTypeAttrGenerator()
		unionAttr = EnsureUnionTypeAttrGenerator()
	}
	var body strings.Builder
	for _, st := range g.Types.StructTypes {
		// pre-validated typesComplete
		var decl string
		if structAttr != nil {
			decl = st.OutputStructDeclOpts(g.Rng, structAttr)
		} else {
			decl = st.OutputStructDecl()
		}
		// residual ERROR sticky — no invent soft-continue later structs past decl residual
		if HasError() {
			return ""
		}
		if decl == "" {
			// incomplete struct decl IR sticky — no invent types-section header only
			SetError(ErrGeneric)
			return ""
		}
		body.WriteString(decl)
		body.WriteString("\n")
	}
	for _, ut := range g.Types.UnionTypes {
		// pre-validated typesComplete
		var decl string
		if unionAttr != nil {
			decl = ut.OutputUnionDeclOpts(g.Rng, unionAttr)
		} else {
			decl = ut.OutputUnionDecl()
		}
		// residual ERROR sticky — no invent soft-continue later unions past decl residual
		if HasError() {
			return ""
		}
		if decl == "" {
			// incomplete union decl IR sticky — no invent types-section header only
			SetError(ErrGeneric)
			return ""
		}
		body.WriteString(decl)
		body.WriteString("\n")
	}
	// no invent section header without live type decls
	if body.Len() == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("/* --- STRUCT/UNION TYPES --- */\n\n")
	b.WriteString(body.String())
	return b.String()
}

// OutputGlobals emits GlobalList declarations.
// Incomplete GlobalList / Arrays fails closed sticky (no invent empty-section success
// past nil holes via soft return "").
func (g *ProgramGenerator) OutputGlobals() string {
	// ProgramGenerator always live at emit; sticky incomplete no invent empty globals section
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
	// complete empty globals (no VS or empty list) soft empty section
	if g.VS == nil || len(g.VS.GlobalList) == 0 {
		return ""
	}
	// incomplete GlobalList fails closed sticky (no invent partial section / empty shell)
	if !VariablesComplete(g.VS.GlobalList) {
		SetError(ErrGeneric)
		return ""
	}
	arrayByName := map[string]*ArrayVariable{}
	// ArrayVariable* always live on Arrays; nil hole fails closed sticky
	for _, av := range g.VS.Arrays {
		if av == nil {
			SetError(ErrGeneric)
			return ""
		}
		arrayByName[av.Name] = av
	}
	// C++ GlobalList may hold collective + itemized member; emit array def once
	emittedArray := map[string]bool{}
	var body strings.Builder
	for _, v := range g.VS.GlobalList {
		// pre-validated VariablesComplete; Type always live for OutputDef
		if v.Type == nil {
			SetError(ErrGeneric)
			return ""
		}
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent scalar OutputDef for IsArray shell not in Arrays map)
		if v.IsArray && v.AsArray == nil {
			SetError(ErrGeneric)
			return ""
		}
		if av := arrayByName[v.Name]; av != nil {
			if emittedArray[v.Name] {
				// dual-count collective+itemized — intentional C++ list shape, skip re-emit
				continue
			}
			emittedArray[v.Name] = true
			// ArrayVariable::OutputDef always live; sticky no invent "static \n" for empty
			def := av.OutputDef()
			// residual ERROR sticky — no invent soft-continue later globals past OutputDef residual
			if HasError() {
				return ""
			}
			if def == "" {
				SetError(ErrGeneric)
				return ""
			}
			if g.Opts.ForceGlobalsStatic {
				body.WriteString("static ")
			}
			body.WriteString(def)
			body.WriteString("\n")
			continue
		}
		// IsArray with AsArray but missing from Arrays map still must use array OutputDef
		// (no invent scalar OutputDefFull for live array not registered on VS.Arrays)
		if v.IsArray && v.AsArray != nil {
			def := v.AsArray.OutputDef()
			// residual ERROR sticky — no invent soft-continue later globals past OutputDef residual
			if HasError() {
				return ""
			}
			if def == "" {
				SetError(ErrGeneric)
				return ""
			}
			if g.Opts.ForceGlobalsStatic {
				body.WriteString("static ")
			}
			body.WriteString(def)
			body.WriteString("\n")
			continue
		}
		// Variable::OutputDef with force_globals_static + prefix_name + optional attrs
		// sticky no invent blank global line for incomplete IR
		def := v.OutputDefFull(g.Opts.ForceGlobalsStatic, g.Opts.PrefixName, g.Opts.VariableAttributes, g.Rng)
		// residual ERROR sticky — no invent soft-continue later globals past OutputDefFull residual
		if HasError() {
			return ""
		}
		if def == "" {
			SetError(ErrGeneric)
			return ""
		}
		body.WriteString(def)
		body.WriteString("\n")
	}
	// no invent section header without any live global defs
	if body.Len() == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("/* --- GLOBAL VARIABLES --- */\n\n")
	b.WriteString(body.String())
	b.WriteString("\n")
	return b.String()
}

// OutputFunctions mirrors OutputForwardDeclarations + OutputFunctions.
// Function.cpp:812–830 — optional FORWARD ALIAS DECLARATIONS when func_attr_flag.
// Incomplete Funcs fails closed sticky (no invent empty-section / soft-skip hole).
func (g *ProgramGenerator) OutputFunctions() string {
	// ProgramGenerator always live at emit; sticky incomplete no invent empty functions section
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
	// incomplete Funcs list fails closed sticky (no invent partial section past hole)
	if !FunctionsComplete(g.Funcs.Funcs) {
		SetError(ErrGeneric)
		return ""
	}
	var forwards, aliases, bodies strings.Builder
	for _, f := range g.Funcs.Funcs {
		// pre-validated FunctionsComplete
		if f.IsBuiltin {
			continue
		}
		// incomplete header IR — fail closed whole functions section
		d := f.OutputForwardDeclOpts(g.Opts.ForceGlobalsStatic, g.Rng, g.Opts.FunctionAttributes)
		// residual ERROR sticky — no invent soft-continue later funcs past forward residual
		if HasError() {
			return ""
		}
		if d == "" {
			// incomplete header IR sticky — no invent partial functions section
			SetError(ErrGeneric)
			return ""
		}
		forwards.WriteString(d)
		forwards.WriteString("\n")
		// Function.cpp:820–826 — alias decls when FunctionAttributes
		if g.Opts.FunctionAttributes {
			a := f.OutputForwardDeclAlias(g.Opts.ForceGlobalsStatic)
			// residual ERROR sticky — no invent soft-continue later funcs past alias residual
			if HasError() {
				return ""
			}
			if a == "" {
				// alias expected when FunctionAttributes; incomplete AliasName sticky
				SetError(ErrGeneric)
				return ""
			}
			aliases.WriteString(a)
			aliases.WriteString("\n")
		}
		body := f.OutputOpts(g.Opts.ForceGlobalsStatic, g.Opts.FunctionAttributes, g.Rng)
		// residual ERROR sticky — no invent soft-continue later funcs past body residual
		if HasError() {
			return ""
		}
		if body == "" {
			// incomplete body IR sticky — no invent forward-only section
			SetError(ErrGeneric)
			return ""
		}
		bodies.WriteString(body)
		bodies.WriteString("\n")
	}
	// no invent section headers without live content
	if forwards.Len() == 0 && aliases.Len() == 0 && bodies.Len() == 0 {
		return ""
	}
	var b strings.Builder
	if forwards.Len() > 0 {
		b.WriteString("\n\n/* --- FORWARD DECLARATIONS --- */\n")
		b.WriteString(forwards.String())
	}
	if aliases.Len() > 0 {
		b.WriteString("\n\n/* --- FORWARD ALIAS DECLARATIONS --- */\n")
		b.WriteString(aliases.String())
	}
	if bodies.Len() > 0 {
		b.WriteString("\n\n/* --- FUNCTIONS --- */\n")
		b.WriteString(bodies.String())
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
// Incomplete GlobalList / UnionFacts fails closed sticky (no invent empty-hash success
// or skip-all-fields via IsFieldReadable soft false past holes).
func HashGlobalVariablesWithUnionFacts(vs *VariableSelector, unionFacts []*FactUnion) string {
	// VariableSelector always live for global hash; sticky no invent empty hash without it
	if vs == nil {
		SetError(ErrGeneric)
		return ""
	}
	// incomplete GlobalList fails closed sticky (no invent empty hash past nil hole)
	if !VariablesComplete(vs.GlobalList) {
		SetError(ErrGeneric)
		return ""
	}
	// incomplete union map fails closed sticky (no invent all-fields-unreadable past hole)
	if unionFacts != nil && !UnionFactsComplete(unionFacts) {
		SetError(ErrGeneric)
		return ""
	}
	ctrl := GetLastCtrlVars()
	var b strings.Builder
	for _, v := range vs.GlobalList {
		// pre-validated VariablesComplete
		// empty hash is legitimate for ePointer / unreadable union fields (Variable.cpp)
		part := v.hashOutput(ctrl, unionFacts)
		// residual ERROR sticky — no invent soft-continue later globals past hash residual hole
		if HasError() {
			return ""
		}
		b.WriteString(part)
	}
	return b.String()
}

// hashGlobals is HashGlobalVariables using first function's UnionFacts when available.
func (g *ProgramGenerator) hashGlobals() string {
	// ProgramGenerator always live at hash emit; sticky incomplete no invent empty hash
	if g == nil {
		SetError(ErrGeneric)
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
	// --nomain: soft empty (optionally omit main)
	if g != nil && g.Opts.NoMain {
		return ""
	}
	// ProgramGenerator + VS always live for main emit; sticky no invent main shell without them
	if g == nil || g.VS == nil {
		SetError(ErrGeneric)
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
	// residual ERROR sticky — no invent main body past array-init residual hole
	if HasError() {
		return ""
	}

	// first-function invocation builder (shared by blind_check and normal path)
	// OutputMgr.cpp:97 — ExtensionMgr::MakeFuncInvocation always live when Funcs non-empty
	var firstInv string
	var f0 *Function
	if len(g.Funcs.Funcs) > 0 {
		// Function* always live; sticky no invent main without first call shell
		if g.Funcs.Funcs[0] == nil {
			SetError(ErrGeneric)
			return ""
		}
		f0 = g.Funcs.Funcs[0]
		// skip builtin first (unlikely); still need live invoke for user first
		if !f0.IsBuiltin {
			cg := EmptyCGContext().WithFuncList(&g.Funcs)
			cg.Types = &g.Types
			inv := BuildUserInvocation(g.Rng, g.Opts, g.Probs, g.VS, g.Tables, &cg, &g.Funcs, f0)
			// no soft invent name()+"()" or main without call when build/output fails
			// Failed soft re-pick; nil/empty output sticky incomplete IR
			if inv == nil {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return ""
			}
			if inv.Failed {
				return ""
			}
			firstInv = inv.Output()
			// residual ERROR sticky — no invent main body past inv.Output residual hole
			if HasError() {
				return ""
			}
			if firstInv == "" {
				SetError(ErrGeneric)
				return ""
			}
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
			// Variable* always live; sticky no invent skip nil holes in dump list
			if v == nil {
				SetError(ErrGeneric)
				return ""
			}
			dump := v.OutputValueDump("checksum ", 1, endUnion)
			if dump == "" && HasError() {
				return ""
			}
			b.WriteString(dump)
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
	// OutputMgr.cpp:127–136 — always emit live first invoke when present
	if firstInv != "" {
		b.WriteString("    " + firstInv + ";\n")
		// OutputMgr.cpp:136–140 — OutputPtrResets when !dangling_global_ptrs
		if f0 != nil && !g.Opts.DanglingGlobalPointers {
			resets := OutputPtrResets(f0.DeadGlobals, g.Opts)
			// incomplete dead_globals IR fails closed sticky whole main
			if len(f0.DeadGlobals) > 0 && resets == "" {
				if !HasError() {
					SetError(ErrGeneric)
				}
				return ""
			}
			b.WriteString(resets)
		}
	}
	// OutputMgr.cpp:141–145 — step_hash path invokes csmith_compute_hash; else inline HashGlobalVariables
	// (guarded by compute_hash so crc_* are defined)
	// no invent csmith_compute_hash() call when hashFuncDefReady is false (missing def)
	if g.Opts.ComputeHash {
		if g.hashFuncDefReady() {
			b.WriteString("    csmith_compute_hash();\n")
		} else {
			// OutputMgr.cpp:142 — HashGlobalVariables after OutputArrayInitializers
			// (ctrl vars from that path via get_last_ctrl_vars; no soft GetNew here)
			// also used when step-hash helpers fail closed incomplete ctrl IR
			b.WriteString(g.hashGlobals())
			// residual ERROR sticky — no invent main past hashGlobals residual hole
			if HasError() {
				return ""
			}
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
	// ProgramGenerator always live at emit; sticky incomplete no invent empty hash def
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
	// no invent function shells when helpers disabled or ctrl IR incomplete
	if !g.hashFuncDefReady() {
		return ""
	}
	// OutputMgr.cpp:213–218 — GetMaxArrayDimension + get_new_ctrl_vars + OutputArrayCtrlVars
	// prepare ctrl decl first; no invent partial function shell on incomplete ctrl IR
	dimen := GetMaxArrayDimension(g.VS.GlobalList)
	var ctrlDecl string
	if dimen > 0 {
		ctrl := GetNewCtrlVars(g.Opts)
		ctrlDecl = OutputArrayCtrlVars(ctrl, dimen, "    ")
		if ctrlDecl == "" {
			// incomplete ctrl IR — fail closed empty (call sites use hashFuncDefReady;
			// config undersize soft; broken name sticky inside OutputArrayCtrlVars)
			return ""
		}
	}
	var b strings.Builder
	b.WriteString("\nvoid csmith_compute_hash(void)\n{\n")
	b.WriteString(ctrlDecl)
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
// Incomplete dead_globals fails closed empty (no invent soft-skip hole as partial resets).
func OutputPtrResets(ptrs []*Variable, opts Options) string {
	// incomplete ptr list fails closed sticky (no invent empty-reset success past holes)
	if !VariablesComplete(ptrs) {
		SetError(ErrGeneric)
		return ""
	}
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
		// pre-validated VariablesComplete
		// C++ isArray always ArrayVariable*; missing AsArray sticky
		// (no invent synthetic shell from ArraySizes / soft re-pick partial resets)
		if v.IsArray && v.AsArray == nil {
			SetError(ErrGeneric)
			return ""
		}
		av := v.AsArray
		sizes := v.ArraySizes
		if av != nil && len(av.Sizes) > 0 {
			sizes = av.Sizes
		}
		if v.IsArray && len(sizes) > 0 {
			if len(names) == 0 {
				// C++ assumes get_last_ctrl_vars() non-empty after OutputArrayInitializers
				// incomplete ctrl IR sticky — fail closed whole resets
				if !HasError() {
					SetError(ErrGeneric)
				}
				return ""
			}
			// ArrayVariable::output_init(out, &zero, ctrl_vars, 1) — always loop form
			// force loop even if NoLoopInitializer (globals); pass live zero, not invent
			savedInit, savedExpr := av.Init, av.InitExpr
			av.Init = zero
			av.InitExpr = nil
			initOut := outputArrayInitForced(av, "    ", names, opts.PostIncrOperator)
			av.Init, av.InitExpr = savedInit, savedExpr
			// residual ERROR sticky — no invent soft-continue later resets past init residual
			if HasError() {
				return ""
			}
			// incomplete array init IR sticky — fail closed whole resets
			if initOut == "" {
				SetError(ErrGeneric)
				return ""
			}
			b.WriteString(initOut)
			continue
		}
		// OutputMgr.cpp:337 — Variable::Output always live sticky; no invent " = 0;" without name
		out := v.OutputC()
		// residual ERROR sticky — no invent soft-continue later resets past OutputC residual
		if HasError() {
			return ""
		}
		if out == "" {
			SetError(ErrGeneric)
			return ""
		}
		b.WriteString("    " + out + " = 0;\n")
	}
	return b.String()
}

// outputArrayInitForced is OutputInit without NoLoopInitializer early-out.
// Used by OutputPtrResets (upstream always loops for array dead_globals).
// ArrayVariable.cpp:619–655 — init->Output only; no invent "0" when init missing.
func outputArrayInitForced(av *ArrayVariable, indent string, ctrl []string, postIncr bool) string {
	// ArrayVariable always live for forced init; sticky incomplete no invent empty reset
	if av == nil {
		SetError(ErrGeneric)
		return ""
	}
	// ArrayVariable.cpp:622–623 — collective itemized members skip (policy empty)
	if av.Collective != nil {
		return ""
	}
	// undersized / empty ctrl sticky — no invent letter names
	if len(ctrl) < len(av.Sizes) {
		SetError(ErrGeneric)
		return ""
	}
	for i := range av.Sizes {
		if ctrl[i] == "" {
			SetError(ErrGeneric)
			return ""
		}
	}
	// ArrayVariable.cpp:649 — init->Output; always live Expression* sticky (no invent "0")
	var initVal string
	if av.InitExpr != nil {
		initVal = av.InitExpr.Output()
		// residual ERROR sticky — no invent forced loop-init past Output residual hole
		if HasError() {
			return ""
		}
	} else if av.Init != nil {
		initVal = av.Init.Value
	}
	if initVal == "" {
		SetError(ErrGeneric)
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
	// ArrayVariable::output_with_indices always live access sticky; no invent " = init;" without LHS
	access := av.OutputWithIndices(ctrl)
	// residual ERROR sticky — no invent forced loop-init past OutputWithIndices residual
	if HasError() {
		return ""
	}
	if access == "" {
		SetError(ErrGeneric)
		return ""
	}
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
	// ProgramGenerator always live for goGenerator; sticky incomplete no invent empty program
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
	g.Initialize()
	var b strings.Builder
	b.WriteString(g.OutputHeader())
	// residual ERROR sticky — no invent program past OutputHeader residual hole
	if HasError() {
		return ""
	}
	g.GenerateAllTypes()
	if HasError() {
		return ""
	}
	// struct/union inventory non-empty must emit live decls (no invent types-only skip)
	structsOut := g.OutputStructTypes()
	// residual ERROR sticky — no invent program past OutputStructTypes residual hole
	if HasError() {
		return ""
	}
	if (len(g.Types.StructTypes) > 0 || len(g.Types.UnionTypes) > 0) && structsOut == "" {
		SetError(ErrGeneric)
		return ""
	}
	b.WriteString(structsOut)
	g.GenerateFunctions()
	// Function.cpp:797/805 ERROR_RETURN — stop output when generation failed
	if HasError() {
		return ""
	}
	// make_first always yields a built user function; no invent header-only program
	// Function* always live on Funcs; nil hole fails closed sticky whole program emit
	// (no invent soft-skip hole and still claim hasUser from a later slot)
	hasUser := false
	for _, f := range g.Funcs.Funcs {
		if f == nil {
			SetError(ErrGeneric)
			return ""
		}
		if !f.IsBuiltin && f.BuildState == BuildBuilt && f.Body != nil {
			hasUser = true
			break
		}
	}
	if !hasUser {
		return ""
	}
	// GlobalList non-empty must emit live defs (no invent drop incomplete globals)
	globalsOut := g.OutputGlobals()
	// residual ERROR sticky — no invent program past OutputGlobals residual hole
	if HasError() {
		return ""
	}
	if g.VS != nil && len(g.VS.GlobalList) > 0 && globalsOut == "" {
		SetError(ErrGeneric)
		return ""
	}
	b.WriteString(globalsOut)
	// functions section always live after successful GenerateFunctions
	funcsOut := g.OutputFunctions()
	// residual ERROR sticky — no invent program past OutputFunctions residual hole
	if HasError() {
		return ""
	}
	if funcsOut == "" {
		SetError(ErrGeneric)
		return ""
	}
	b.WriteString(funcsOut)
	// OutputHashFuncDef empty when helpers off or ctrl IR incomplete (main falls back)
	// no invent partial helper shells — OutputHashFuncDef already fail-closed empty
	b.WriteString(g.OutputHashFuncDef())
	// residual ERROR sticky — no invent program past OutputHashFuncDef residual hole
	if HasError() {
		return ""
	}
	// OutputMgr always emits main unless --nomain; incomplete main fails whole program sticky
	mainOut := g.OutputMain()
	// residual ERROR sticky — no invent program past OutputMain residual hole
	if HasError() {
		return ""
	}
	if mainOut == "" && !g.Opts.NoMain {
		SetError(ErrGeneric)
		return ""
	}
	b.WriteString(mainOut)
	// DefaultOutputMgr.cpp:194 — OutputTail after main (statistics comment)
	b.WriteString(OutputTail(g.Funcs.Funcs, g.Opts))
	// residual ERROR sticky — no invent program past OutputTail residual hole
	if HasError() {
		return ""
	}
	// DefaultProgramGenerator.cpp:73–77 — identify_wrappers writes wrapper.h
	// Library-first: append N_WRAP definition as a trailing section for consumers.
	if g.Opts.IdentifyWrappers {
		b.WriteString("\n/* --- wrapper.h (identify_wrappers) ---\n")
		b.WriteString(OutputWrapperH())
		// residual ERROR sticky — no invent program past OutputWrapperH residual hole
		if HasError() {
			return ""
		}
		b.WriteString("--- end wrapper.h --- */\n")
	}
	return b.String()
}

// WrapperHeader returns wrapper.h content when identify_wrappers is set.
// DefaultProgramGenerator.cpp:73–77.
func (g *ProgramGenerator) WrapperHeader() string {
	// ProgramGenerator always live; sticky incomplete no invent empty wrapper header
	if g == nil {
		SetError(ErrGeneric)
		return ""
	}
	// --identify-wrappers off: complete empty (soft option omit)
	if !g.Opts.IdentifyWrappers {
		return ""
	}
	return OutputWrapperH()
}
