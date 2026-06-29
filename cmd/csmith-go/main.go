package main

import (
	"os"

	"csmith/internal/cli"
	"csmith/pkg/errorhandler"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		errorhandler.ReportError(err)
		os.Exit(1)
	}
}
