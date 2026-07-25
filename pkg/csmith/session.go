// Session owns all mutable generation state for one run (or the unit-test bag).
//
// Public pure API:
//
//	out, err := NewSession(opts).Generate(ctx)
//
// Rule: generation mutables live only on *Session. Generate is bag-local
// (g.Sess / cg.Sess); it never installs the unit-test ambient bag and does
// not toggle any package meta lock (pureGenStrict deleted).
//
// Quarantined ambient (unit tests only):
//   - testAmbientSession + Process*/SetError bridges for legacy unit tests
//   - NewVariableSelector / NewFactMgr / EmptyCGContext install ambient
//   - vsSess/envSess/fmSess/cgSess/gSess panic if owner.Sess unset
//     (constructors / NewProgramGenerator set it)
//   - sessOrAmbient/sessNoteError/sessOpts/sessProbs/sessRng(nil) panics
//
// Read-only package data: const tables, name maps, builtin lists, simpleTypes
// (canonical eSimple *Type cache — Used marks live on Session.simpleUsed, not
// on the package Type objects).
//
// Concurrent Generate in one process is unsupported (upstream: one gen/process).
// Fuzz workers are separate OS processes.
// Generate is not fully pure while Process* bridges + testAmbientSession remain.
//
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// Session is the full mutable generation bag (CGOptions + C++ statics mirror).
type Session struct {
	Opts         Options
	Probs        *Probabilities
	Rng          *Rng
	StmtTab      *ThresholdTable
	ScopeTab     *ThresholdTable
	AssignOpsTab *DistributionTable
	ExprTables   *ExprTables
	ProgramGen   *ProgramGenerator
	RandomNumber *RandomNumber

	// Error::r_error_
	GenError int

	// Statement::sid
	NextStmID int
	// StatementGoto::stm_labels
	StmLabels map[int]string

	// Type::derived_types pointer cache + platform sizes for SizeInBytes
	PointerCache    map[*Type]*Type
	PlatformIntSize int
	PlatformPtrSize int
	// simpleUsed is Type::used for package simpleTypes[] (process-static *Type
	// cache must not carry run-local used marks — C++ process-static Used would
	// race multi-Generate; library bags keep marks session-local).
	simpleUsed [MaxSimpleTypes]bool

	// Bookkeeper static counters (subset; full fields inlined via package funcs)
	BK bookkeeperState

	// OutputMgr statics
	MonitoredFuncs []string
	CurrFunc       string
	OutputMgrKind  OutputMgrKind
	StructOutput   string
	SplitPaths     []string
	OutputFile     string

	// PartialExpander
	PartialExpands       map[StatementType]bool
	PartialExpandsBackup map[StatementType]bool

	// ArrayVariable static seed
	ArrayInitSeed uint32

	// util gensym_count + analysis errlog
	GenSym         GenSym
	AnalysisErrLog strings.Builder

	// CompatibleChecker process static
	CompatibleCheck bool

	// ExtensionMgr / AbsExtension process values
	ExtensionActive bool
	ExtValues       []*ExtensionValue
	ExtKind         string
	CoverageSize    int
	CoverageTests   []*Constant

	// Attribute generators
	VarAttrGenerator   *AttributeGenerator
	FuncAttrGenerator  *AttributeGenerator
	LabelAttrGenerator *AttributeGenerator
	StructTypeAttrGen  *AttributeGenerator
	UnionTypeAttrGen   *AttributeGenerator

	// FactMgr meta facts
	MetaFactPointToEnabled  bool
	MetaFactUnionEnabled    bool
	InUserInvocationRevisit bool

	// Variable::ctrl_vars
	CtrlVarsVectors [][]*Variable
	CtrlVarsCount   uint64

	// FunctionInvocationUser return-fact registry
	ReturnFactInvocations  []*Invocation
	ReturnFactPoints       []*FactPointTo
	ReturnUnionInvocations []*Invocation
	ReturnUnionFacts       []*FactUnion

	// FactPointTo aggregates
	AllPtrs    []*Variable
	AllAliases [][]*Variable

	// Analysis / DFS helpers
	FailedStm           *Stmt
	DFSImpl             *Rng
	SequenceFactorySep  byte
	SequenceFactoryLive []*LinearSequence
	WrapperNames        []string
}

