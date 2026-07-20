// Upstream: CGOptions.h / CGOptions.cpp (set_default_settings and option fields).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
// MaxGlobals is a Go-only limit until full GlobalList modeling lands; not a C++ pad for RNG.
package csmith

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// processOpts mirrors C++ CGOptions process-wide static configuration.
// Constant::make_random, FactPointTo::is_valid_ptr, choose_var bookkeeping,
// and Block emission read CGOptions; library-first Go keeps an explicit
// process Options set for each generation run (see SetProcessOptions).
//
// processProbs mirrors C++ Probabilities process singleton (same lifetime as
// CGOptions for a generation). CreateVariable / create_field_vars read it;
// nil means not initialized — fail closed, no invent NewProbabilities(opts).
//
// processRng mirrors DefaultRndNumGenerator process instance. CreateVariable
// Constant::make_random burns the same stream as the rest of generation; nil
// means library/test path without NewProgramGenerator (not NewRng(1) invent).
//
// processStmtTab mirrors Statement static ProbabilityTable (pStatementProb).
// Block::make_random / nested GenerateBody share one session table; nil means
// fail closed (no invent NewStatementThresholdTable mid-invocation).
//
// processScopeTab mirrors VariableSelector::scopeTable_ (InitScopeTable once).
// VariableSelectionProbability shares one session table; nil means fail closed
// (no invent NewScopeThresholdTable per draw).
//
// processAssignOpsTab mirrors StatementAssign::assignOpsTable_ (InitProbabilityTable).
// processExprTables mirrors Expression::exprTable_/paramTable_ (InitProbabilityTables).
// Both are set once from Probabilities::initialize_group_probs; nil = fail closed.
var (
	processOptsMu       sync.RWMutex
	processOpts         = Defaults()
	processProbs        *Probabilities
	processRng          *Rng
	processStmtTab      *ThresholdTable
	processScopeTab     *ThresholdTable
	processAssignOpsTab *DistributionTable
	processExprTables   *ExprTables
)

// SetProcessOptions installs the active process Options (CGOptions mirror).
// NewProgramGenerator calls this so CreateVariable / ChooseVarFull / Block.Output
// use session options instead of inventing Defaults().
func SetProcessOptions(o Options) {
	processOptsMu.Lock()
	processOpts = o
	processOptsMu.Unlock()
}

// ProcessOptions returns the active process Options (CGOptions mirror).
// Safe default is Defaults() until SetProcessOptions is called.
func ProcessOptions() Options {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processOpts
}

// SetProcessProbabilities installs the session Probabilities singleton.
// NewProgramGenerator sets this to the same table shared with VS / generator.
func SetProcessProbabilities(p *Probabilities) {
	processOptsMu.Lock()
	processProbs = p
	processOptsMu.Unlock()
}

// ProcessProbabilities returns the active process Probabilities (may be nil).
// C++ Probabilities::GetInstance() is always live after init; nil here is
// fail-closed for library paths that ran without NewProgramGenerator.
func ProcessProbabilities() *Probabilities {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processProbs
}

// SetProcessRng installs the session DefaultRndNumGenerator (shared with generator).
// NewProgramGenerator sets this to the same *Rng used for generation draws.
func SetProcessRng(r *Rng) {
	processOptsMu.Lock()
	processRng = r
	processOptsMu.Unlock()
}

// ProcessRng returns the active process Rng (may be nil outside a generation run).
func ProcessRng() *Rng {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processRng
}

// SetProcessStmtTab installs the session statement probability table.
// NewProgramGenerator sets this to the same StmtTab used for generation.
func SetProcessStmtTab(t *ThresholdTable) {
	processOptsMu.Lock()
	processStmtTab = t
	processOptsMu.Unlock()
}

// ProcessStmtTab returns the active statement ThresholdTable (may be nil).
// C++ Statement probability table is always live after init; nil is fail-closed
// for library paths without NewProgramGenerator.
func ProcessStmtTab() *ThresholdTable {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processStmtTab
}

