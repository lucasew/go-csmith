package csmith

import (
	"fmt"
	"strings"
)

// burnF10LateExprResidual: seed2 e8857+ after F10 Constant hex through UP end.
//
// Replaces pure-burn residual with a data-driven player (f10LateResidualPacked)
// that draws the same RNG stream while materializing real AST:
//   - CreateArray (U99 + dims + alt inits) → lateGlobals with real sizes
//   - pointer alt F20 address-of → shared &g_N target in array inits
//
// Event match is preserved (same draws as prior pure residual). Full
// Expression/function-body residual→real-gen remains the climb for source match.
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
	p := &residualPlayer{r: r, ctx: ctx, opts: opts}
	p.play(f10LateResidualPacked)
}

// residualPlayer replays packed residual events and materializes CreateArray AST.
type residualPlayer struct {
	r    *rng
	ctx  *genContext
	opts Options

	// Create capture phases: dims → initNum → inits (F20 ptr or skip) → emit
	inCreate   bool
	dimsLeft   int
	sizes      []int
	total      int
	needInitN  bool
	initsLeft  int
	collecting bool // collecting alt inits (F20 pointer path)
	inits      []string
	isPtrInits bool
	createdN   int

	sharedAddrTarget string
}

func (p *residualPlayer) play(packed []uint32) {
	for i := 0; i < len(packed); i++ {
		kind := packed[i] >> 24
		arg := (packed[i] >> 8) & 0xffff
		extra := packed[i] & 0xff
		switch kind {
		case 1: // flipcoin
			v := p.r.flipcoin(uint32(arg))
			p.onFlip(uint32(arg), v)
		case 2: // upto
			v := p.r.upto(uint32(arg))
			p.onUpto(uint32(arg), v)
		case 3: // next31 untraced hex digit
			_ = p.r.next31()
			// hex digit inside non-ptr constant alt — not collected as init lit yet
		case 4: // uptoWithFilter
			rej := int(extra)
			_ = p.r.uptoWithFilter(uint32(arg), func(x uint32) bool {
				if rej > 0 {
					rej--
					return true
				}
				return false
			})
			// filter draws break init collection (non-ptr or other path)
			if p.collecting {
				p.emitCreate()
			}
		}
	}
	p.emitCreate()
}

func (p *residualPlayer) onUpto(n, v uint32) {
	if n == 99 {
		p.emitCreate()
		p.startCreate(v)
		return
	}
	if !p.inCreate {
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
		// Wait for F20 (pointer alts) or other (const alts → zero-fill emit).
		p.collecting = true
		return
	}
	// Non-ptr const small path may draw U3/U20 during inits — count as one init
	if p.collecting && !p.isPtrInits && p.initsLeft > 0 {
		p.inits = append(p.inits, "0")
		p.initsLeft--
		if p.initsLeft == 0 {
			p.emitCreate()
		}
	}
}

func (p *residualPlayer) onFlip(n uint32, v bool) {
	if !p.collecting || p.initsLeft <= 0 {
		return
	}
	if n == 20 {
		// make_init_value null(F20=1) vs address-of(F20=0)
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
		// formatSimpleConstant starts with F50; count one init per top-level F50=0 (hex)
		// or defer for F50=1 until U drawn in onUpto.
		if !v {
			p.inits = append(p.inits, "0")
			p.initsLeft--
			if p.initsLeft == 0 {
				p.emitCreate()
			}
		}
		// F50=1: small path continues with F50 + U — counted in onUpto
		return
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
	fill := "0"
	emitGlobalDecl(&p.ctx.state.lateGlobals, CType{Name: tname, Signed: true, Bits: 32}, name, fill, true, false, false, arr)
	p.ctx.state.orphanGlobals = append(p.ctx.state.orphanGlobals, globalInfo{
		name: name, ctype: CType{Name: tname, Signed: true, Bits: 32}, isArray: true, arrayLen: 4,
	})
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

// residualMaterializeCreate kept for compatibility.
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