// bookkeeperState is Bookkeeper.cpp static counters for one session.
type bookkeeperState struct {
	structDepthCnts                  []int
	unionVarCnt                      int
	exprDepthCnts                    []int
	blkDepthCnts                     []int
	dereferenceLevelCnts             []int
	addressTakenCnt                  int
	writeDereferenceCnts             []int
	readDereferenceCnts              []int
	cmpPtrToNull                     int
	cmpPtrToPtr                      int
	cmpPtrToAddr                     int
	readVolatileCnt                  int
	writeVolatileCnt                 int
	readNonVolatileCnt               int
	writeNonVolatileCnt              int
	readVolatileThruPtrCnt           int
	writeVolatileThruPtrCnt          int
	pointerAvailForDeref             int
	volatileAvail                    int
	structsWithBitfields             int
	varsWithBitfields                []int
	varsWithFullBitfields            []int
	varsWithBitfieldsAddressTakenCnt int
	bitfieldsInTotal                 int
	unamedBitfieldsInTotal           int
	constBitfieldsInTotal            int
	volatileBitfieldsInTotal         int
	lhsBitfieldsStructsVarsCnt       int
	rhsBitfieldsStructsVarsCnt       int
	lhsBitfieldCnt                   int
	rhsBitfieldCnt                   int
	forwardJumpCnt                   int
	backwardJumpCnt                  int
	useNewVarCnt                     int
	useOldVarCnt                     int
	oobCnt                           int
	relyOnIntSize                    bool
	relyOnPtrSize                    bool
}

// testAmbientSession is the quarantined Process* bag for unit tests outside
// Generate. Generate never installs or writes this object. Non-Sess Process*
// wrappers pass it explicitly into *Sess helpers. Delete once unit tests pass
// explicit *Session and non-Sess bridges are removed.
var testAmbientSession = newSession()

func newSession() *Session {
	s := &Session{
		Opts:                   Defaults(),
		PlatformIntSize:        4,
		PlatformPtrSize:        8,
		ArrayInitSeed:          0xABCDEF,
		MetaFactPointToEnabled: true,
		MetaFactUnionEnabled:   true,
		SequenceFactorySep:     LinearSequenceDefaultSep,
		StmLabels:              map[int]string{},
		PointerCache:           map[*Type]*Type{},
	}
	return s
}

// currentSession returns the quarantined unit-test ambient bag.
// Not used by Generate (bag-local s / g.Sess / cg.Sess only).
func currentSession() *Session {
	return testAmbientSession
}

// sessOrAmbient returns s when non-nil; panics on nil.
// Unit-test bridges must pass testAmbientSession (or vsSess/cgSess/fmSess/envSess)
// explicitly — no silent ambient dual-fill from residual *Sess(nil).
func sessOrAmbient(s *Session) *Session {
	if s != nil {
		return s
	}
	panic("residual ambient sessOrAmbient(nil)")
}

// firstSess returns the first non-nil session among a, b.
// Both nil panics — no silent ambient dual-fill (pass vsSess/cgSess/gSess or
// testAmbientSession explicitly).
func firstSess(a, b *Session) *Session {
	if a != nil {
		return a
	}
	if b != nil {
		return b
	}
	panic("firstSess: both nil (pass vsSess/cgSess or testAmbientSession)")
}

// NewSession constructs a pure run bag with the given options (no ambient write).
func NewSession(opts Options) *Session {
	s := newSession()
	s.Opts = opts
	return s
}

// CurrentSession returns the quarantined unit-test ambient bag.
// Prefer holding *Session from NewSession; do not use from Generate paths.
func CurrentSession() *Session { return currentSession() }
