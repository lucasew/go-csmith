package csmith

func findNamed(s *Session, name string) *Variable {
	if s == nil || s.ProgramGen == nil {
		return nil
	}
	if s.ProgramGen.VS != nil {
		for _, v := range s.ProgramGen.VS.GlobalList {
			if v != nil && v.Name == name {
				return v
			}
		}
	}
	for _, fn := range s.ProgramGen.Funcs.Funcs {
		if fn == nil {
			continue
		}
		if EffectComplete(fn.FEffect) {
			for _, v := range fn.FEffect.ReadVarsSess(s) {
				if v != nil && v.Name == name {
					return v
				}
			}
		}
	}
	return nil
}

func findFn(s *Session, name string) *Function {
	if s == nil || s.ProgramGen == nil {
		return nil
	}
	for _, fn := range s.ProgramGen.Funcs.Funcs {
		if fn != nil && fn.Name == name {
			return fn
		}
	}
	return nil
}
