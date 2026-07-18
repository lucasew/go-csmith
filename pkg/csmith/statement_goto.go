// Upstream: StatementGoto.cpp (make_random) with back/forward edge selection.
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// goodGotoTarget reports statements that may receive a label (not jump/return-ish).
// StatementGoto.cpp:99–109 — disallow break/continue/goto/return as targets.
func goodGotoTarget(st Stmt) bool {
	switch st.Kind {
	case StmtReturn, StmtBreak, StmtContinue, StmtGoto:
		return false
	default:
		return true
	}
}

// MakeRandomGoto mirrors StatementGoto::make_random.
// StatementGoto.cpp:61+ — 40% back-edge; choose_visible_read_var for cond; cfg_edge.
func MakeRandomGoto(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	cg CGContext,
	blk *Block,
) Stmt {
	_ = opts
	_ = probs
	if r == nil || cg.CurrentFunc == nil {
		return Stmt{Kind: StmtGoto}
	}

	// condition: prefer already-read visible var (StatementGoto.cpp:118–130)
	var cond *Expression
	if cg.EffectAccum != nil {
		var uf []*FactUnion
		if cg.FM != nil {
			uf = cg.FM.UnionFacts
		}
		if v := ChooseVisibleReadVar(r, blk, cg.EffectAccum.ReadVars(), GetIntType(), uf); v != nil {
			cg.NoteRead(v)
			cond = &Expression{Term: TermVariable, Var: v, ExprType: GetIntType()}
		}
	}
	if cond == nil {
		cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
		if cond == nil {
			cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
		}
	}

	label := "lbl_1"
	if vs != nil {
		label = vs.Sym.Next("lbl_")
	}

	st := Stmt{Kind: StmtGoto, Expr: cond}
	if st.StmID == 0 {
		st.StmID = AllocStmID()
	}

	// 40% prefer back-edge (StatementGoto.cpp:76)
	wantBack := r.RndFlipcoin(40)
	if wantBack {
		type cand struct {
			blk *Block
			idx int
		}
		var cands []cand
		f := cg.CurrentFunc
		for _, b := range f.Blocks {
			if b == nil {
				continue
			}
			for i, s := range b.Stmts {
				if goodGotoTarget(s) && !s.MustReturn() {
					cands = append(cands, cand{b, i})
				}
			}
		}
		if blk != nil {
			for i, s := range blk.Stmts {
				if goodGotoTarget(s) && !s.MustReturn() {
					cands = append(cands, cand{blk, i})
				}
			}
		}
		if len(cands) > 0 {
			c := cands[r.RndUpto(uint32(len(cands)))]
			tgt := &c.blk.Stmts[c.idx]
			if tgt.SourceLabel == "" {
				tgt.SourceLabel = label
			} else {
				label = tgt.SourceLabel
			}
			if tgt.StmID == 0 {
				tgt.StmID = AllocStmID()
			}
			st.Label = label
			st.GotoBack = true
			// StatementGoto.cpp:139 — create_cfg_edge(sg, other_stm, false, true)
			if cg.FM != nil {
				cg.FM.CreateCFGEdgeTo(st.StmID, c.blk, tgt.StmID, false, true)
			}
			return st
		}
	}

	// forward edge: label after goto in MakeRandomBlock (StatementGoto.cpp:203 false back)
	st.Label = label
	st.GotoForward = true
	if cg.FM != nil && blk != nil {
		// dest is future label stmt in same block — DestStmID 0 until placed
		cg.FM.CreateCFGEdgeTo(st.StmID, blk, 0, false, false)
	}
	return st
}
