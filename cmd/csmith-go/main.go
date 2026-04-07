package main

import (
	"os"

	"csmith/internal/cli"
	"csmith/pkg/errorhandler"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		errorhandler.ReportError(err, "cli_execute")
		os.Exit(1)
	}
}
