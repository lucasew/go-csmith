package errorhandler

import (
	"fmt"
	"os"
)

// ReportError centralizes error reporting.
func ReportError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}
