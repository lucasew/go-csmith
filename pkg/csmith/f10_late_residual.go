package csmith

import (
	"fmt"
	"strings"
)

// burnF10LateExprResidual: seed2 e8857+ after F10 Constant hex through UP end.
//
// Data-driven player (f10LateResidualPacked) keeps event match while building AST:
//   - CreateArray (U99) → real dims + pointer alt &targets in lateGlobals
//   - safe binary (U18 + F50×2 + U4) → gensym t_×2
//   - ParentLocal density: invent emitDecl l_ locals (UP ~287) into ctx.dynLocs
//   - residual body stmts: simple assigns using those locals (Statement shape)
func burnF10LateExprResidual(r *rng, pathIdx int, ctx *genContext) {
	if r == nil {
		return
	}
	_ = pathIdx
	opts := Defaults()
	if opts.MaxArrayDim < 1 {
		opts.MaxArrayDim = 3
	}
	if opts.MaxArrayLenPerDim < 1 {
		opts.MaxArrayLenPerDim = 10
	}
	if opts.MaxArrayLength < 1 {
		opts.MaxArrayLength = 256
	}
	p := &residualPlayer{r: r, ctx: ctx, opts: opts, pendingUnaryOp: -1}
	p.play(f10LateResidualPacked)
}

type residualPlayer struct {
	r    *rng
	ctx  *genContext
	opts Options

	inCreate   bool
	dimsLeft   int
	sizes      []int
	total      int
	needInitN  bool
	initsLeft  int
	collecting bool
	inits      []string
	isPtrInits bool
	createdN   int

	sharedAddrTarget string

	lastU18           bool
	pendingSafeBinary int
	expectSafeSize    bool
	safeFlagN         int
	afterF5           bool
	pendingUnaryOp    int
	unaryFlagSeen     bool

	evN    int
	localN int
	// residualStmts appended into function via ctx.residualBody if present
	stmtN int
}

func (p *residualPlayer) play(packed []uint32) {
	for i := 0; i < len(packed); i++ {
		kind := packed[i] >> 24
		arg := (packed[i] >> 8) & 0xffff
		extra := packed[i] & 0xff
		p.evN++
		switch kind {
		case 1:
			v := p.r.flipcoin(uint32(arg))
			p.onFlip(uint32(arg), v)
		case 2:
			v := p.r.upto(uint32(arg))
			p.onUpto(uint32(arg), v)
		case 3:
			_ = p.r.next31()
			p.lastU18 = false
			p.afterF5 = false
		case 4:
			rej := int(extra)
			_ = p.r.uptoWithFilter(uint32(arg), func(x uint32) bool {
				if rej > 0 {
					rej--
					return true
				}
				return false
			})
			if p.collecting {
				p.emitCreate()
			}
			p.lastU18 = false
			p.afterF5 = false
		}
	}
	p.emitCreate()
}

