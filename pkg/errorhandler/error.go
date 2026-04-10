package errorhandler

import (
	"fmt"
	"os"
)

// ReportError centralizes error reporting for unexpected failures.
// As instructed by global directives, all unrecoverable errors should go through here.
func ReportError(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[ERROR] %s: %v\n", context, err)
}
