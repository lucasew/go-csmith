package csmith

import (
	"fmt"
	"os"
	"runtime"

	"csmith/pkg/errorhandler"
)

const (
	lcgA          uint64 = 0x5DEECE66D
	lcgC          uint64 = 0xB
	lcgMask       uint64 = (1 << 48) - 1
	traceFilePerm        = 0644
)

// rng is compatible with the libc srand48/lrand48 core recurrence used by Csmith.
type rng struct {
	state     uint64
	trace     bool
	traceSite bool
	traceRaw  bool
	traceFile string
	tracePos  uint64
}

func newRNG(seed uint64) *rng {
	// srand48 semantics.
	r := &rng{state: ((seed << 16) + 0x330E) & lcgMask}
	if os.Getenv("CSMITH_TRACE_RNG") != "" {
		r.trace = true
		r.traceSite = os.Getenv("CSMITH_TRACE_RNG_SITE") != ""
		r.traceRaw = os.Getenv("CSMITH_TRACE_RNG_RAW") != ""
		r.traceFile = os.Getenv("CSMITH_TRACE_RNG_FILE")
		if r.traceFile == "" {
			r.traceFile = "/tmp/csmith-go-rng.trace"
		}
		err := os.WriteFile(r.traceFile, []byte(fmt.Sprintf("# seed=%d\n", seed)), traceFilePerm)
		if err != nil {
			errorhandler.ReportError(err, "failed to initialize trace file")
		}
	}
	return r
}

func (r *rng) next31() uint32 {
	r.state = (lcgA*r.state + lcgC) & lcgMask
	return uint32(r.state >> 17)
}

func (r *rng) upto(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	raw := r.next31()
	x := raw % n
	r.traceU(n, x, 0, raw)
	return x
}

func (r *rng) uptoWithFilter(n uint32, reject func(uint32) bool) uint32 {
	if n == 0 {
		return 0
	}
	raw := r.next31()
	x := raw % n
	if reject == nil || !reject(x) {
		r.traceU(n, x, 0, raw)
		return x
	}
	var tries uint32
	// Safety guard: avoid pathological infinite loops when all candidates are rejected.
	// Keep this large enough to preserve normal behavior while guaranteeing progress.
	const maxRejectRetries uint32 = 1 << 16
	for reject != nil && reject(x) && tries < maxRejectRetries {
		raw = r.next31()
		x = raw % n
		tries++
	}
	if reject != nil && reject(x) {
		panic(fmt.Sprintf("rng.uptoWithFilter: exceeded retry limit (n=%d tries=%d raw=%d)", n, tries, raw))
	}
	r.traceU(n, x, tries, raw)
	return x
}

func (r *rng) appendTraceLine(line string) {
	f, err := os.OpenFile(r.traceFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, traceFilePerm)
	if err != nil {
		errorhandler.ReportError(err, "failed to open trace file for appending")
		return
	}
	_, err = f.WriteString(line)
	if err != nil {
		errorhandler.ReportError(err, "failed to write to trace file")
	}
	err = f.Close()
	if err != nil {
		errorhandler.ReportError(err, "failed to close trace file")
	}
}

func (r *rng) traceU(n uint32, x uint32, tries uint32, raw uint32) {
	if r.trace {
		r.tracePos++
		var line string
		if r.traceSite && r.traceRaw {
			line = fmt.Sprintf("%d U %d -> %d tries=%d raw=%d @%s\n", r.tracePos, n, x, tries, raw, traceCaller())
		} else if r.traceSite {
			line = fmt.Sprintf("%d U %d -> %d @%s\n", r.tracePos, n, x, traceCaller())
		} else if r.traceRaw {
			line = fmt.Sprintf("%d U %d -> %d tries=%d raw=%d\n", r.tracePos, n, x, tries, raw)
		} else {
			line = fmt.Sprintf("%d U %d -> %d\n", r.tracePos, n, x)
		}
		r.appendTraceLine(line)
	}
}

func (r *rng) flipcoin(p uint32) bool {
	if p > 100 {
		p = 100
	}
	raw := r.next31()
	v := raw % 100
	ok := v < p
	if r.trace {
		r.tracePos++
		var b uint32
		if ok {
			b = 1
		}
		var line string
		if r.traceSite && r.traceRaw {
			line = fmt.Sprintf("%d F %d -> %d raw=%d @%s\n", r.tracePos, p, b, raw, traceCaller())
		} else if r.traceSite {
			line = fmt.Sprintf("%d F %d -> %d @%s\n", r.tracePos, p, b, traceCaller())
		} else if r.traceRaw {
			line = fmt.Sprintf("%d F %d -> %d raw=%d\n", r.tracePos, p, b, raw)
		} else {
			line = fmt.Sprintf("%d F %d -> %d\n", r.tracePos, p, b)
		}
		r.appendTraceLine(line)
	}
	return ok
}

func traceCaller() string {
	var pcs [12]uintptr
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		fr, more := frames.Next()
		name := fr.Function
		if name != "" && name != "csmith/pkg/csmith.(*rng).upto" && name != "csmith/pkg/csmith.(*rng).uptoWithFilter" && name != "csmith/pkg/csmith.(*rng).flipcoin" && name != "csmith/pkg/csmith.(*rng).traceU" {
			return name
		}
		if !more {
			break
		}
	}
	return "unknown"
}
