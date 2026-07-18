// Upstream: Block.h / Block.cpp (BlockProbability, make_random).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

import (
	"fmt"
	"strings"
)

// Stmt is a minimal statement record (emit only; full Statement subclasses later).
type Stmt struct {
	Kind StatementType
	// Expr is return value / assign RHS / if-test when present.
	Expr *Expression
	// LhsVar is assign target when Kind==StmtAssign.
	LhsVar *Variable
	// Then/Else for if; Then is for-body for for.
	Then *Block
	Else *Block
	// Loop holds for-loop control (init/test/incr).
	Loop *LoopControl
	// AssignOp for StmtAssign (default simple).
	AssignOp AssignOp
	// ArrayAccess if set, used as LHS text (itemized array).
	ArrayAccess string
	// Label for goto target name (StmtGoto).
	Label string
	// SourceLabel is emitted before this statement (back-edge target).
	SourceLabel string
	// GotoForward: after this goto, insert a labeled no-op in the block.
	GotoForward bool
	// GotoBack: label lives on an earlier statement (SourceLabel).
	GotoBack bool
}

// Block mirrors Block : Statement with local_vars and stms.
type Block struct {
	Parent    *Block
	Func      *Function
	LocalVars []*Variable
	Stmts     []Stmt
	Looping   bool
	blockSize int // CGOptions::max_block_size at creation
	// TmpVars mirrors macro_tmp_vars (gensym t_ for safe math).
	TmpVars map[string]ESimpleType
}

// CreateNewTmpVar mirrors Block::create_new_tmp_var.
// Block.cpp:216–219.
func (b *Block) CreateNewTmpVar(sym *GenSym, st ESimpleType) string {
	if b == nil {
		return "t_0"
	}
	if b.TmpVars == nil {
		b.TmpVars = make(map[string]ESimpleType)
	}
	name := "t_1"
	if sym != nil {
		name = sym.Next("t_")
	}
	b.TmpVars[name] = st
	return name
}

// BlockProbability mirrors Block.cpp BlockProbability.

// Keep-filter on {block_size-1} forces return value max_block_size-1.
// Block.cpp:88–94.
func BlockProbability(blockSize int, r *Rng) int {
	if blockSize < 1 {
		return 0
	}
	// rnd_upto(block_size) with Keep filter only accepting block_size-1
	// → always block_size-1 after retries.
	_ = r
	return blockSize - 1
}

// MakeRandomBlock mirrors Block::make_random without DFA / nested loop / FactMgr.
// Block.cpp:115–226.
func MakeRandomBlock(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
	looping bool,
) *Block {
	if r == nil {
		return nil
	}
	f := cg.CurrentFunc
	parent := (*Block)(nil)
	if f != nil && len(f.Stack) > 0 {
		parent = f.Stack[len(f.Stack)-1]
	}
	b := &Block{
		Parent:    parent,
		Func:      f,
		Looping:   looping,
		blockSize: opts.MaxBlockSize,
	}
	if f != nil {
		f.Stack = append(f.Stack, b)
		f.Blocks = append(f.Blocks, b)
	}
	max := BlockProbability(b.blockSize, r)
	cg.BlkDepth++
	for i := 0; i <= max; i++ {
		st := makeRandomStmt(r, opts, probs, vs, tables, stmtTab, cg, b)
		b.Stmts = append(b.Stmts, st)
		// Forward goto: place labeled no-op after (simplified good forward target).
		if st.Kind == StmtGoto && st.GotoForward && st.Label != "" {
			b.Stmts = append(b.Stmts, Stmt{Kind: StmtLabel, SourceLabel: st.Label})
		}
		if st.Kind == StmtReturn {
			break
		}
	}
	cg.BlkDepth--
	if f != nil && len(f.Stack) > 0 {
		f.Stack = f.Stack[:len(f.Stack)-1]
	}
	return b
}