// SetProcessScopeTab installs the session VariableSelector::scopeTable_.
// NewProgramGenerator / InitScopeTable set this once per generation.
func SetProcessScopeTab(t *ThresholdTable) {
	processOptsMu.Lock()
	processScopeTab = t
	processOptsMu.Unlock()
}

// ProcessScopeTab returns the active scope ThresholdTable (may be nil).
// C++ scopeTable_ is always live after InitScopeTable; nil is fail-closed.
func ProcessScopeTab() *ThresholdTable {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processScopeTab
}

// InitScopeTable mirrors VariableSelector::InitScopeTable.
// VariableSelector.cpp:110–122 — create once from CGOptions::global_variables.
func InitScopeTable(opts Options) {
	SetProcessScopeTab(NewScopeThresholdTable(opts))
}

// SetProcessAssignOpsTable installs StatementAssign::assignOpsTable_.
func SetProcessAssignOpsTable(t *DistributionTable) {
	processOptsMu.Lock()
	processAssignOpsTab = t
	processOptsMu.Unlock()
}

// ProcessAssignOpsTable returns the session assign-ops table (may be nil).
func ProcessAssignOpsTable() *DistributionTable {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processAssignOpsTab
}

// SetProcessExprTables installs Expression::exprTable_/paramTable_ session pair.
func SetProcessExprTables(t *ExprTables) {
	processOptsMu.Lock()
	processExprTables = t
	processOptsMu.Unlock()
}

// ProcessExprTables returns the session Expression term tables (may be nil).
func ProcessExprTables() *ExprTables {
	processOptsMu.RLock()
	defer processOptsMu.RUnlock()
	return processExprTables
}

// InitSessionProbabilityTables mirrors Probabilities::initialize_group_probs
// StatementAssign::InitProbabilityTable + Expression::InitProbabilityTables
// and installs Statement::stmtTable_ from process Probabilities pStatementProb.
// Probabilities.cpp:565–578 / Statement.cpp:133–139.
func InitSessionProbabilityTables(opts Options) {
	SetProcessAssignOpsTable(NewAssignOpsTable(opts))
	SetProcessExprTables(NewExprTables(opts))
	// Statement::InitProbabilityTable — share process probs statement table
	if p := ProcessProbabilities(); p != nil {
		SetProcessStmtTab(p.StatementThresholdTable())
	}
}

const defaultPlatformInfoPath = "platform.info"

