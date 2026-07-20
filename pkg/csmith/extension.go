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
			IsConsts:     []bool{false},
			IsVolatiles:  []bool{false},
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
var processExtensionActive bool // true only when a non-null extension was created

// CreateExtension mirrors ExtensionMgr::CreateExtension.
// ExtensionMgr.cpp:48–64 — klee/crest/coverage_test construct; else leave null.
// Non-default extensions not ported: sticky fail closed (no silent no-op invent).
func CreateExtension(opts Options) {
	processExtensionActive = false
	if opts.Klee || opts.Crest || opts.CoverageTest {
		// KleeExtension / CrestExtension / CoverageTestExtension not on fair spine yet
		SetError(ErrGeneric)
		return
	}
	// null extension (default) — no-op success
}

// DestroyExtension mirrors ExtensionMgr::DestroyExtension.
// ExtensionMgr.cpp:66–69.
func DestroyExtension() {
	processExtensionActive = false
}

// ExtensionActive is true when a non-null AbsExtension is installed.
func ExtensionActive() bool { return processExtensionActive }

// ExtensionMgrGenerateValues mirrors ExtensionMgr::GenerateValues.
// ExtensionMgr.cpp:84–88 — null → no-op.
func ExtensionMgrGenerateValues() {
	if !processExtensionActive {
		return
	}
	// non-null not ported; sticky if ever active without impl
	SetError(ErrGeneric)
}

// ExtensionMgrGenerateFirstParameterList mirrors GenerateFirstParameterList.
// ExtensionMgr.cpp:77–82 — null → no-op.
func ExtensionMgrGenerateFirstParameterList(f *Function, vs *VariableSelector) {
	if !processExtensionActive {
		return
	}
	_ = f
	_ = vs
	SetError(ErrGeneric)
}

// ExtensionMgrOutputHeader mirrors OutputHeader — null → empty.
// ExtensionMgr.cpp:101–107.
func ExtensionMgrOutputHeader() string {
	if !processExtensionActive {
		return ""
	}
	SetError(ErrGeneric)
	return ""
}

// ExtensionMgrOutputTail mirrors OutputTail.
// ExtensionMgr.cpp:109–115 — null → "    return 0;\n".
func ExtensionMgrOutputTail() string {
	if !processExtensionActive {
		return AbsExtensionTab + "return 0;\n"
	}
	SetError(ErrGeneric)
	return ""
}

// ExtensionMgrOutputInit mirrors OutputInit.
// ExtensionMgr.cpp:117–129 — null → main signature + "{".
func ExtensionMgrOutputInit(acceptArgc bool) string {
	if processExtensionActive {
		SetError(ErrGeneric)
		return ""
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
	if processExtensionActive {
		SetError(ErrGeneric)
		return ""
	}
	return AbsExtensionOutputFirstFunInvocation(invokeOut)
}
