// Upstream: KleeExtension.cpp / CrestExtension.cpp / CoverageTestExtension.cpp
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// extensionValues holds AbsExtension values_ for the active non-null extension.

// --- KleeExtension ---

// KleeInputBaseName mirrors KleeExtension::input_base_name_ ("input").
const KleeInputBaseName = "input"

// KleeOutputHeader mirrors KleeExtension::OutputHeader.
func KleeOutputHeader() string {
	return "#include \"klee/klee.h\"\n"
}

// KleeOutputSymbolics mirrors KleeExtension::output_symbolics.
func KleeOutputSymbolics(values []*ExtensionValue) string {
	if !extensionValuesComplete(values) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	for count, value := range values {
		b.WriteString(AbsExtensionTab)
		b.WriteString("klee_make_symbolic(&")
		b.WriteString(value.Name)
		b.WriteString(", sizeof(")
		b.WriteString(value.Name)
		b.WriteString("), \"")
		b.WriteString(fmt.Sprintf("%s%d", KleeInputBaseName, count))
		b.WriteString("\");\n")
	}
	return b.String()
}

// KleeOutputInit mirrors KleeExtension::OutputInit.
func KleeOutputInit(values []*ExtensionValue) string {
	var b strings.Builder
	b.WriteString("int main(void)\n{\n")
	b.WriteString(AbsExtensionDefaultOutputDefinitions(values, false))
	if sessHasError(nil) {
		return ""
	}
	b.WriteString(KleeOutputSymbolics(values))
	return b.String()
}

// KleeOutputTail mirrors KleeExtension::OutputTail.
func KleeOutputTail() string {
	return AbsExtensionTab + "return 0;\n"
}

// --- CrestExtension ---

// CrestInputBaseName mirrors CrestExtension::input_base_name_ ("CREST_").
const CrestInputBaseName = "CREST_"