// Options is the canonical API-level configuration contract for generation.
// Defaults are aligned with Csmith's CGOptions::set_default_settings where possible.
type Options struct {
	Seed uint64

	// Output/layout
	OutputPath    string
	MaxSplitFiles int
	SplitFilesDir string
	NoMain        bool

	// Target sizing (from platform.info or explicit override)
	PlatformInfoPath string
	IntSize          int
	PointerSize      int
	IntSizeExplicit  bool
	PointerExplicit  bool

	// Size/depth controls
	MaxFuncs             int
	MaxParams            int
	Func1MaxParams       int
	MaxBlockSize         int
	MaxBlockDepth        int
	MaxExprComplexity    int
	MaxStructFields      int
	MaxUnionFields       int
	MaxNestedStructLevel int
	MaxPointerDepth      int
	MaxArrayDim          int
	MaxArrayLenPerDim    int
	MaxArrayLength       int
	MaxArrayNumInLoop    int
	MaxExhaustiveDepth   int
	InlineFunctionProb   int
	BuiltinFunctionProb  int
	ArrayOOBProb         int
	StopByStmt           int
	CoverageTestSize     int

	// Extension/mode switches
	RandomBased   bool
	DFSExhaustive bool
	LangCPP       bool
	CPP11         bool
	FastExecution bool
	DepthProtect  bool

	// Core generation features
	ComputeHash            bool
	AcceptArgc             bool
	Arrays                 bool
	Bitfields              bool
	CompoundAssignment     bool
	Consts                 bool
	Divs                   bool
	Muls                   bool
	EmbeddedAssigns        bool
	CommaOperators         bool
	PreIncrOperator        bool
	PreDecrOperator        bool
	PostIncrOperator       bool
	PostDecrOperator       bool
	UnaryPlusOperator      bool
	Jumps                  bool
	LongLong               bool
	Int8                   bool
	UInt8                  bool
	EnableFloat            bool
	Math64                 bool
	InlineFunction         bool
	Pointers               bool
	Structs                bool
	ReturnStructs          bool
	ArgStructs             bool
	Unions                 bool
	ReturnUnions           bool
	ArgUnions              bool
	TakeUnionFieldAddr     bool
	VolStructUnionFields   bool
	ConstStructUnionFields bool
	Volatiles              bool
	VolatilePointers       bool
	ConstPointers          bool
	GlobalVariables        bool
	StrictConstArrays      bool
	AccessOnce             bool
	StrictVolatileRule     bool
	AddrTakenOfLocals      bool
	DanglingGlobalPointers bool
	NoReturnDeadPointer    bool
	// InterestedFacts is FactMgr meta_facts bitmask (ePointTo|eUnionWrite).
	// 0 means use DefaultInterestedFacts at generation time.
	InterestedFacts          int
	HashValuePrintf          bool
	SignedCharIndex          bool
	ForceGlobalsStatic       bool
	ForceNonUniformArrayInit bool
	Int128                   bool
	UInt128                  bool
	BinaryConstant           bool
	SafeMath                 bool
	PackedStruct             bool
	Paranoid                 bool
	Quiet                    bool
	Concise                  bool
	Builtins                 bool
	RandomRandom             bool
	StepHashByStmt           bool
	ConstAsCondition         bool
	MatchExactQualifiers     bool
	BlindCheckGlobal         bool
	FreshArrayCtrlVarNames   bool
	IdentifyWrappers         bool
	MarkMutableConst         bool
	Klee                     bool
	Crest                    bool
	CComp                    bool
	CoverageTest             bool
	FixedStructFields        bool
	ExpandStruct             bool
	CompactOutput            bool
	PrefixName               bool
	SequenceNamePrefix       bool
	CompatibleCheck          bool
	// NullPointerDerefProb mirrors null_pointer_dereference_prob [0,100]; default 0.
	NullPointerDerefProb int
	// DeadPointerDerefProb mirrors dead_pointer_dereference_prob [0,100]; default 0.
	DeadPointerDerefProb int
	MathNoTmp            bool
	StrictFloat          bool
	WrapVolatiles        bool
	AllowConstVolatile   bool
	FunctionAttributes   bool
	TypeAttributes       bool
	LabelAttributes      bool
	VariableAttributes   bool

	StructOutput             string
	DFSDebugSequence         string
	PartialExpand            string
	DeltaMonitor             string
	DeltaOutput              string
	GoDelta                  string
	DeltaInput               string
	ProbabilityConfiguration string
	DumpDefaultProbabilities string
	DumpRandomProbabilities  string
	SafeMathWrappers         string
	MonitorFuncs             string
	EnableBuiltinKinds       string
	DisableBuiltinKinds      string
	NoDeltaReduction         bool
	// VolTestsMach mirrors CGOptions::vol_tests_mach_ (machine string for vol tests).
	VolTestsMach string

	// Keep an escape hatch for the current simplified generator shape.
	MaxGlobals int
}

