package main

import (
	"fmt"
	"os"

	"github.com/cloud-byte-consulting/teo/internal/testreport"
)

func main() {
	junit := "test-results/junit.xml"
	if len(os.Args) > 1 {
		junit = os.Args[1]
	}
	if err := testreport.EmbedTokenReportInJUnit(junit); err != nil {
		fmt.Fprintf(os.Stderr, "embed token report: %v\n", err)
		os.Exit(1)
	}
}
