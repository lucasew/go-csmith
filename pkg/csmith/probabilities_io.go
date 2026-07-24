// Upstream: Probabilities.cpp dump/parse + DestroyInstance + get_pname/get_sname.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Single-elem separator (SingleProbElem::single_elem_sep_char).
const singleElemSep = '='

// Group delimiters (GroupProbElem).
const (
	groupOpenDelim  = '['
	groupCloseDelim = ']'
	equalOpenDelim  = '('
	equalCloseDelim = ')'
	groupSepChar    = ','
	commentPrefix   = '#'
)

// probSName maps ProbName → configuration string (set_single_name_maps).
var probSName = map[ProbName]string{
	PMoreStructUnionProb:            "more_struct_union_type_prob",
	PBitFieldsCreationProb:          "bitfields_creation_prob",
	PBitFieldInNormalStructProb:     "bitfield_in_normal_struct_prob",
	PScalarFieldInFullBitFieldsProb: "scalar_field_in_full_bitfields_struct_prob",
	PExhaustiveBitFieldsProb:        "exhaustive_bitfield_prob",
	PBitFieldsSignedProb:            "bitfields_signed_prob",
	PSafeOpsSignedProb:              "safe_ops_signed_prob",
	PSelectDerefPointerProb:         "select_deref_pointer_prob",
	PRegularVolatileProb:            "regular_volatile_prob",
	PRegularConstProb:               "regular_const_prob",
	PStricterConstProb:              "stricter_const_prob",
	PLooserConstProb:                "looser_const_prob",
	PFieldVolatileProb:              "field_volatile_prob",
	PFieldConstProb:                 "field_const_prob",
	PStdUnaryFuncProb:               "std_unary_func_prob",
	PShiftByNonConstantProb:         "shift_by_non_constant_prob",
	PPointerAsLTypeProb:             "pointer_as_ltype_prob",
	PStructAsLTypeProb:              "struct_as_ltype_prob",
	PUnionAsLTypeProb:               "union_as_ltype_prob",
	PFloatAsLTypeProb:               "float_as_ltype_prob",
	PNewArrayVariableProb:           "new_array_var_prob",
	PAccessOnceVariableProb:         "access_once_var_prob",
	PInlineFunctionProb:             "inline_function_prob",
	PBuiltinFunctionProb:            "builtin_function_prob",
	PArrayOOBProb:                   "array_oob_prob",
	PFuncAttrProb:                   "func_attr_flag",
	PTypeAttrProb:                   "type_attr_flag",
	PLabelAttrProb:                  "label_attr_flag",
	PVarAttrProb:                    "var_attr_flag",
	PBinaryConstProb:                "binary_constant",
	PStatementProb:                  "statement_prob",
	PAssignOpsProb:                  "assign_ops_prob",
	PUnaryOpsProb:                   "assign_unary_ops_prob",
	PBinaryOpsProb:                  "assign_binary_ops_prob",
	PSimpleTypesProb:                "simple_types_prob",
	PSafeOpsSizeProb:                "safe_ops_size_prob",
}

// reverse map built once
var snameToPname map[string]ProbName

func init() {
	snameToPname = make(map[string]ProbName, len(probSName))
	for p, s := range probSName {
		snameToPname[s] = p
	}
}