func Defaults() Options {
	return Options{
		OutputPath:       "",
		MaxSplitFiles:    0,
		SplitFilesDir:    "",
		NoMain:           false,
		PlatformInfoPath: defaultPlatformInfoPath,
		IntSize:          int(unsafe.Sizeof(int(0))),
		PointerSize:      int(unsafe.Sizeof(uintptr(0))),

		MaxFuncs:             10,
		MaxParams:            5,
		Func1MaxParams:       3,
		MaxBlockSize:         4,
		MaxBlockDepth:        5,
		MaxExprComplexity:    10,
		MaxStructFields:      10,
		MaxUnionFields:       5,
		MaxNestedStructLevel: 3,
		// CGOptions::max_indirect_level = CGOPTIONS_DEFAULT_MAX_INDIRECT_LEVEL (5).
		// Older residual-era default of 2 was wrong vs upstream.
		MaxPointerDepth:     5,
		MaxArrayDim:         3,
		MaxArrayLenPerDim:   10,
		MaxArrayLength:      256,
		MaxArrayNumInLoop:   4,
		MaxExhaustiveDepth:  -1,
		InlineFunctionProb:  50,
		BuiltinFunctionProb: 50,
		ArrayOOBProb:        0,
		StopByStmt:          -1,
		CoverageTestSize:    500,

		RandomBased:   true,
		DFSExhaustive: false,
		LangCPP:       false,
		CPP11:         false,
		FastExecution: false,
		DepthProtect:  false,

		ComputeHash:              true,
		AcceptArgc:               true,
		Arrays:                   true,
		Bitfields:                true,
		CompoundAssignment:       true,
		Consts:                   true,
		Divs:                     true,
		Muls:                     true,
		EmbeddedAssigns:          true,
		CommaOperators:           true,
		PreIncrOperator:          true,
		PreDecrOperator:          true,
		PostIncrOperator:         true,
		PostDecrOperator:         true,
		UnaryPlusOperator:        true,
		Jumps:                    true,
		LongLong:                 true,
		Int8:                     true,
		UInt8:                    true,
		EnableFloat:              false,
		Math64:                   true,
		InlineFunction:           false,
		Pointers:                 true,
		Structs:                  true,
		ReturnStructs:            true,
		ArgStructs:               true,
		Unions:                   true,
		ReturnUnions:             true,
		ArgUnions:                true,
		TakeUnionFieldAddr:       true,
		VolStructUnionFields:     true,
		ConstStructUnionFields:   true,
		Volatiles:                true,
		VolatilePointers:         true,
		ConstPointers:            true,
		GlobalVariables:          true,
		StrictConstArrays:        false,
		AccessOnce:               false,
		StrictVolatileRule:       false,
		AddrTakenOfLocals:        true,
		DanglingGlobalPointers:   true,
		NoReturnDeadPointer:      true,
		HashValuePrintf:          true,
		SignedCharIndex:          true,
		ForceGlobalsStatic:       true,
		ForceNonUniformArrayInit: true,
		Int128:                   false,
		UInt128:                  false,
		BinaryConstant:           false,
		SafeMath:                 true,
		PackedStruct:             true,
		Paranoid:                 false,
		Quiet:                    false,
		Concise:                  false,
		Builtins:                 false,
		RandomRandom:             false,
		StepHashByStmt:           false,
		ConstAsCondition:         false,
		MatchExactQualifiers:     false,
		BlindCheckGlobal:         false,
		FreshArrayCtrlVarNames:   false,
		IdentifyWrappers:         false,
		MarkMutableConst:         false,
		Klee:                     false,
		Crest:                    false,
		CComp:                    false,
		CoverageTest:             false,
		FixedStructFields:        false,
		ExpandStruct:             false,
		CompactOutput:            false,
		PrefixName:               false,
		SequenceNamePrefix:       false,
		CompatibleCheck:          false,
		NullPointerDerefProb:     0,
		DeadPointerDerefProb:     0,
		MathNoTmp:                false,
		StrictFloat:              false,
		WrapVolatiles:            false,
		AllowConstVolatile:       true,
		FunctionAttributes:       false,
		TypeAttributes:           false,
		LabelAttributes:          false,
		VariableAttributes:       false,
		StructOutput:             "",
		DFSDebugSequence:         "",
		PartialExpand:            "",
		DeltaMonitor:             "",
		DeltaOutput:              "",
		GoDelta:                  "",
		DeltaInput:               "",
		ProbabilityConfiguration: "",
		DumpDefaultProbabilities: "",
		DumpRandomProbabilities:  "",
		SafeMathWrappers:         "",
		MonitorFuncs:             "",
		EnableBuiltinKinds:       "",
		DisableBuiltinKinds:      "",
		NoDeltaReduction:         false,
		VolTestsMach:             "",

		MaxGlobals: 80,
	}
}

// IsRandom mirrors CGOptions::is_random.
// CGOptions.cpp:380 — return random_based_.
func (o Options) IsRandom() bool { return o.RandomBased }

