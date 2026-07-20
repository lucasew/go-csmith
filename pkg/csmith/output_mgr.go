// Upstream: OutputMgr.h / OutputMgr.cpp (monitored_funcs / curr_func).
// Pin: pkgs.csmith git 0cdc710315cfee9035e22ef4363ca479270d1934.
package csmith

// Process OutputMgr statics (OutputMgr.cpp:72–86).
var (
	// monitoredFuncs mirrors OutputMgr::monitored_funcs_.
	monitoredFuncs []string
	// currFunc mirrors OutputMgr::curr_func_.
	currFunc string
)

// ParseStringOptions mirrors CGOptions::parse_string_options.
// CGOptions.cpp:562–577 — split on comma; empty input no-op.
// Does not trim spaces (upstream substr without strip).
func ParseStringOptions(vname string) []string {
	if vname == "" {
		return nil
	}
	var v []string
	pos1 := 0
	for {
		pos2 := -1
		for i := pos1; i < len(vname); i++ {
			if vname[i] == ',' {
				pos2 = i
				break
			}
		}
		if pos2 >= 0 {
			v = append(v, vname[pos1:pos2])
			pos1 = pos2 + 1
		} else {
			v = append(v, vname[pos1:])
			break
		}
	}
	return v
}

// SetMonitoredFuncs mirrors CGOptions::monitored_funcs.
// CGOptions.cpp:558–560 — parse into OutputMgr::monitored_funcs_.
func SetMonitoredFuncs(fnames string) {
	monitoredFuncs = ParseStringOptions(fnames)
}

// MonitoredFuncs returns a copy of OutputMgr::monitored_funcs_.
func MonitoredFuncs() []string {
	if len(monitoredFuncs) == 0 {
		return nil
	}
	return append([]string(nil), monitoredFuncs...)
}

// ClearMonitoredFuncs resets process monitored list (tests / finalization).
func ClearMonitoredFuncs() {
	monitoredFuncs = nil
	currFunc = ""
}

// SetCurrFunc mirrors OutputMgr::set_curr_func.
// OutputMgr.cpp:77–79.
func SetCurrFunc(fname string) {
	currFunc = fname
}

// CurrFunc mirrors OutputMgr::curr_func_.
func CurrFunc() string { return currFunc }

// IsMonitoredFunc mirrors OutputMgr::is_monitored_func.
// OutputMgr.cpp:81–86 — empty list → all monitored; else curr must be in list.
func IsMonitoredFunc() bool {
	if len(monitoredFuncs) == 0 {
		return true
	}
	for _, n := range monitoredFuncs {
		if n == currFunc {
			return true
		}
	}
	return false
}
