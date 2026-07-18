// Upstream: Attribute.h / Attribute.cpp (AttributeGenerator, BooleanAttribute).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import "strings"

// Attribute is the base for __attribute__((...)) generators.
// Attribute.cpp: Attribute / BooleanAttribute / MultiChoiceAttribute.
type Attribute interface {
	// MakeRandom returns attribute text or "" if not selected.
	MakeRandom(r *Rng) string
}

// BooleanAttribute mirrors BooleanAttribute — emit name with probability.
// Attribute.cpp:41–48.
type BooleanAttribute struct {
	Name string
	Prob int // 0–100 flipcoin
}

// MakeRandom implements Attribute.
func (a *BooleanAttribute) MakeRandom(r *Rng) string {
	if a == nil || r == nil {
		return ""
	}
	p := a.Prob
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	if r.RndFlipcoin(uint32(p)) {
		return a.Name
	}
	return ""
}

// MultiChoiceAttribute mirrors MultiChoiceAttribute — name(choices[i]).
// Attribute.cpp:50–70 subset.
type MultiChoiceAttribute struct {
	Name    string
	Prob    int
	Choices []string
}

// MakeRandom implements Attribute.
func (a *MultiChoiceAttribute) MakeRandom(r *Rng) string {
	if a == nil || r == nil || len(a.Choices) == 0 {
		return ""
	}
	p := a.Prob
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	if !r.RndFlipcoin(uint32(p)) {
		return ""
	}
	i := int(r.RndUpto(uint32(len(a.Choices))))
	return a.Name + "(" + a.Choices[i] + ")"
}

// AttributeGenerator mirrors AttributeGenerator — list of attributes.
// Attribute.cpp:18–33 Output.
type AttributeGenerator struct {
	Attributes []Attribute
}

// Output mirrors AttributeGenerator::Output — " __attribute__((a, b))" or "".
func (g *AttributeGenerator) Output(r *Rng) string {
	if g == nil || len(g.Attributes) == 0 {
		return ""
	}
	var parts []string
	for _, a := range g.Attributes {
		if a == nil {
			continue
		}
		s := a.MakeRandom(r)
		if s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " __attribute__((" + strings.Join(parts, ", ") + "))"
}

// NewVarAttrGenerator mirrors Variable::InitializeVariableAttributes default set.
// Variable.cpp:83–99 — common/uncommon/deprecated/unused/used + aligned multi.
// Probabilities left as 10 (VarAttrProb-ish) until CGOptions::vars_attr_prob lands.
func NewVarAttrGenerator() *AttributeGenerator {
	p := 10
	return &AttributeGenerator{Attributes: []Attribute{
		&MultiChoiceAttribute{Name: "aligned", Prob: p, Choices: []string{"8", "16", "32"}},
		&BooleanAttribute{Name: "common", Prob: p},
		&BooleanAttribute{Name: "uncommon", Prob: p},
		&BooleanAttribute{Name: "deprecated", Prob: p},
		&BooleanAttribute{Name: "unused", Prob: p},
		&BooleanAttribute{Name: "used", Prob: p},
	}}
}