// makeRandomStmt picks a statement kind and fills a minimal Stmt.
// Full Statement::make_random deferred; uses StatementProbability + simple filters.
func makeRandomStmt(
	r *Rng,
	opts Options,
	probs *Probabilities,
	vs *VariableSelector,
	tables *ExprTables,
	stmtTab *ThresholdTable,
	cg CGContext,
	b *Block,
) Stmt {
	if stmtTab == nil {
		stmtTab = NewStatementThresholdTable(opts)
	}
	// StatementFilter subset: reject Block; reject Continue/Break unless IN_LOOP;
	// reject compound when blk_depth >= max.
	f := filterFunc(func(v uint32) bool {
		k := NumberToType(stmtTab, v)
		if k == StmtBlock {
			return true
		}
		if (k == StmtBreak || k == StmtContinue) && !cg.InLoop() {
			return true
		}
		if cg.BlkDepth >= opts.MaxBlockDepth && IsCompound(k) {
			return true
		}
		// void return type → reject Return
		if k == StmtReturn && cg.CurrentFunc != nil && cg.CurrentFunc.ReturnType != nil &&
			cg.CurrentFunc.ReturnType.IsSimple() && cg.CurrentFunc.ReturnType.Simple() == EVoid {
			return true
		}
		return false
	})
	kind := StatementProbabilityFilter(r, stmtTab, f)
	st := Stmt{Kind: kind}
	switch kind {
	case StmtReturn:
		// Expression::make_random for return type
		if cg.CurrentFunc != nil && cg.CurrentFunc.ReturnType != nil {
			st.Expr = MakeRandomExpression(r, opts, tables, vs, cg, cg.CurrentFunc.ReturnType, nil, false, false, MaxTermTypes, cg.ExprDepth)
			// If Function/Assign/Comma picked and returned nil, force Constant
			if st.Expr == nil {
				st.Expr = MakeRandomExpression(r, opts, tables, vs, cg, cg.CurrentFunc.ReturnType, nil, true, false, TermConstant, cg.ExprDepth)
			}
		}
	case StmtAssign:
		return MakeRandomAssign(r, opts, probs, vs, tables, cg, nil)
	case StmtBreak, StmtContinue:
		// bare
	case StmtIfElse:
		return *MakeRandomIf(r, opts, probs, vs, tables, stmtTab, cg)
	case StmtFor:
		return *MakeRandomFor(r, opts, probs, vs, tables, stmtTab, cg)
	case StmtArrayOp:
		return MakeRandomArrayOp(r, opts, probs, vs, tables, stmtTab, cg)
	case StmtGoto:
		return MakeRandomGoto(r, opts, probs, vs, tables, cg, b)
	case StmtInvoke:
		// still stubs
	default:
	}
	_ = b
	_ = probs
	return st
}

