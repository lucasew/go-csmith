package csmith

// seedTypesForTest mirrors Type::GenerateAllTypes before Function factories.
// Production always runs GenerateAllTypesEnv first; tests must too (no soft invent return type).
func seedTypesForTest(r *Rng, opts Options, probs *Probabilities, vs *VariableSelector, list *FunctionList) {
	if r == nil {
		r = NewRngSess(testAmbientSession, 1)
	}
	if probs == nil {
		probs = NewProbabilities(opts)
	}
	var env *TypeEnv
	if list != nil && list.Types != nil {
		env = list.Types
	} else if vs != nil && vs.Types != nil {
		env = vs.Types
	} else {
		env = &TypeEnv{Sess: testAmbientSession}
	}
	if len(env.AllTypes) == 0 {
		GenerateAllTypesEnv(r, opts, probs, env)
	}
	if vs != nil {
		vs.Types = env
	}
	if list != nil {
		list.Types = env
	}
}