// FuncAttrFlag mirrors CGOptions::func_attr_flag — FunctionAttributes option.
// CGOptions.cpp DEFINE_GETTER_SETTER_BOOL(func_attr_flag).
func (o Options) FuncAttrFlag() bool { return o.FunctionAttributes }

// VolTestsMachValue mirrors CGOptions::vol_tests_mach.
// CGOptions.cpp:589–591.
func (o Options) VolTestsMachValue() string { return o.VolTestsMach }

// SetVolTests mirrors CGOptions::set_vol_tests.
// Declared CGOptions.h:289; no .cpp body on pin 0cdc710 (header-only / dead link).
// Last known body (csmith-2.1.0 CGOptions.cpp): accept "x86"|"x86_64", store mach, return true.
// enable_vol_tests flag was removed from the pin; only vol_tests_mach_ remains (Variable dump).
// Incomplete/unknown mach fails closed false (no invent silent store of invalid host string).
func (o *Options) SetVolTests(s string) bool {
	if o == nil {
		SetError(ErrGeneric)
		return false
	}
	if s == "x86" || s == "x86_64" {
		o.VolTestsMach = s
		return true
	}
	return false
}

// ApplyMonitoredFuncs installs Options.MonitorFuncs into OutputMgr process list.
// Call from generation setup (CGOptions::monitored_funcs).
func (o Options) ApplyMonitoredFuncs() {
	SetMonitoredFuncs(o.MonitorFuncs)
}

// AllowInt64 mirrors CGOptions::allow_int64.
// CGOptions.cpp: !has_extension_support() && math64() && longlong().
func (o Options) AllowInt64() bool {
	if o.Klee || o.Crest || o.CoverageTest {
		return false
	}
	return o.Math64 && o.LongLong
}

func (o Options) resolvePlatformInfo() (Options, error) {
	path := strings.TrimSpace(o.PlatformInfoPath)
	if path == "" {
		path = defaultPlatformInfoPath
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			if o.IntSize <= 0 {
				o.IntSize = int(unsafe.Sizeof(int(0)))
			}
			if o.PointerSize <= 0 {
				o.PointerSize = int(unsafe.Sizeof(uintptr(0)))
			}
			return o, nil
		}
		return o, err
	}
	defer f.Close()

	seenInt := false
	seenPtr := false
	fileInt := 0
	filePtr := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "integer size =") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "integer size ="))
			n, err := strconv.Atoi(v)
			if err != nil {
				return o, fmt.Errorf("invalid integer size in %s", path)
			}
			fileInt = n
			seenInt = true
		}
		if strings.HasPrefix(line, "pointer size =") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "pointer size ="))
			n, err := strconv.Atoi(v)
			if err != nil {
				return o, fmt.Errorf("invalid pointer size in %s", path)
			}
			filePtr = n
			seenPtr = true
		}
	}
	if err := scanner.Err(); err != nil {
		return o, err
	}
	if !seenInt {
		return o, fmt.Errorf("please specify integer size in %s", path)
	}
	if !seenPtr {
		return o, fmt.Errorf("please specify pointer size in %s", path)
	}
	if !o.IntSizeExplicit {
		o.IntSize = fileInt
	}
	if !o.PointerExplicit {
		o.PointerSize = filePtr
	}
	return o, nil
}

