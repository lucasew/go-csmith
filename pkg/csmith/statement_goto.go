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

// MakeRandomGoto mirrors StatementGoto::make_random (simplified, no FactMgr).
// StatementGoto.cpp:61+ —
//
//	40% prefer back-edge into an earlier good statement (label on target);
//	else forward edge with label placed after the goto in the current block.
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

	// condition: ExpressionVariable-like (no const)
	cond := MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, true, TermVariable, cg.ExprDepth)
	if cond == nil {
		cond = MakeRandomExpression(r, opts, tables, vs, cg, GetIntType(), nil, true, false, TermVariable, cg.ExprDepth)
	}

	label := "lbl_1"
	if vs != nil {
		label = vs.Sym.Next("lbl_")
	}

	// 40% prefer back-edge (StatementGoto.cpp:76)
	wantBack := r.RndFlipcoin(40)
	if wantBack {
		// find_good_jump_block(back) + pick statement — search all function blocks
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
			for i, st := range b.Stmts {
				if goodGotoTarget(st) {
					cands = append(cands, cand{b, i})
				}
			}
		}
		// also current block prefix (already committed stmts)
		if blk != nil {
			for i, st := range blk.Stmts {
				if goodGotoTarget(st) {
					cands = append(cands, cand{blk, i})
				}
			}
		}
		if len(cands) > 0 {
			c := cands[r.RndUpto(uint32(len(cands)))]
			// attach or reuse SourceLabel on target statement
			tgt := &c.blk.Stmts[c.idx]
			if tgt.SourceLabel == "" {
				tgt.SourceLabel = label
			} else {
				label = tgt.SourceLabel
			}
			return Stmt{Kind: StmtGoto, Expr: cond, Label: label, GotoBack: true}
		}
		// fall through to forward if no target
	}

	// forward edge: label will be placed after this stmt in MakeRandomBlock
	return Stmt{Kind: StmtGoto, Expr: cond, Label: label, GotoForward: true}
}
