package errorhandler

import (
	"fmt"
	"os"
)

// ReportError centralizes error reporting. In the future, this can be wired to Sentry or another error tracking service.
func ReportError(err error, context string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", context, err)
	}
}
