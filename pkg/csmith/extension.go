// Upstream: ExtensionValue.* / AbsExtension.* / ExtensionMgr.*
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// ExtensionValue mirrors ExtensionValue — named typed extension param/value.
// ExtensionValue.cpp:42–52.
type ExtensionValue struct {
	Type  *Type
	Name  string
	Value *Constant
	Qfer  CVQualifiers
}

// NewExtensionValue mirrors ExtensionValue::ExtensionValue(type, name).
// ExtensionValue.cpp:42–52 — qfer non-const non-vol at depth 0.
func NewExtensionValue(typ *Type, name string) *ExtensionValue {
	// Type always live; sticky nil (no invent void/default type shell)
	if typ == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	// empty name sticky (no invent base_name_only)
	if name == "" {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	return &ExtensionValue{
		Type: typ,
		Name: name,
		Qfer: CVQualifiers{
			IsConsts:    []bool{false},
			IsVolatiles: []bool{false},
		},
	}
}

// AbsExtensionTab mirrors AbsExtension::tab_ ("    ").
const AbsExtensionTab = "    "

// AbsExtensionBaseName mirrors AbsExtension::base_name_ ("x").
const AbsExtensionBaseName = "x"

// AbsExtensionInitialize mirrors AbsExtension::Initialize.
// AbsExtension.cpp:52–62 — num simple types named x0..x{n-1}.
// Incomplete num or RNG/probs sticky nil (no invent empty values as success).
func AbsExtensionInitialize(num int, r *Rng, probs *Probabilities) []*ExtensionValue {
	return AbsExtensionInitializeSess(nil, num, r, probs)
}

func AbsExtensionInitializeSess(s *Session, num int, r *Rng, probs *Probabilities) []*ExtensionValue {
	if num < 0 {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if num == 0 {
		return nil
	}
	// Rng + Probabilities always live for choose_random_simple
	if r == nil || probs == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	values := make([]*ExtensionValue, 0, num)
	for i := 0; i < num; i++ {
		st := ChooseRandomNonvoidSimple(r, probs)
		if sessHasError(s) {
			return nil
		}
		typ := GetSimpleType(st)
		if typ == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		ev := NewExtensionValue(typ, fmt.Sprintf("%s%d", AbsExtensionBaseName, i))
		if ev == nil || sessHasError(s) {
			return nil
		}
		values = append(values, ev)
	}
	return values
}

// AbsExtensionDefaultOutputDefinitions mirrors default_output_definitions.
// AbsExtension.cpp:93–105 — tab + type + name [ = 0]; per value.
// Incomplete values sticky "" (no invent partial definitions section).}

func AbsExtensionDefaultOutputDefinitions(values []*ExtensionValue, initFlag bool) string {
	return AbsExtensionDefaultOutputDefinitionsSess(nil, values, initFlag)
}

func AbsExtensionDefaultOutputDefinitionsSess(s *Session, values []*ExtensionValue, initFlag bool) string {
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if len(values) == 0 {
		return "\n"
	}
	var b strings.Builder
	for _, v := range values {
		b.WriteString(AbsExtensionTab)
		// Type::Output
		cn := v.Type.CName()
		if sessHasError(s) || cn == "" {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return ""
		}
		b.WriteString(cn)
		b.WriteString(" ")
		b.WriteString(v.Name)
		if initFlag {
			b.WriteString(" = 0")
		}
		b.WriteString(";\n")
	}
	b.WriteString("\n")
	return b.String()
}

// AbsExtensionOutputFirstFunInvocation mirrors OutputFirstFunInvocation.
// AbsExtension.cpp:107–113 — "    " + invoke.Output + ";\n".
// Incomplete invoke string sticky "" (no invent bare ";" call).}

func AbsExtensionOutputFirstFunInvocation(invokeOut string) string {
	if invokeOut == "" {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return AbsExtensionTab + invokeOut + ";\n"
}

// AbsExtensionGenerateFirstParameterList mirrors GenerateFirstParameterList.
// AbsExtension.cpp:81–90 — GenerateParameterVariable per value into func.Params.
// Incomplete func/values/VS sticky false.
func AbsExtensionGenerateFirstParameterList(f *Function, values []*ExtensionValue, vs *VariableSelector) bool {
	var s *Session
	if vs != nil {
		s = vs.Sess
	}
	return AbsExtensionGenerateFirstParameterListSess(s, f, values, vs)
}

// AbsExtensionGenerateFirstParameterListSess is AbsExtensionGenerateFirstParameterList on bag s.
func AbsExtensionGenerateFirstParameterListSess(s *Session, f *Function, values []*ExtensionValue, vs *VariableSelector) bool {
	if f == nil || vs == nil {
		sessNoteError(s, ErrGeneric)
		return false
	}
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
		return false
	}
	for _, ev := range values {
		q := ev.Qfer
		v := vs.GenerateParameterVariableTyped(ev.Type, q)
		if v == nil || sessHasError(s) {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return false
		}
		f.Param = append(f.Param, v)
	}
	return true
}

// AbsExtensionMakeFuncInvocation mirrors MakeFuncInvocation.
// AbsExtension.cpp:64–78 — FunctionInvocationUser with ExpressionVariable params.
// Incomplete sticky nil.
func AbsExtensionMakeFuncInvocation(f *Function, values []*ExtensionValue) *Invocation {
	return AbsExtensionMakeFuncInvocationSess(nil, f, values)
}

// AbsExtensionMakeFuncInvocationSess is AbsExtensionMakeFuncInvocation on bag s.
func AbsExtensionMakeFuncInvocationSess(s *Session, f *Function, values []*ExtensionValue) *Invocation {
	if f == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	// Build a FunctionInvocationUser with fixed ExpressionVariable params.
	fi := &Invocation{User: f}
	for _, ev := range values {
		// VariableSelector::new_variable(name, type, null init, qfer)
		v := CreateVariableScalarsSess(s, ev.Name, ev.Type, false, false)
		if v == nil {
			sessNoteError(s, ErrGeneric)
			return nil
		}
		v.Qfer = ev.Qfer
		// ExpressionVariable
		e := &Expression{
			Term:     TermVariable,
			Var:      v,
			ExprType: ev.Type,
		}
		fi.Args = append(fi.Args, e)
	}
	return fi
}

func extensionValuesComplete(values []*ExtensionValue) bool {
	for _, v := range values {
		if v == nil || v.Type == nil || v.Name == "" {
			return false
		}
	}
	return true
}

// --- ExtensionMgr process singleton ---

// processExtension mirrors ExtensionMgr::extension_ (nil = default null extension).

// CreateExtension mirrors ExtensionMgr::CreateExtension.
// ExtensionMgr.cpp:48–64 — klee/crest/coverage_test construct; else leave null.
// Uses process RNG/probs when live; incomplete → sticky fail closed.
func CreateExtension(opts Options) {
	CreateExtensionSess(nil, opts)
}

// CreateExtensionSess is CreateExtension on an explicit session bag.
func CreateExtensionSess(s *Session, opts Options) {
	s = sessOrAmbient(s)
	if !opts.Klee && !opts.Crest && !opts.CoverageTest {
		s.ExtensionActive = false
		s.ExtKind = ""
		s.ExtValues = nil
		s.CoverageTests = nil
		return
	}
	r := sessRng(s)
	probs := sessProbs(s)
	if r == nil || probs == nil {
		// CreateInstance may not have run yet — library paths pass via CreateExtensionFull
		sessNoteError(s, ErrGeneric)
		return
	}
	CreateExtensionFullSess(s, opts, r, probs)
}

// DestroyExtension// DestroyExtension mirrors ExtensionMgr::DestroyExtension.
// ExtensionMgr.cpp:66–69.
func DestroyExtension() {
	DestroyExtensionSess(nil)
}

// DestroyExtensionSess clears ExtensionMgr state on an explicit session bag.
func DestroyExtensionSess(s *Session) {
	s = sessOrAmbient(s)
	s.ExtensionActive = false
	s.ExtKind = ""
	s.ExtValues = nil
	s.CoverageTests = nil
	s.CoverageSize = 0
}

// ExtensionActive is true when a non-null AbsExtension is installed.
func ExtensionActive() bool { return ExtensionActiveSess(nil) }

// ExtensionActiveSess reports ExtensionActive on an explicit session bag.
func ExtensionActiveSess(s *Session) bool { return sessOrAmbient(s).ExtensionActive }

// ExtensionMgrGenerateValues mirrors ExtensionMgr::GenerateValues.
// ExtensionMgr.cpp:84–88 — null → no-op; Klee/Crest empty; Coverage already built.
func ExtensionMgrGenerateValues() { ExtensionMgrGenerateValuesSess(nil) }

// ExtensionMgrGenerateValuesSess is ExtensionMgrGenerateValues on an explicit bag.
func ExtensionMgrGenerateValuesSess(s *Session) {
	if !ExtensionActiveSess(s) {
		return
	}
	// Klee/Crest GenerateValues are empty; Coverage fills at Create
}

// ExtensionMgrGenerateFirstParameterList mirrors GenerateFirstParameterList.
// ExtensionMgr.cpp:77–82 — null → no-op.
func ExtensionMgrGenerateFirstParameterList(f *Function, vs *VariableSelector) {
	ExtensionMgrGenerateFirstParameterListSess(nil, f, vs)
}

// ExtensionMgrGenerateFirstParameterListSess is GenerateFirstParameterList on bag s.
func ExtensionMgrGenerateFirstParameterListSess(s *Session, f *Function, vs *VariableSelector) {
	s = sessOrAmbient(s)
	if !s.ExtensionActive {
		return
	}
	if !AbsExtensionGenerateFirstParameterListSess(s, f, s.ExtValues, vs) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
	}
}

// ExtensionMgrOutputHeader mirrors OutputHeader — null → empty.
// ExtensionMgr.cpp:101–107.
func ExtensionMgrOutputHeader() string { return ExtensionMgrOutputHeaderSess(nil) }

// ExtensionMgrOutputHeaderSess is ExtensionMgrOutputHeader on an explicit bag.
func ExtensionMgrOutputHeaderSess(s *Session) string {
	s = sessOrAmbient(s)
	if !s.ExtensionActive {
		return ""
	}
	switch s.ExtKind {
	case "klee":
		return KleeOutputHeader()
	case "crest":
		return CrestOutputHeader()
	case "coverage":
		return "" // CoverageTestExtension::OutputHeader empty
	default:
		sessNoteError(s, ErrGeneric)
		return ""
	}
}

// ExtensionMgrOutputTail mirrors OutputTail.
// ExtensionMgr.cpp:109–115 — null → "    return 0;\n".
func ExtensionMgrOutputTail() string { return ExtensionMgrOutputTailSess(nil) }

// ExtensionMgrOutputTailSess is ExtensionMgrOutputTail on an explicit bag.
func ExtensionMgrOutputTailSess(s *Session) string {
	s = sessOrAmbient(s)
	if !s.ExtensionActive {
		return AbsExtensionTab + "return 0;\n"
	}
	switch s.ExtKind {
	case "klee":
		return KleeOutputTail()
	case "crest":
		return CrestOutputTail()
	case "coverage":
		return CoverageOutputTail()
	default:
		sessNoteError(s, ErrGeneric)
		return ""
	}
}

// ExtensionMgrOutputInit mirrors OutputInit.
// ExtensionMgr.cpp:117–129 — null → main signature + "{".
func ExtensionMgrOutputInit(acceptArgc bool) string {
	return ExtensionMgrOutputInitSess(nil, acceptArgc)
}

// ExtensionMgrOutputInitSess is ExtensionMgrOutputInit on an explicit bag.
func ExtensionMgrOutputInitSess(s *Session, acceptArgc bool) string {
	s = sessOrAmbient(s)
	if s.ExtensionActive {
		switch s.ExtKind {
		case "klee":
			return KleeOutputInitSess(s, s.ExtValues)
		case "crest":
			return CrestOutputInitSess(s, s.ExtValues)
		case "coverage":
			return CoverageOutputInitSess(s, s.ExtValues, s.CoverageTests, s.CoverageSize)
		default:
			sessNoteError(s, ErrGeneric)
			return ""
		}
	}
	var b strings.Builder
	if acceptArgc {
		b.WriteString("int main (int argc, char* argv[])\n")
	} else {
		b.WriteString("int main (void)\n")
	}
	b.WriteString("{\n")
	return b.String()
}

// ExtensionMgrOutputFirstFunInvocation mirrors OutputFirstFunInvocation.
// ExtensionMgr.cpp:131–141 — null path same as AbsExtension default.
func ExtensionMgrOutputFirstFunInvocation(invokeOut string) string {
	return ExtensionMgrOutputFirstFunInvocationSess(nil, invokeOut)
}

// ExtensionMgrOutputFirstFunInvocationSess is OutputFirstFunInvocation on bag s.
func ExtensionMgrOutputFirstFunInvocationSess(s *Session, invokeOut string) string {
	s = sessOrAmbient(s)
	if s.ExtensionActive {
		switch s.ExtKind {
		case "klee", "crest":
			return AbsExtensionOutputFirstFunInvocation(invokeOut)
		case "coverage":
			return CoverageOutputFirstFunInvocationSess(s, s.ExtValues, invokeOut, s.CoverageSize)
		default:
			sessNoteError(s, ErrGeneric)
			return ""
		}
	}
	return AbsExtensionOutputFirstFunInvocation(invokeOut)
}

// ExtensionValues returns active extension values_ (may be nil).
func ExtensionValues() []*ExtensionValue { return ExtensionValuesSess(nil) }

// ExtensionValuesSess returns ExtValues on an explicit session bag.
func ExtensionValuesSess(s *Session) []*ExtensionValue { return sessOrAmbient(s).ExtValues }

// ExtensionKind returns "klee"|"crest"|"coverage"|"" .
func ExtensionKind() string { return ExtensionKindSess(nil) }

// ExtensionKindSess returns ExtKind on an explicit session bag.
func ExtensionKindSess(s *Session) string { return sessOrAmbient(s).ExtKind }
