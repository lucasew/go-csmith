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
// KleeOutputSymbolicsSess is KleeOutputSymbolics with explicit session residual sticky.
func KleeOutputSymbolicsSess(s *Session, values []*ExtensionValue) string {
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
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
// KleeOutputInitSess is KleeOutputInit with explicit session residual sticky.
func KleeOutputInitSess(s *Session, values []*ExtensionValue) string {
	var b strings.Builder
	b.WriteString("int main(void)\n{\n")
	b.WriteString(AbsExtensionDefaultOutputDefinitionsSess(s, values, false))
	if sessHasError(s) {
		return ""
	}
	b.WriteString(KleeOutputSymbolicsSess(s, values))
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
// CrestExtension.cpp:49–74 — simple types only; default assert(0);return "" is
// NDEBUG no-op → empty suffix (emits CREST_(x) for longlong/float/int128).
// Soft invent sticky-abort on default desynced --crest --uint128 (seed-1 x4).
// CrestTypeToStringSess is CrestTypeToString with explicit session residual sticky.
func CrestTypeToStringSess(s *Session, t *Type) string {
	if t == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !t.IsSimpleSess(s) {
		// residual ERROR sticky — no invent empty suffix past IsSimple residual
		if sessHasError(s) {
			return ""
		}
		// C++ assert(t->eType == eSimple); NDEBUG continues to switch default → ""
		return ""
	}
	// residual ERROR sticky after IsSimple residual true
	if sessHasError(s) {
		return ""
	}
	switch t.SimpleSess(s) {
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
		// CrestExtension.cpp:69–73 — assert(0); break; assert(0); return "";
		// Release golden: empty type token, still emit CREST_(name).
		return ""
	}
}

// CrestOutputSymbolics mirrors CrestExtension::output_symbolics.
// CrestOutputSymbolicsSess is CrestOutputSymbolics with explicit session residual sticky.
func CrestOutputSymbolicsSess(s *Session, values []*ExtensionValue) string {
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	for _, value := range values {
		// value.Type always live (extensionValuesComplete)
		ts := CrestTypeToStringSess(s, value.Type)
		// residual ERROR sticky — no invent soft-skip later symbolics past type residual
		if sessHasError(s) {
			return ""
		}
		// empty ts is complete NDEBUG path (CREST_(x4) for int128 etc.)
		if value.Name == "" {
			sessNoteError(s, ErrGeneric)
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
// CrestOutputInitSess is CrestOutputInit with explicit session residual sticky.
func CrestOutputInitSess(s *Session, values []*ExtensionValue) string {
	var b strings.Builder
	b.WriteString("int main(void)\n{\n")
	b.WriteString(AbsExtensionDefaultOutputDefinitionsSess(s, values, false))
	if sessHasError(s) {
		return ""
	}
	b.WriteString(CrestOutputSymbolicsSess(s, values))
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
// CoverageGenerateValuesSess is CoverageGenerateValues with sticky on run bag.
func CoverageGenerateValuesSess(s *Session, values []*ExtensionValue, inputsSize int, r *Rng, opts Options, probs *Probabilities) []*Constant {
	if inputsSize <= 0 || r == nil || probs == nil {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
		return nil
	}
	var out []*Constant
	for _, value := range values {
		for j := 0; j < inputsSize; j++ {
			c := MakeRandomSess(s, value.Type, opts, probs, r)
			if c == nil || sessHasError(s) {
				return nil
			}
			out = append(out, c)
		}
	}
	return out
}

// CoverageOutputArrayInit mirrors output_array_init for one value's row.
// CoverageTestExtension.cpp:60–81 — sliding window over flat test_values_:
//
//	last_index = inputs_size + count - 1
//	emit test_values_[count .. last_index]  (length inputs_size)
//
// GenerateValues still packs [v0×N][v1×N]…; emit does NOT use base:=count*N
// (that invents non-overlapping rows; golden slides by one each array).
func CoverageOutputArrayInitSess(s *Session, tests []*Constant, count, inputsSize int) string {
	if inputsSize <= 0 || count < 0 {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// CoverageTestExtension.cpp:61–63 — single element at index count
	if inputsSize == 1 {
		if count >= len(tests) {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		return tests[count].OutputOptsSess(s, sessOpts(s))
	}
	// CoverageTestExtension.cpp:66–80 — last_index = inputs_size + count - 1
	last := inputsSize + count - 1
	if last >= len(tests) || count > last {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	lenN := 0
	for i := count; i < last; i++ {
		if lenN%10 == 0 {
			b.WriteString("\n")
			b.WriteString(AbsExtensionTab)
			b.WriteString(AbsExtensionTab)
		}
		o := tests[i].OutputOptsSess(s, sessOpts(s))
		if sessHasError(s) || o == "" {
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
	o := tests[last].OutputOptsSess(s, sessOpts(s))
	if sessHasError(s) || o == "" {
		return ""
	}
	b.WriteString(o)
	return b.String()
}

// CoverageOutputDecls mirrors CoverageTestExtension::output_decls.}

func CoverageOutputDeclsSess(s *Session, values []*ExtensionValue, tests []*Constant, inputsSize int) string {
	if !extensionValuesComplete(values) {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(AbsExtensionDefaultOutputDefinitionsSess(s, values, false))
	if sessHasError(s) {
		return ""
	}
	for count, value := range values {
		b.WriteString(AbsExtensionTab)
		cn := value.Type.CNameSess(s)
		if sessHasError(s) || cn == "" {
			if !sessHasError(s) {
				sessNoteError(s, ErrGeneric)
			}
			return ""
		}
		b.WriteString(cn)
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("%s%d", CoverageArrayBaseName, count))
		b.WriteString(fmt.Sprintf("[%d] = {", inputsSize))
		init := CoverageOutputArrayInitSess(s, tests, count, inputsSize)
		if sessHasError(s) {
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

// CoverageOutputFirstFunInvocation mirrors OutputFirstFunInvocation.}

// CoverageOutputFirstFunInvocationSess is CoverageOutputFirstFunInvocation with explicit session residual sticky.
func CoverageOutputFirstFunInvocationSess(s *Session, values []*ExtensionValue, invokeOut string, inputsSize int) string {
	if !extensionValuesComplete(values) || invokeOut == "" || inputsSize <= 0 {
		sessNoteError(s, ErrGeneric)
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
// CoverageOutputInitSess is CoverageOutputInit with explicit session residual sticky.
func CoverageOutputInitSess(s *Session, values []*ExtensionValue, tests []*Constant, inputsSize int) string {
	var b strings.Builder
	b.WriteString("int main(void)\n{\n")
	b.WriteString(CoverageOutputDeclsSess(s, values, tests, inputsSize))
	return b.String()
}

// CoverageOutputTail mirrors CoverageTestExtension::OutputTail.
func CoverageOutputTail() string {
	return AbsExtensionTab + "return 0;\n"
}

// --- CreateExtension wiring ---

// CreateExtensionFull installs Klee/Crest/Coverage when options request them.
// Replaces sticky-only CreateExtension for those flags.
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

	// AbsExtension::Initialize(func1_max_params, values) only at CreateExtension.
	// CoverageTestExtension::GenerateValues runs later (Function.cpp:809) so
	// Constant::make_random does not burn generation RNG before types/functions.
	s.ExtValues = AbsExtensionInitializeSess(s, opts.Func1MaxParams, r, probs)
	if s.ExtValues == nil || sessHasError(s) {
		if !sessHasError(s) {
			sessNoteError(s, ErrGeneric)
		}
		s.ExtKind = ""
		return
	}
	s.ExtensionActive = true
}
