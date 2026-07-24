// Session owns all mutable generation state for one run (or the unit-test bag).
//
// Rule: nothing writable is a package-level var of generation state.
//   - Read-only package data only: const tables, name maps, builtin string lists.
//   - All mutable run state lives on *Session.
//
// Access bridge (temporary): currentSession() returns the active bag so existing
// Process*/package helpers keep working without threading *Session through every
// call yet. The bridge itself is the last global write; next step is to pass
// *Session explicitly and delete activeSession.
//
// Concurrent Generate in one process is unsupported (upstream: one gen/process).
// Fuzz workers are separate OS processes.
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

// defaultSession is the bag used by unit tests outside Generate.
// It is still a heap object, not "generation state spilled into package vars".
var defaultSession = newSession()

// activeSession is the only remaining package-level write for the Process* bridge.
// TODO: pass *Session explicitly and delete this.
var activeSession *Session

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

func currentSession() *Session {
	if activeSession != nil {
		return activeSession
	}
	return defaultSession
}

// activateSession makes s the Process* target until restore.
func activateSession(s *Session) (restore func()) {
	prev := activeSession
	activeSession = s
	return func() { activeSession = prev }
}

// BeginGenerateSession installs a fresh session for one full-program Generate.
func BeginGenerateSession() (restore func()) {
	return activateSession(newSession())
}

// CurrentSession returns the active session bag (tests / migration helpers).
func CurrentSession() *Session { return currentSession() }