// Output emits C for the block with indent levels.
func (b *Block) Output(indent int) string {
	if b == nil {
		return "{\n}\n"
	}
	pad := strings.Repeat("    ", indent)
	inner := strings.Repeat("    ", indent+1)
	var sb strings.Builder
	sb.WriteString(pad + "{\n")
	// Block::OutputTmpVariableList
	for name, st := range b.TmpVars {
		sb.WriteString(inner)
		sb.WriteString(GetSimpleType(st).CName() + " " + name + " = 0;\n")
	}
	for _, lv := range b.LocalVars {
		if lv == nil || lv.Type == nil {
			continue
		}
		sb.WriteString(inner)
		if lv.IsArray && len(lv.ArraySizes) > 0 {
			av := &ArrayVariable{Variable: *lv, Sizes: lv.ArraySizes, InitValues: lv.ArrayInits}
			sb.WriteString(av.OutputDef())
			sb.WriteString("\n")
			continue
		}
		if lv.IsConst() {
			sb.WriteString("const ")
		}
		if lv.IsVolatile() {
			sb.WriteString("volatile ")
		}
		sb.WriteString(lv.Type.CName() + " " + lv.Name)
		if lv.Init != nil {
			sb.WriteString(" = " + lv.Init.Value)
		}
		sb.WriteString(";\n")
	}
	for _, st := range b.Stmts {
		if st.SourceLabel != "" {
			sb.WriteString(inner + st.SourceLabel + ":\n")
		}
		if st.Kind == StmtLabel {
			sb.WriteString(inner + "    ;\n")
			continue
		}
		sb.WriteString(inner)
		switch st.Kind {
		case StmtReturn:
			sb.WriteString("return")
			if st.Expr != nil {
				sb.WriteString(" " + st.Expr.Output())
			}
			sb.WriteString(";\n")
		case StmtAssign:
			lhs := ""
			if st.ArrayAccess != "" {
				lhs = st.ArrayAccess
			} else if st.LhsVar != nil {
				lhs = st.LhsVar.Name
			}
			if lhs != "" {
				rhs := "0"
				if st.Expr != nil {
					rhs = st.Expr.Output()
				}
				sb.WriteString(st.AssignOp.AssignOpC(lhs, rhs) + ";\n")
			} else {
				sb.WriteString("/* assign */;\n")
			}
		case StmtBreak:
			sb.WriteString("break;\n")
		case StmtContinue:
			sb.WriteString("continue;\n")
		case StmtFor:
			if st.Loop != nil && st.Loop.IV != nil {
				iv := st.Loop.IV.Name
				init := fmt.Sprintf("%s = %d", iv, st.Loop.InitN)
				test := fmt.Sprintf("%s %s %d", iv, st.Loop.TestOp.CmpOpC(), st.Loop.LimitN)
				incr := st.Loop.IncrOp.AssignOpC(iv, fmt.Sprintf("%d", st.Loop.IncrN))
				sb.WriteString(fmt.Sprintf("for (%s; %s; %s)\n", init, test, incr))
				if st.Then != nil {
					sb.WriteString(st.Then.Output(indent + 1))
				} else {
					sb.WriteString(inner + "{\n" + inner + "}\n")
				}
			} else {
				sb.WriteString("/* for-stub */;\n")
			}
		case StmtIfElse:
			sb.WriteString("if (")
			if st.Expr != nil {
				sb.WriteString(st.Expr.Output())
			} else {
				sb.WriteString("0")
			}
			sb.WriteString(")\n")
			if st.Then != nil {
				sb.WriteString(st.Then.Output(indent + 1))
			} else {
				sb.WriteString(inner + "{\n" + inner + "}\n")
			}
			sb.WriteString(inner + "else\n")
			if st.Else != nil {
				sb.WriteString(st.Else.Output(indent + 1))
			} else {
				sb.WriteString(inner + "{\n" + inner + "}\n")
			}
		case StmtGoto:
			if st.Label != "" {
				sb.WriteString("if (")
				if st.Expr != nil {
					sb.WriteString(st.Expr.Output())
				} else {
					sb.WriteString("0")
				}
				sb.WriteString(")\n")
				sb.WriteString(inner + "    goto " + st.Label + ";\n")
			} else {
				sb.WriteString("/* goto-stub */;\n")
			}
		case StmtArrayOp:
			// Emit as for-loop over array write (MakeRandomArrayOp filled Loop+Then).
			if st.Loop != nil && st.Loop.IV != nil {
				iv := st.Loop.IV.Name
				init := fmt.Sprintf("%s = %d", iv, st.Loop.InitN)
				test := fmt.Sprintf("%s %s %d", iv, st.Loop.TestOp.CmpOpC(), st.Loop.LimitN)
				incr := st.Loop.IncrOp.AssignOpC(iv, fmt.Sprintf("%d", st.Loop.IncrN))
				sb.WriteString(fmt.Sprintf("for (%s; %s; %s)\n", init, test, incr))
				if st.Then != nil {
					sb.WriteString(st.Then.Output(indent + 1))
				} else {
					sb.WriteString(inner + "{\n" + inner + "}\n")
				}
			} else {
				sb.WriteString("/* arrayop-stub */;\n")
			}
		case StmtInvoke:
			sb.WriteString("/* invoke-stub */;\n")
		default:
			sb.WriteString("/* stmt */;\n")
		}
	}
	sb.WriteString(pad + "}\n")
	return sb.String()
}