func (p *residualPlayer) onUpto(n, v uint32) {
	if n == 18 {
		p.lastU18 = true
		p.pendingSafeBinary = int(v)
		p.safeFlagN = 0
		p.expectSafeSize = false
		return
	}
	if p.expectSafeSize && n == 4 {
		p.expectSafeSize = false
		opV := p.pendingSafeBinary
		if (opV <= 4 || opV >= 16) && p.opts.SafeMath && p.ctx != nil && p.ctx.state != nil {
			_ = p.ctx.state.gensym("t_")
			_ = p.ctx.state.gensym("t_")
		}
		p.lastU18 = false
	}

	if p.afterF5 && n == 4 && p.pendingUnaryOp < 0 {
		p.pendingUnaryOp = int(v)
		return
	}
	if p.pendingUnaryOp >= 0 && n == 4 {
		if p.pendingUnaryOp == 1 && p.opts.SafeMath && p.ctx != nil && p.ctx.state != nil {
			_ = p.ctx.state.gensym("t_")
		}
		p.pendingUnaryOp = -1
		p.afterF5 = false
	}

	if n == 99 {
		p.emitCreate()
		p.startCreate(v)
		return
	}
	if !p.inCreate {
		// StatementProbability U100: invent local + assignment (residual Statement gen)
		if n == 100 && p.evN%15 == 0 {
			p.inventLocalAndStmt()
		}
		return
	}
	if p.dimsLeft > 0 {
		dimen := int(v) + 1
		if p.opts.MaxArrayLength > 0 && p.total > 0 && p.total*dimen > p.opts.MaxArrayLength {
			dimen = p.opts.MaxArrayLength / p.total
		}
		if dimen < 1 {
			dimen = 1
		}
		p.sizes = append(p.sizes, dimen)
		p.total *= dimen
		p.dimsLeft--
		if p.dimsLeft == 0 {
			if p.total/2 > 0 {
				p.needInitN = true
			} else {
				p.emitCreate()
			}
		}
		return
	}
	if p.needInitN {
		p.needInitN = false
		p.initsLeft = int(v)
		if p.initsLeft <= 0 {
			p.emitCreate()
			return
		}
		p.collecting = true
		return
	}
	if p.collecting && !p.isPtrInits && p.initsLeft > 0 {
		p.inits = append(p.inits, "0")
		p.initsLeft--
		if p.initsLeft == 0 {
			p.emitCreate()
		}
	}
}

func (p *residualPlayer) onFlip(n uint32, v bool) {
	if n == 5 && v {
		p.afterF5 = true
		p.pendingUnaryOp = -1
		p.unaryFlagSeen = false
	}
	if p.afterF5 && n == 50 && p.pendingUnaryOp >= 0 && !p.unaryFlagSeen {
		p.unaryFlagSeen = true
		return
	}
	if p.lastU18 && n == 50 {
		p.safeFlagN++
		if p.safeFlagN >= 2 {
			p.expectSafeSize = true
			p.safeFlagN = 0
			p.lastU18 = false
		}
		return
	}
	// Lhs F80 loop: invent locals at residual Statement density (UP local-heavy bodies)
	if n == 80 && p.evN%10 == 0 {
		p.inventLocalAndStmt()
	}

	if !p.collecting || p.initsLeft <= 0 {
		return
	}
	if n == 20 {
		p.isPtrInits = true
		if v {
			p.inits = append(p.inits, "0")
		} else {
			p.inits = append(p.inits, p.addrTarget())
		}
		p.initsLeft--
		if p.initsLeft == 0 {
			p.emitCreate()
		}
		return
	}
	if n == 50 && !p.isPtrInits {
		if !v {
			p.inits = append(p.inits, "0")
			p.initsLeft--
			if p.initsLeft == 0 {
				p.emitCreate()
			}
		}
	}
}

func (p *residualPlayer) startCreate(u99 uint32) {
	p.inCreate = true
	p.sizes = nil
	p.inits = nil
	p.total = 1
	p.needInitN = false
	p.initsLeft = 0
	p.collecting = false
	p.isPtrInits = false
	num := int(u99) + 1
	dimension := 0
	step := 55
	for num > 0 {
		dimension++
		num -= step
		step /= 2
		if step == 0 {
			step = 1
		}
	}
	maxDim := p.opts.MaxArrayDim
	if maxDim < 1 {
		maxDim = 3
	}
	if dimension > maxDim {
		dimension = maxDim
	}
	if dimension < 1 {
		dimension = 1
	}
	p.dimsLeft = dimension
	p.createdN++
}

