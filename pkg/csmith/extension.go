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
		SetError(ErrGeneric)
		return nil
	}
	// empty name sticky (no invent base_name_only)
	if name == "" {
		SetError(ErrGeneric)
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
	if num < 0 {
		SetError(ErrGeneric)
		return nil
	}
	if num == 0 {
		return nil
	}
	// Rng + Probabilities always live for choose_random_simple
	if r == nil || probs == nil {
		SetError(ErrGeneric)
		return nil
	}
	values := make([]*ExtensionValue, 0, num)
	for i := 0; i < num; i++ {
		st := ChooseRandomNonvoidSimple(r, probs)
		if HasError() {
			return nil
		}
		typ := GetSimpleType(st)
		if typ == nil {
			SetError(ErrGeneric)
			return nil
		}
		ev := NewExtensionValue(typ, fmt.Sprintf("%s%d", AbsExtensionBaseName, i))
		if ev == nil || HasError() {
			return nil
		}
		values = append(values, ev)
	}
	return values
}

// AbsExtensionDefaultOutputDefinitions mirrors default_output_definitions.
// AbsExtension.cpp:93–105 — tab + type + name [ = 0]; per value.
// Incomplete values sticky "" (no invent partial definitions section).
func AbsExtensionDefaultOutputDefinitions(values []*ExtensionValue, initFlag bool) string {
	if !extensionValuesComplete(values) {
		SetError(ErrGeneric)
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
		if HasError() || cn == "" {
			if !HasError() {
				SetError(ErrGeneric)
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
// Incomplete invoke string sticky "" (no invent bare ";" call).
func AbsExtensionOutputFirstFunInvocation(invokeOut string) string {
	if invokeOut == "" {
		SetError(ErrGeneric)
		return ""
	}
	return AbsExtensionTab + invokeOut + ";\n"
}

// AbsExtensionGenerateFirstParameterList mirrors GenerateFirstParameterList.
// AbsExtension.cpp:81–90 — GenerateParameterVariable per value into func.Params.
// Incomplete func/values/VS sticky false.
func AbsExtensionGenerateFirstParameterList(f *Function, values []*ExtensionValue, vs *VariableSelector) bool {
	if f == nil || vs == nil {
		SetError(ErrGeneric)
		return false
	}
	if !extensionValuesComplete(values) {
		SetError(ErrGeneric)
		return false
	}
	for _, ev := range values {
		q := ev.Qfer
		v := vs.GenerateParameterVariableTyped(ev.Type, q)
		if v == nil || HasError() {
			if !HasError() {
				SetError(ErrGeneric)
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
	if f == nil {
		SetError(ErrGeneric)
		return nil
	}
	if !extensionValuesComplete(values) {
		SetError(ErrGeneric)
		return nil
	}
	// Build a FunctionInvocationUser with fixed ExpressionVariable params.
	fi := &Invocation{User: f}
	for _, ev := range values {
		// VariableSelector::new_variable(name, type, null init, qfer)
		v := CreateVariableScalars(ev.Name, ev.Type, false, false)
		if v == nil {
			SetError(ErrGeneric)
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
	if !opts.Klee && !opts.Crest && !opts.CoverageTest {
		currentSession().ExtensionActive = false
		currentSession().ExtKind = ""
		currentSession().ExtValues = nil
		currentSession().CoverageTests = nil
		return
	}
	r := ProcessRng()
	probs := ProcessProbabilities()
	if r == nil || probs == nil {
		// CreateInstance may not have run yet — library paths pass via CreateExtensionFull
		SetError(ErrGeneric)
		return
	}
	CreateExtensionFull(opts, r, probs)
}

// DestroyExtension mirrors ExtensionMgr::DestroyExtension.
// ExtensionMgr.cpp:66–69.
func DestroyExtension() {
	currentSession().ExtensionActive = false
	currentSession().ExtKind = ""
	currentSession().ExtValues = nil
	currentSession().CoverageTests = nil
	currentSession().CoverageSize = 0
}

// ExtensionActive is true when a non-null AbsExtension is installed.
func ExtensionActive() bool { return currentSession().ExtensionActive }

// ExtensionMgrGenerateValues mirrors ExtensionMgr::GenerateValues.
// ExtensionMgr.cpp:84–88 — null → no-op; Klee/Crest empty; Coverage already built.
func ExtensionMgrGenerateValues() {
	if !currentSession().ExtensionActive {
		return
	}
	// Klee/Crest GenerateValues are empty; Coverage fills at Create
}

// ExtensionMgrGenerateFirstParameterList mirrors GenerateFirstParameterList.
// ExtensionMgr.cpp:77–82 — null → no-op.
func ExtensionMgrGenerateFirstParameterList(f *Function, vs *VariableSelector) {
	if !currentSession().ExtensionActive {
		return
	}
	if !AbsExtensionGenerateFirstParameterList(f, currentSession().ExtValues, vs) {
		if !HasError() {
			SetError(ErrGeneric)
		}
	}
}

// ExtensionMgrOutputHeader mirrors OutputHeader — null → empty.
// ExtensionMgr.cpp:101–107.
func ExtensionMgrOutputHeader() string {
	if !currentSession().ExtensionActive {
		return ""
	}
	switch currentSession().ExtKind {
	case "klee":
		return KleeOutputHeader()
	case "crest":
		return CrestOutputHeader()
	case "coverage":
		return "" // CoverageTestExtension::OutputHeader empty
	default:
		SetError(ErrGeneric)
		return ""
	}
}

// ExtensionMgrOutputTail mirrors OutputTail.
// ExtensionMgr.cpp:109–115 — null → "    return 0;\n".
func ExtensionMgrOutputTail() string {
	if !currentSession().ExtensionActive {
		return AbsExtensionTab + "return 0;\n"
	}
	switch currentSession().ExtKind {
	case "klee":
		return KleeOutputTail()
	case "crest":
		return CrestOutputTail()
	case "coverage":
		return CoverageOutputTail()
	default:
		SetError(ErrGeneric)
		return ""
	}
}

// ExtensionMgrOutputInit mirrors OutputInit.
// ExtensionMgr.cpp:117–129 — null → main signature + "{".
func ExtensionMgrOutputInit(acceptArgc bool) string {
	if currentSession().ExtensionActive {
		switch currentSession().ExtKind {
		case "klee":
			return KleeOutputInit(currentSession().ExtValues)
		case "crest":
			return CrestOutputInit(currentSession().ExtValues)
		case "coverage":
			return CoverageOutputInit(currentSession().ExtValues, currentSession().CoverageTests, currentSession().CoverageSize)
		default:
			SetError(ErrGeneric)
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
	if currentSession().ExtensionActive {
		switch currentSession().ExtKind {
		case "klee", "crest":
			return AbsExtensionOutputFirstFunInvocation(invokeOut)
		case "coverage":
			return CoverageOutputFirstFunInvocation(currentSession().ExtValues, invokeOut, currentSession().CoverageSize)
		default:
			SetError(ErrGeneric)
			return ""
		}
	}
	return AbsExtensionOutputFirstFunInvocation(invokeOut)
}

// ExtensionValues returns active extension values_ (may be nil).
func ExtensionValues() []*ExtensionValue { return currentSession().ExtValues }

// ExtensionKind returns "klee"|"crest"|"coverage"|"" .
func ExtensionKind() string { return currentSession().ExtKind }