func (o Options) Validate() error {
	if o.IntSize <= 0 {
		return fmt.Errorf("int-size must be positive")
	}
	if o.PointerSize <= 0 {
		return fmt.Errorf("ptr-size must be positive")
	}
	if o.MaxFuncs < 1 {
		return fmt.Errorf("max-funcs must be at least 1")
	}
	if o.MaxBlockSize < 1 {
		return fmt.Errorf("max-block-size must be at least 1")
	}
	if o.MaxBlockDepth < 1 {
		return fmt.Errorf("max-stmt-depth must be at least 1")
	}
	if o.MaxGlobals < 1 {
		return fmt.Errorf("max-globals must be at least 1")
	}
	if o.Func1MaxParams > o.MaxParams {
		return fmt.Errorf("func1_max_params() cannot be larger than max_params()")
	}
	if o.InlineFunctionProb < 0 || o.InlineFunctionProb > 100 {
		return fmt.Errorf("inline-function-prob value must between [0,100]")
	}
	if o.BuiltinFunctionProb < 0 || o.BuiltinFunctionProb > 100 {
		return fmt.Errorf("builtin-function-prob value must between [0,100]")
	}
	if o.ArrayOOBProb < 0 || o.ArrayOOBProb > 100 {
		return fmt.Errorf("array-oob-prob value must between [0,100]")
	}
	// CGOptions.cpp — single knobs only (null_pointer / dead_pointer_dereference_prob)
	if o.NullPointerDerefProb < 0 || o.NullPointerDerefProb > 100 {
		return fmt.Errorf("null-pointer-dereference-prob value must between [0,100]")
	}
	if o.DeadPointerDerefProb < 0 || o.DeadPointerDerefProb > 100 {
		return fmt.Errorf("dead-pointer-dereference-prob value must between [0,100]")
	}
	if !o.LangCPP && o.CPP11 {
		return fmt.Errorf("--cpp11 option makes sense only with --lang-cpp option enabled")
	}
	if o.DFSExhaustive {
		if o.MaxExhaustiveDepth <= 0 {
			return fmt.Errorf("max-exhaustive-depth must be at least 0")
		}
		if !o.Structs && o.ExpandStruct {
			return fmt.Errorf("expand-struct/struct-specific options cannot be used with --no-structs")
		}
		if o.Klee || o.Crest || o.CoverageTest {
			return fmt.Errorf("exhaustive mode doesn't support klee|crest|coverage-test extension")
		}
	}
	if o.RandomBased {
		if o.DFSExhaustive {
			return fmt.Errorf("random-based and dfs-exhaustive modes cannot both be enabled")
		}
		if o.SequenceNamePrefix {
			return fmt.Errorf("--sequence-name-prefix option can only be used with --dfs-exhaustive")
		}
	}
	if !o.RandomBased {
		if o.MaxSplitFiles > 0 {
			return fmt.Errorf("max_split_files can only be applied to random mode")
		}
		if o.SplitFilesDir != "" {
			return fmt.Errorf("split_files_dir can only be applied to random mode")
		}
	}
	if o.DeltaMonitor != "" && o.GoDelta != "" {
		return fmt.Errorf("you cannot specify --delta-monitor and --go-delta monitor at the same time")
	}
	// CGOptions.cpp:525–532 — empty split_files_dir → "./output"; create only if max_split_files>0
	if o.SplitFilesDir == "" {
		o.SplitFilesDir = "./output"
	}
	if o.MaxSplitFiles > 0 {
		if err := os.MkdirAll(o.SplitFilesDir, 0o755); err != nil {
			return fmt.Errorf("cannot create dir for split files: %w", err)
		}
	}
	extCount := 0
	if o.Klee {
		extCount++
	}
	if o.Crest {
		extCount++
	}
	if o.CoverageTest {
		extCount++
	}
	if extCount > 1 {
		return fmt.Errorf("you could only specify --klee or --crest or --coverage-test")
	}
	return nil
}

func (o Options) normalizeUpstreamFlow() Options {
	// Upstream fast-execution turns on C++ mode and tightens options.
	if o.FastExecution {
		o.LangCPP = true
		o.Jumps = false
		o.MaxArrayLenPerDim = min(o.MaxArrayLenPerDim, 5)
	}
	// Upstream C++ normalization.
	if o.LangCPP {
		o.MatchExactQualifiers = true
		o.VolStructUnionFields = false
		o.ConstStructUnionFields = false
	}
	// Upstream DFS mode forces fixed struct fields.
	// CGOptions.cpp:410–417 resolve_exhaustive_options side effects.
	if o.DFSExhaustive {
		o.FixedStructFields = true
		if o.CompatibleCheck {
			EnableCompatibleCheck()
		}
	}
	return o
}

func (o Options) validate() error {
	return o.normalizeUpstreamFlow().Validate()
}