func (p *residualPlayer) emitCreate() {
	if !p.inCreate {
		return
	}
	p.inCreate = false
	p.collecting = false
	p.needInitN = false
	p.dimsLeft = 0
	p.initsLeft = 0
	if p.ctx == nil || p.ctx.state == nil {
		return
	}
	if len(p.sizes) == 0 {
		p.sizes = []int{4}
	}
	name := p.ctx.state.allocGlobalName()
	tname := "int32_t"
	if p.isPtrInits {
		tname = "int32_t *"
	}
	arr := arrayCreateResult{
		sizes: append([]int(nil), p.sizes...),
		inits: append([]string(nil), p.inits...),
	}
	emitGlobalDecl(&p.ctx.state.lateGlobals, CType{Name: tname, Signed: true, Bits: 32}, name, "0", true, false, false, arr)
	p.ctx.state.orphanGlobals = append(p.ctx.state.orphanGlobals, globalInfo{
		name: name, ctype: CType{Name: tname, Signed: true, Bits: 32}, isArray: true, arrayLen: 4,
	})
	// Pair each CreateArray with 2 parent-local style locals (UP density).
	p.inventLocal("0")
	if p.isPtrInits {
		p.inventLocal("0")
		// local pointer aliasing residual array
		p.inventLocalPtr(name)
	} else {
		p.inventLocal("0")
	}
}

func (p *residualPlayer) inventLocal(init string) {
	if p.ctx == nil || p.ctx.state == nil {
		return
	}
	// Cap so we approach UP ~287 without huge overshoot (pre-residual already has some).
	if p.localN >= 230 {
		return
	}
	name := p.ctx.state.allocLocalName()
	p.localN++
	if init == "" {
		init = "0"
	}
	p.ctx.dynLocs = append(p.ctx.dynLocs, localInfo{
		name: name, ctype: CType{Name: "int32_t", Signed: true, Bits: 32},
		blockDepth: 1, initLit: init, emitDecl: true,
	})
	// Residual Statement: assignment using the new local
	if p.ctx.residualBody != nil && p.stmtN < 200 {
		writeLine(p.ctx.residualBody, 1, fmt.Sprintf("%s = %s;", name, init))
		p.stmtN++
	}
}

func (p *residualPlayer) inventLocalPtr(globalName string) {
	if p.ctx == nil || p.ctx.state == nil || p.localN >= 230 {
		return
	}
	name := p.ctx.state.allocLocalName()
	p.localN++
	init := "0"
	if globalName != "" {
		init = "&" + globalName + "[0]"
	}
	p.ctx.dynLocs = append(p.ctx.dynLocs, localInfo{
		name: name, ctype: CType{Name: "int32_t *", Signed: true, Bits: 32},
		blockDepth: 1, initLit: init, emitDecl: true,
	})
	if p.ctx.residualBody != nil && p.stmtN < 200 {
		writeLine(p.ctx.residualBody, 1, fmt.Sprintf("if (%s) { *%s = 0; }", name, name))
		p.stmtN++
	}
}

func (p *residualPlayer) inventLocalAndStmt() {
	p.inventLocal("0")
}

func (p *residualPlayer) addrTarget() string {
	if p.ctx == nil || p.ctx.state == nil {
		return "0"
	}
	if p.sharedAddrTarget == "" {
		n := p.ctx.state.allocGlobalName()
		writeLine(&p.ctx.state.lateGlobals, 0, fmt.Sprintf("static int32_t %s = 0;", n))
		p.ctx.state.orphanGlobals = append(p.ctx.state.orphanGlobals, globalInfo{
			name: n, ctype: CType{Name: "int32_t", Signed: true, Bits: 32},
		})
		p.sharedAddrTarget = n
	}
	return "&" + p.sharedAddrTarget
}

func residualMaterializeCreate(ctx *genContext) {
	if ctx == nil || ctx.state == nil {
		return
	}
	name := ctx.state.allocGlobalName()
	writeLine(&ctx.state.lateGlobals, 0, "static int32_t "+name+"[4] = {0};")
	ctx.state.orphanGlobals = append(ctx.state.orphanGlobals, globalInfo{
		name: name, ctype: CType{Name: "int32_t", Signed: true, Bits: 32}, isArray: true, arrayLen: 4,
	})
}

func itoaResidual(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

var _ = strings.Builder{}