// CrestTypeToString mirrors CrestExtension::type_to_string.
// CrestExtension.cpp:52–78 — simple types only; sticky "" on non-simple.
func CrestTypeToString(t *Type) string {
	if t == nil || !t.IsSimple() {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	switch t.Simple() {
	case EChar:
		return "char"
	case EUChar:
		return "unsigned_char"
	case EShort:
		return "short"
	case EUShort:
		return "unsigned_short"
	case EInt:
		return "int"
	case EUInt:
		return "unsigned_int"
	case ELong:
		return "int"
	case EULong:
		return "unsigned_int"
	default:
		sessNoteError(nil, ErrGeneric)
		return ""
	}
}

// CrestOutputSymbolics mirrors CrestExtension::output_symbolics.
func CrestOutputSymbolics(values []*ExtensionValue) string {
	if !extensionValuesComplete(values) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	for _, value := range values {
		ts := CrestTypeToString(value.Type)
		if sessHasError(nil) || ts == "" {
			return ""
		}
		b.WriteString(AbsExtensionTab)
		b.WriteString(CrestInputBaseName)
		b.WriteString(ts)
		b.WriteString("(")
		b.WriteString(value.Name)
		b.WriteString(");\n")
	}
	return b.String()
}

// CrestOutputInit mirrors CrestExtension::OutputInit.
func CrestOutputInit(values []*ExtensionValue) string {
	var b strings.Builder
	b.WriteString("int main(void)\n{\n")
	b.WriteString(AbsExtensionDefaultOutputDefinitions(values, false))
	if sessHasError(nil) {
		return ""
	}
	b.WriteString(CrestOutputSymbolics(values))
	return b.String()
}

// CrestOutputHeader mirrors CrestExtension::OutputHeader.
func CrestOutputHeader() string {
	return "#include \"crest.h\"\n"
}

// CrestOutputTail mirrors CrestExtension::OutputTail.
func CrestOutputTail() string {
	return AbsExtensionTab + "return 0;\n"
}

// --- CoverageTestExtension ---

// CoverageArrayBaseName / CoverageArrayIndex mirrors CoverageTestExtension statics.
const (
	CoverageArrayBaseName = "a"
	CoverageArrayIndex    = "test_index"
)

// CoverageGenerateValues mirrors CoverageTestExtension::GenerateValues.
// CoverageTestExtension.cpp:52–61 — make_random per value × inputs_size.
func CoverageGenerateValues(values []*ExtensionValue, inputsSize int, r *Rng, opts Options, probs *Probabilities) []*Constant {
	if inputsSize <= 0 || r == nil || probs == nil {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	if !extensionValuesComplete(values) {
		sessNoteError(nil, ErrGeneric)
		return nil
	}
	var out []*Constant
	for _, value := range values {
		for j := 0; j < inputsSize; j++ {
			c := MakeRandom(value.Type, opts, probs, r)
			if c == nil || sessHasError(nil) {
				return nil
			}
			out = append(out, c)
		}
	}
	return out
}

// CoverageOutputArrayInit mirrors output_array_init for one value's row.
// count is the value index; tests layout is [v0_t0, v0_t1, ..., v1_t0, ...].
func CoverageOutputArrayInit(tests []*Constant, count, inputsSize int) string {
	if inputsSize <= 0 || count < 0 {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	base := count * inputsSize
	if base+inputsSize > len(tests) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	if inputsSize == 1 {
		return tests[base].Output()
	}
	var b strings.Builder
	lenN := 0
	last := base + inputsSize - 1
	for i := base; i < last; i++ {
		if lenN%10 == 0 {
			b.WriteString("\n")
			b.WriteString(AbsExtensionTab)
			b.WriteString(AbsExtensionTab)
		}
		o := tests[i].Output()
		if sessHasError(nil) || o == "" {
			return ""
		}
		b.WriteString(o)
		b.WriteString(", ")
		lenN++
	}
	if lenN%10 == 0 {
		b.WriteString("\n")
		b.WriteString(AbsExtensionTab)
		b.WriteString(AbsExtensionTab)
	}
	o := tests[last].Output()
	if sessHasError(nil) || o == "" {
		return ""
	}
	b.WriteString(o)
	return b.String()
}

// CoverageOutputDecls mirrors CoverageTestExtension::output_decls.
func CoverageOutputDecls(values []*ExtensionValue, tests []*Constant, inputsSize int) string {
	if !extensionValuesComplete(values) {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(AbsExtensionDefaultOutputDefinitions(values, false))
	if sessHasError(nil) {
		return ""
	}
	for count, value := range values {
		b.WriteString(AbsExtensionTab)
		cn := value.Type.CName()
		if sessHasError(nil) || cn == "" {
			if !sessHasError(nil) {
				sessNoteError(nil, ErrGeneric)
			}
			return ""
		}
		b.WriteString(cn)
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("%s%d", CoverageArrayBaseName, count))
		b.WriteString(fmt.Sprintf("[%d] = {", inputsSize))
		init := CoverageOutputArrayInit(tests, count, inputsSize)
		if sessHasError(nil) {
			return ""
		}
		b.WriteString(init)
		b.WriteString("};\n")
	}
	b.WriteString(AbsExtensionTab)
	b.WriteString("int ")
	b.WriteString(CoverageArrayIndex)
	b.WriteString(";\n")
	return b.String()
}

// CoverageOutputFirstFunInvocation mirrors OutputFirstFunInvocation.
func CoverageOutputFirstFunInvocation(values []*ExtensionValue, invokeOut string, inputsSize int) string {
	if !extensionValuesComplete(values) || invokeOut == "" || inputsSize <= 0 {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(AbsExtensionTab)
	b.WriteString("for(")
	b.WriteString(CoverageArrayIndex)
	b.WriteString(" = 0; ")
	b.WriteString(CoverageArrayIndex)
	b.WriteString(" < ")
	b.WriteString(fmt.Sprintf("%d", inputsSize))
	b.WriteString("; ")
	b.WriteString(CoverageArrayIndex)
	b.WriteString("++) {\n")
	for count, value := range values {
		b.WriteString(AbsExtensionTab)
		b.WriteString(AbsExtensionTab)
		b.WriteString(value.Name)
		b.WriteString(" = ")
		b.WriteString(fmt.Sprintf("%s%d", CoverageArrayBaseName, count))
		b.WriteString("[")
		b.WriteString(CoverageArrayIndex)
		b.WriteString("];\n")
	}
	b.WriteString(AbsExtensionTab)
	b.WriteString(AbsExtensionTab)
	b.WriteString(invokeOut)
	b.WriteString(";\n")
	b.WriteString(AbsExtensionTab)
	b.WriteString("}\n")
	return b.String()
}

// CoverageOutputInit mirrors CoverageTestExtension::OutputInit.
func CoverageOutputInit(values []*ExtensionValue, tests []*Constant, inputsSize int) string {
	var b strings.Builder
	b.WriteString("int main(void)\n{\n")
	b.WriteString(CoverageOutputDecls(values, tests, inputsSize))
	return b.String()
}

// CoverageOutputTail mirrors CoverageTestExtension::OutputTail.
func CoverageOutputTail() string {
	return AbsExtensionTab + "return 0;\n"
}

// --- CreateExtension wiring ---

// CreateExtensionFull installs Klee/Crest/Coverage when options request them.
// Replaces sticky-only CreateExtension for those flags.
func CreateExtensionFull(opts Options, r *Rng, probs *Probabilities) {
	CreateExtensionFullSess(nil, opts, r, probs)
}

// CreateExtensionFullSess installs Klee/Crest/Coverage on an explicit session bag.
func CreateExtensionFullSess(s *Session, opts Options, r *Rng, probs *Probabilities) {
	s = sessOrAmbient(s)
	s.ExtensionActive = false
	s.ExtValues = nil
	s.ExtKind = ""
	s.CoverageTests = nil
	s.CoverageSize = 0

	if opts.Klee {
		s.ExtKind = "klee"
	} else if opts.Crest {
		s.ExtKind = "crest"
	} else if opts.CoverageTest {
		s.ExtKind = "coverage"
		s.CoverageSize = opts.CoverageTestSize
		if s.CoverageSize <= 0 {
			sessNoteError(s, ErrGeneric)
			return
		}
	} else {
		return // null extension
	}

	// AbsExtension::Initialize(func1_max_params, values)
	s.ExtValues = AbsExtensionInitialize(opts.Func1MaxParams, r, probs)
	if s.ExtValues == nil || sessHasError(s) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		s.ExtKind = ""
		return
	}
	if s.ExtKind == "coverage" {
		s.CoverageTests = CoverageGenerateValues(s.ExtValues, s.CoverageSize, r, opts, probs)
		if s.CoverageTests == nil || sessHasError(s) {
			s.ExtKind = ""
			s.ExtValues = nil
			return
		}
	}
	s.ExtensionActive = true
}
