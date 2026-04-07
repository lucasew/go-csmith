package errorhandler

import (
	"fmt"
	"os"
)

// ReportError is the single, centralized error-reporting function for the project.
// All code paths that handle unexpected errors MUST funnel through this function.
func ReportError(err error, context string) {
	if err == nil {
		return
	}
	// In the future, this can be wired to Sentry or another reporting backend.
	// For now, it logs to stderr with context.
	if context != "" {
		fmt.Fprintf(os.Stderr, "Error [%s]: %v\n", context, err)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
