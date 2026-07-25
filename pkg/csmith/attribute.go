// Upstream: Attribute.h / Attribute.cpp (AttributeGenerator, BooleanAttribute,
// MultiChoiceAttribute, AlignedAttribute, SectionAttribute).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strconv"
	"strings"
)

// Attribute is the base for __attribute__((...)) generators.
// Attribute.cpp: Attribute / BooleanAttribute / MultiChoiceAttribute /
// AlignedAttribute / SectionAttribute.
type Attribute interface {
	// MakeRandom returns attribute text or "" if not selected.
	MakeRandom(r *Rng) string
	// MakeRandomSess is MakeRandom with explicit session residual sticky.
	MakeRandomSess(s *Session, r *Rng) string
}

func clampProb(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// BooleanAttribute mirrors BooleanAttribute — emit name with probability.
// Attribute.cpp:41–48.
type BooleanAttribute struct {
	Name string
	Prob int // 0–100 flipcoin
}

// MakeRandom implements Attribute.
func (a *BooleanAttribute) MakeRandom(r *Rng) string {
	return a.MakeRandomSess(testAmbientSession, r)
}

// MakeRandomSess implements Attribute with explicit session residual sticky.
func (a *BooleanAttribute) MakeRandomSess(s *Session, r *Rng) string {
	// Attribute* always live at MakeRandom; sticky no invent "" (not-selected) past hole
	if a == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute always has process RNG; sticky no invent skip shell without it
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute name from ctor; sticky no invent empty __attribute__ token
	if a.Name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if r.RndFlipcoinSess(s, uint32(clampProb(a.Prob))) {
		return a.Name
	}
	return ""
}

// MultiChoiceAttribute mirrors MultiChoiceAttribute — name("choice").
// Attribute.cpp:50–70 — quoted choice string.
type MultiChoiceAttribute struct {
	Name    string
	Prob    int
	Choices []string
}

// MakeRandom implements Attribute.
func (a *MultiChoiceAttribute) MakeRandom(r *Rng) string {
	return a.MakeRandomSess(testAmbientSession, r)
}

// MakeRandomSess implements Attribute with explicit session residual sticky.
func (a *MultiChoiceAttribute) MakeRandomSess(s *Session, r *Rng) string {
	// Attribute* always live at MakeRandom; sticky no invent "" (not-selected) past hole
	if a == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute always has process RNG + non-empty choices; sticky no invent without them
	if r == nil || len(a.Choices) == 0 {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute name from ctor; sticky no invent ("choice") without name
	if a.Name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !r.RndFlipcoinSess(s, uint32(clampProb(a.Prob))) {
		return ""
	}
	i := int(r.RndUptoSess(s, uint32(len(a.Choices))))
	// Attribute.cpp:66 — name + "(\"" + choice + "\")"; choice always live string
	// sticky no invent name("") for empty choice slot
	if a.Choices[i] == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	return a.Name + "(\"" + a.Choices[i] + "\")"
}

// AlignedAttribute mirrors AlignedAttribute — name(1<<k) for k in [0, alignment).
// Attribute.cpp:72–87.
type AlignedAttribute struct {
	Name      string
	Prob      int
	Alignment int // max exponent exclusive (upstream alignment_factor)
}

// MakeRandom implements Attribute.
func (a *AlignedAttribute) MakeRandom(r *Rng) string {
	return a.MakeRandomSess(testAmbientSession, r)
}

// MakeRandomSess implements Attribute with explicit session residual sticky.
func (a *AlignedAttribute) MakeRandomSess(s *Session, r *Rng) string {
	// Attribute* always live at MakeRandom; sticky no invent "" (not-selected) past hole
	if a == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute always has process RNG; sticky no invent skip shell without it
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute name from ctor; sticky no invent bare "(N)" without name
	if a.Name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !r.RndFlipcoinSess(s, uint32(clampProb(a.Prob))) {
		return ""
	}
	// Attribute.cpp:82–84 — 1 << rnd_upto(alignment); alignment from ctor (no invent 1)
	n := a.Alignment
	if n < 1 {
		// broken Attribute IR sticky — emit nothing (no soft invent alignment=1)
		sessNoteError(s, ErrGeneric)
		return ""
	}
	exp := int(r.RndUptoSess(s, uint32(n)))
	if exp < 0 {
		exp = 0
	}
	if exp > 30 {
		exp = 30
	}
	return a.Name + "(" + strconv.Itoa(1<<uint(exp)) + ")"
}

// SectionAttribute mirrors SectionAttribute — section("usersectionN").
// Attribute.cpp:89–102.
type SectionAttribute struct {
	Name string
	Prob int
}

// MakeRandom implements Attribute.
func (a *SectionAttribute) MakeRandom(r *Rng) string {
	return a.MakeRandomSess(testAmbientSession, r)
}

// MakeRandomSess implements Attribute with explicit session residual sticky.
func (a *SectionAttribute) MakeRandomSess(s *Session, r *Rng) string {
	// Attribute* always live at MakeRandom; sticky no invent "" (not-selected) past hole
	if a == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	// Attribute always has process RNG; sticky no invent skip shell without it
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if !r.RndFlipcoinSess(s, uint32(clampProb(a.Prob))) {
		return ""
	}
	// Attribute.cpp:97–99 — rnd_upto(10); name from ctor sticky (no invent "section")
	if a.Name == "" {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	n := int(r.RndUptoSess(s, 10))
	name := a.Name
	return fmt.Sprintf("%s(\"usersection%d\")", name, n)
}

// AttributeGenerator mirrors AttributeGenerator — list of attributes.
// Attribute.cpp:18–33 Output.
type AttributeGenerator struct {
	Attributes []Attribute
}

// Output mirrors AttributeGenerator::Output — " __attribute__((a, b))" or "".
// AttributeGenerator always live at emit; sticky empty (no invent soft-skip past hole).
// Empty Attributes is complete empty (not incomplete IR).
func (g *AttributeGenerator) Output(r *Rng) string {
	return g.OutputSess(testAmbientSession, r)
}

// OutputSess is Output with explicit session residual sticky.
func (g *AttributeGenerator) OutputSess(s *Session, r *Rng) string {
	if g == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	if len(g.Attributes) == 0 {
		return ""
	}
	// AttributeGenerator always has process RNG when attributes exist; sticky no invent skip
	if r == nil {
		sessNoteError(s, ErrGeneric)
		return ""
	}
	var parts []string
	for _, a := range g.Attributes {
		// Attribute* always live in C++; sticky no invent skip nil holes
		if a == nil {
			sessNoteError(s, ErrGeneric)
			return ""
		}
		part := a.MakeRandomSess(s, r)
		// residual ERROR sticky — no invent soft-continue later attrs past MakeRandom residual
		if sessHasError(s) {
			return ""
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " __attribute__((" + strings.Join(parts, ", ") + "))"
}

// NewVarAttrGenerator mirrors Variable::InitializeVariableAttributes.
// Variable.cpp:83–99 — gated by VariableAttributes; uses VarAttrProb.
func NewVarAttrGenerator(s *Session, opts Options, probs *Probabilities) *AttributeGenerator {
	if !opts.VariableAttributes {
		return &AttributeGenerator{}
	}
	// nil probs → 0% (no invent default 30 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.SingleSess(s, PVarAttrProb)
	}
	return &AttributeGenerator{Attributes: []Attribute{
		&MultiChoiceAttribute{
			Name: "visibility", Prob: p,
			Choices: []string{"default", "hidden", "protected", "internal"},
		},
		&AlignedAttribute{Name: "aligned", Prob: p, Alignment: 8},
		&BooleanAttribute{Name: "common", Prob: p},
		&BooleanAttribute{Name: "uncommon", Prob: p},
		&BooleanAttribute{Name: "deprecated", Prob: p},
		&BooleanAttribute{Name: "unused", Prob: p},
		&BooleanAttribute{Name: "used", Prob: p},
	}}
}

// NewFuncAttrGenerator mirrors Function::InitializeAttributes.
// Function.cpp:82–124 — common booleans + visibility/no_sanitize/optimize + aligned/section.
func NewFuncAttrGenerator(s *Session, opts Options, probs *Probabilities) *AttributeGenerator {
	if !opts.FunctionAttributes {
		return &AttributeGenerator{}
	}
	// nil probs → 0% (no invent default 30 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.SingleSess(s, PFuncAttrProb)
	}
	attrs := make([]Attribute, 0, 40)
	for _, name := range []string{
		"artificial", "flatten", "no_reorder", "hot", "cold", "noipa",
		"used", "unused", "nothrow", "deprecated", "no_icf",
		"no_profile_instrument_function", "noclone", "no_instrument_function",
		"no_sanitize_address", "no_sanitize_thread", "no_sanitize_undefined",
		"no_split_stack", "noinline", "noplt", "stack_protect",
	} {
		attrs = append(attrs, &BooleanAttribute{Name: name, Prob: p})
	}
	attrs = append(attrs,
		&MultiChoiceAttribute{
			Name: "visibility", Prob: p,
			Choices: []string{"default", "hidden", "protected", "internal"},
		},
		&MultiChoiceAttribute{
			Name: "no_sanitize", Prob: p,
			Choices: []string{
				"address", "thread", "undefined", "kernel-address",
				"pointer-compare", "pointer-subtract", "leak",
			},
		},
		&MultiChoiceAttribute{
			Name: "optimize", Prob: p,
			Choices: []string{"-O0", "-O1", "-O2", "-O3", "-Os", "-Ofast", "-Og"},
		},
		&AlignedAttribute{Name: "aligned", Prob: p, Alignment: 16},
		&SectionAttribute{Name: "section", Prob: p},
	)
	return &AttributeGenerator{Attributes: attrs}
}

// NewLabelAttrGenerator mirrors InitializeLabelAttributes.
// Statement.cpp:95–100 — hot/cold with TypeAttrProb when LabelAttributes.
func NewLabelAttrGenerator(s *Session, opts Options, probs *Probabilities) *AttributeGenerator {
	if !opts.LabelAttributes {
		return &AttributeGenerator{}
	}
	// nil probs → 0% (no invent default 50 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.SingleSess(s, PTypeAttrProb)
	}
	return &AttributeGenerator{Attributes: []Attribute{
		&BooleanAttribute{Name: "hot", Prob: p},
		&BooleanAttribute{Name: "cold", Prob: p},
	}}
}

// NewStructTypeAttrGenerator mirrors InitializeTypeAttributes for structs.
// Type.cpp:78–87.
func NewStructTypeAttrGenerator(s *Session, opts Options, probs *Probabilities) *AttributeGenerator {
	if !opts.TypeAttributes {
		return &AttributeGenerator{}
	}
	// nil probs → 0% (no invent default 50 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.SingleSess(s, PTypeAttrProb)
	}
	return &AttributeGenerator{Attributes: []Attribute{
		&AlignedAttribute{Name: "aligned", Prob: p, Alignment: 8},
		&AlignedAttribute{Name: "warn_if_not_aligned", Prob: p, Alignment: 8},
		&BooleanAttribute{Name: "deprecated", Prob: p},
		&BooleanAttribute{Name: "unused", Prob: p},
	}}
}

// NewUnionTypeAttrGenerator mirrors InitializeTypeAttributes for unions.
// Type.cpp:88–97 — + transparent_union.
func NewUnionTypeAttrGenerator(s *Session, opts Options, probs *Probabilities) *AttributeGenerator {
	g := NewStructTypeAttrGenerator(s, opts, probs)
	if !opts.TypeAttributes {
		return g
	}
	// nil probs → 0% (no invent default 50 / NewProbabilities)
	p := 0
	if probs != nil {
		p = probs.SingleSess(s, PTypeAttrProb)
	}
	g.Attributes = append(g.Attributes, &BooleanAttribute{Name: "transparent_union", Prob: p})
	return g
}

// Package-level generators (Variable::var_attr_generator / Function::func_attr_generator).

// InitAttrGeneratorsSess installs attribute generators on an explicit session bag.
// Mirrors InitializeVariableAttributes / InitializeAttributes / InitializeLabelAttributes /
// InitializeTypeAttributes when flags are on.
// Non-Sess InitAttrGenerators deleted — pass testAmbientSession from unit tests.
func InitAttrGeneratorsSess(s *Session, opts Options, probs *Probabilities) {
	s = sessOrAmbient(s)
	s.Opts = opts
	s.Probs = probs
	s.VarAttrGenerator = NewVarAttrGenerator(s, opts, probs)
	s.FuncAttrGenerator = NewFuncAttrGenerator(s, opts, probs)
	s.LabelAttrGenerator = NewLabelAttrGenerator(s, opts, probs)
	s.StructTypeAttrGen = NewStructTypeAttrGenerator(s, opts, probs)
	s.UnionTypeAttrGen = NewUnionTypeAttrGenerator(s, opts, probs)
}

// EnsureVarAttrGeneratorSess returns Variable::var_attr_generator after InitAttrGenerators.
// No soft invent NewVarAttrGenerator with zero opts when init was skipped.
// Non-Sess Ensure* deleted — pass bag explicitly.
func EnsureVarAttrGeneratorSess(s *Session) *AttributeGenerator {
	return sessOrAmbient(s).VarAttrGenerator
}

// EnsureFuncAttrGeneratorSess returns the function attribute generator on bag s.
func EnsureFuncAttrGeneratorSess(s *Session) *AttributeGenerator {
	return sessOrAmbient(s).FuncAttrGenerator
}

// EnsureLabelAttrGeneratorSess returns the label attribute generator on bag s.
func EnsureLabelAttrGeneratorSess(s *Session) *AttributeGenerator {
	return sessOrAmbient(s).LabelAttrGenerator
}

// EnsureStructTypeAttrGeneratorSess returns the struct type attribute generator on bag s.
func EnsureStructTypeAttrGeneratorSess(s *Session) *AttributeGenerator {
	return sessOrAmbient(s).StructTypeAttrGen
}

// EnsureUnionTypeAttrGeneratorSess returns the union type attribute generator on bag s.
func EnsureUnionTypeAttrGeneratorSess(s *Session) *AttributeGenerator {
	return sessOrAmbient(s).UnionTypeAttrGen
}

// ClearAttrGeneratorsSess clears attribute generators on an explicit session bag.
// Non-Sess ClearAttrGenerators deleted — pass bag explicitly.
func ClearAttrGeneratorsSess(s *Session) {
	s = sessOrAmbient(s)
	s.VarAttrGenerator = nil
	s.FuncAttrGenerator = nil
	s.LabelAttrGenerator = nil
	s.StructTypeAttrGen = nil
	s.UnionTypeAttrGen = nil
	s.Probs = nil
}
