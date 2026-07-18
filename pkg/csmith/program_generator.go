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
		Opts:    opts,
		Seed:    seed,
		Rng:     r,
		Probs:   probs,
		VS:      vs,
		Tables:  NewExprTables(opts),
		StmtTab: NewStatementThresholdTable(opts),
	}
	// Share gensym + derived_types across selector and generator.
	vs.Types = &g.Types
	return g
}

// Initialize mirrors DefaultProgramGenerator::initialize (RNG already seeded).
func (g *ProgramGenerator) Initialize() {
	// Type::GenerateSimpleTypes is satisfied by GetSimpleType cache.
	// ExtensionMgr::CreateExtension — null default, nothing to do.
}

// GenerateAllTypes mirrors GenerateAllTypes for non-dfs mode without structs/unions yet.
// Type.cpp:1179–1202 — GenerateSimpleTypes; struct/union deferred (MoreTypesProbability loop).
func (g *ProgramGenerator) GenerateAllTypes() {
	// Simple types are pre-cached. Struct/union generation not ported.
}

// GenerateFunctions mirrors Function.cpp GenerateFunctions (no builtins / FactMgr).
// make_first then GenerateBody for any unbuilt (none if make_first builds body).
func (g *ProgramGenerator) GenerateFunctions() {
	if g == nil {
		return
	}
	g.Funcs.Types = &g.Types
	// Function::make_first
	_ = MakeFirst(g.Rng, g.Opts, g.Probs, g.VS, &g.VS.Sym, g.Tables, g.StmtTab, &g.Funcs)
	// Create body of each function until no new unbuilt remain (Function.cpp:801–807).
	for i := 0; i < len(g.Funcs.Funcs); i++ {
		f := g.Funcs.Funcs[i]
		if f != nil && !f.IsBuilt {
			cg := EmptyCGContext().WithFuncList(&g.Funcs)
			cg.CurrentFunc = f
			cg.Types = &g.Types
			f.GenerateBody(g.Rng, g.Opts, g.Probs, g.VS, g.Tables, g.StmtTab, cg)
		}
	}
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
	return b.String()
}

// OutputGlobals emits GlobalList declarations.
func (g *ProgramGenerator) OutputGlobals() string {
	if g == nil || g.VS == nil || len(g.VS.GlobalList) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("/* --- GLOBAL VARIABLES --- */\n\n")
	for _, v := range g.VS.GlobalList {
		if v == nil || v.Type == nil {
			continue
		}
		// force_globals_static default true
		if g.Opts.ForceGlobalsStatic {
			b.WriteString("static ")
		}
		if v.IsConst() {
			b.WriteString("const ")
		}
		if v.IsVolatile() {
			b.WriteString("volatile ")
		}
		b.WriteString(v.Type.CName())
		b.WriteString(" ")
		b.WriteString(v.Name)
		if v.Init != nil && v.Init.Value != "" {
			b.WriteString(" = ")
			b.WriteString(v.Init.Value)
		}
		b.WriteString(";\n")
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
		b.WriteString(f.OutputForwardDecl())
		b.WriteString("\n")
	}
	b.WriteString("\n\n/* --- FUNCTIONS --- */\n")
	for _, f := range g.Funcs.Funcs {
		if f == nil || f.IsBuiltin {
			continue
		}
		b.WriteString(f.Output())
		b.WriteString("\n")
	}
	return b.String()
}

// OutputMain mirrors OutputMgr::OutputMain (no extension, simplified hash).
// OutputMgr.cpp:92–153.
func (g *ProgramGenerator) OutputMain() string {
	if g.Opts.NoMain {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n/* ---------------------------------------- */\n")
	if g.Opts.AcceptArgc {
		b.WriteString("int main (int argc, char* argv[])\n{\n")
	} else {
		b.WriteString("int main (void)\n{\n")
	}
	b.WriteString("    int print_hash_value = 0;\n")
	if g.Opts.AcceptArgc {
		b.WriteString("    if (argc == 2 && strcmp(argv[1], \"1\") == 0) print_hash_value = 1;\n")
	}
	b.WriteString("    platform_main_begin();\n")
	if g.Opts.ComputeHash {
		b.WriteString("    crc32_gentab();\n")
	}
	// First function invocation
	if len(g.Funcs.Funcs) > 0 && g.Funcs.Funcs[0] != nil {
		f0 := g.Funcs.Funcs[0]
		b.WriteString("    ")
		if f0.ReturnType != nil && !(f0.ReturnType.IsSimple() && f0.ReturnType.Simple() == EVoid) {
			// discard return like many drivers; cast void
			b.WriteString("(void)")
		}
		b.WriteString(f0.Name)
		b.WriteString("();\n")
	}
	if g.Opts.ComputeHash {
		for _, v := range g.VS.GlobalList {
			if v == nil {
				continue
			}
			// HashGlobalVariables simplified: transparent_crc for integers
			b.WriteString(fmt.Sprintf("    transparent_crc(%s, \"%s\", print_hash_value);\n", v.Name, v.Name))
		}
		b.WriteString("    platform_main_end(crc32_context ^ 0xFFFFFFFFUL, print_hash_value);\n")
	} else {
		b.WriteString("    platform_main_end(0,0);\n")
	}
	b.WriteString("}\n")
	return b.String()
}

// GoGenerator mirrors DefaultProgramGenerator::goGenerator.
// DefaultProgramGenerator.cpp:67–72.
func (g *ProgramGenerator) GoGenerator() string {
	g.Initialize()
	var b strings.Builder
	b.WriteString(g.OutputHeader())
	g.GenerateAllTypes()
	g.GenerateFunctions()
	b.WriteString(g.OutputGlobals())
	b.WriteString(g.OutputFunctions())
	b.WriteString(g.OutputMain())
	return b.String()
}