// GetSName mirrors Probabilities::get_sname.
// Incomplete/unknown pname sticky "" (no invent empty token success).
func GetSName(pname ProbName) string {
	s, ok := probSName[pname]
	if !ok {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	return s
}

// GetPName mirrors Probabilities::get_pname.
// Unknown sname sticky Max-like fail closed (returns -1).
func GetPName(sname string) (ProbName, bool) {
	p, ok := snameToPname[sname]
	if !ok {
		sessNoteError(nil, ErrGeneric)
		return 0, false
	}
	return p, true
}

// DestroyProcessProbabilities mirrors Probabilities::DestroyInstance.
// Clears process singleton handle.
func DestroyProcessProbabilities() {
	SetProcessProbabilities(nil)
}

// DumpSingleDefault mirrors SingleProbElem::dump_default — "sname=val".
func DumpSingleDefault(sname string, defaultVal int) string {
	return fmt.Sprintf("%s%c%d", sname, singleElemSep, defaultVal)
}

// DumpSingleVal mirrors SingleProbElem::dump_val.
func DumpSingleVal(sname string, val int) string {
	return DumpSingleDefault(sname, val)
}

// DumpDefaultProbabilities mirrors Probabilities::dump_default_probabilities content.
// Fair subset: all known single probs from this *Probabilities.
// Empty fname sticky (no invent stdout dump as success file write).
func (p *Probabilities) DumpDefaultProbabilities() string {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	// Defaults from a fresh Defaults() table for "default" dump semantics
	def := NewProbabilities(Defaults())
	return def.dumpSingles(false)
}

// DumpActualProbabilities mirrors dump_actual_probabilities content (with seed header).
func (p *Probabilities) DumpActualProbabilities(seed uint64) string {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Seed: %d\n\n", seed))
	b.WriteString(p.dumpSingles(true))
	return b.String()
}

func (p *Probabilities) dumpSingles(actual bool) string {
	_ = actual
	var b strings.Builder
	// Stable order by sname for deterministic dump
	names := make([]string, 0, len(probSName))
	for _, s := range probSName {
		// only singles that live in p.single
		names = append(names, s)
	}
	// sort
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for _, s := range names {
		pn, ok := snameToPname[s]
		if !ok {
			continue
		}
		// skip pure group roots without single entry
		if _, isSingle := p.single[pn]; !isSingle {
			continue
		}
		val := p.Single(pn)
		b.WriteString(DumpSingleVal(s, val))
		b.WriteString("\n\n")
	}
	return b.String()
}

// WriteDumpDefaultProbabilities writes dump to file (C++ ofstream path).
func WriteDumpDefaultProbabilities(fname string) error {
	if fname == "" {
		sessNoteError(nil, ErrGeneric)
		return fmt.Errorf("empty dump path")
	}
	p := ProcessProbabilities()
	if p == nil {
		p = NewProbabilities(Defaults())
	}
	return os.WriteFile(fname, []byte(p.DumpDefaultProbabilities()), 0o644)
}

// WriteDumpActualProbabilities writes actual vals + seed header.
func WriteDumpActualProbabilities(fname string, seed uint64) error {
	if fname == "" {
		sessNoteError(nil, ErrGeneric)
		return fmt.Errorf("empty dump path")
	}
	p := ProcessProbabilities()
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return fmt.Errorf("probabilities not initialized")
	}
	return os.WriteFile(fname, []byte(p.DumpActualProbabilities(seed)), 0o644)
}

// ParseConfiguration mirrors Probabilities::parse_configuration.
// Supports single-line "sname=val" and # comments for the fair single-prob subset.
// Group lines for equal/unequal groups fail closed with error_msg (no invent skip).
func (p *Probabilities) ParseConfiguration(fname string) (errMsg string, ok bool) {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return "nil probabilities", false
	}
	if fname == "" {
		return "fail to open probabilities configuration file!", false
	}
	f, err := os.Open(fname)
	if err != nil {
		return "fail to open probabilities configuration file!", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if msg, ok := p.ParseLine(line); !ok {
			return msg, false
		}
	}
	if err := sc.Err(); err != nil {
		return err.Error(), false
	}
	return "", true
}

// ParseLine mirrors Probabilities::parse_line.
func (p *Probabilities) ParseLine(line string) (errMsg string, ok bool) {
	if p == nil {
		sessNoteError(nil, ErrGeneric)
		return "nil probabilities", false
	}
	trim := strings.TrimSpace(line)
	if trim == "" {
		return "parse empty line", false
	}
	c := trim[0]
	switch c {
	case commentPrefix:
		return "", true
	case groupOpenDelim, equalOpenDelim:
		// Group probability configuration not fully wired on fair spine tables.
		return "group probabilities not supported in fair parse subset", false
	default:
		return p.parseSingleProbability(trim)
	}
}

// parseSingleProbability mirrors parse_single_probability.
func (p *Probabilities) parseSingleProbability(line string) (string, bool) {
	parts := strings.SplitN(line, string(singleElemSep), 2)
	if len(parts) != 2 {
		return "invalid single probability format", false
	}
	sname := strings.TrimSpace(parts[0])
	valStr := strings.TrimSpace(parts[1])
	val, err := strconv.Atoi(valStr)
	if err != nil || val < 0 || val > 100 {
		return "invalid probability value", false
	}
	pn, ok := GetPName(sname)
	if !ok {
		ClearError() // GetPName stickied; surface as config error
		return "invalid string in the configuration file: " + sname, false
	}
	if p.single == nil {
		p.single = make(map[ProbName]int)
	}
	// only update known singles
	if _, exists := p.single[pn]; !exists {
		return "probability not in single table: " + sname, false
	}
	p.single[pn] = val
	return "", true
}

// ParseGroupProbabilities mirrors parse_group_probabilities (equal/unequal).
// Fair spine: returns clear error (group tables are weight slices, not name maps).
func (p *Probabilities) ParseGroupProbabilities(isEqual bool, line string) (string, bool) {
	_ = p
	_ = isEqual
	_ = line
	return "group probabilities not supported in fair parse subset", false
}

// ParseSingleElem mirrors parse_single_elem — "sname=val" → val.
func ParseSingleElem(line string) (sname string, val int, ok bool) {
	parts := strings.SplitN(line, string(singleElemSep), 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	sname = strings.TrimSpace(parts[0])
	v, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", 0, false
	}
	return sname, v, true
}
