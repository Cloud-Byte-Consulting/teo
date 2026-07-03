// Command teo converts standard data formats to Token-Efficient Output and validates TEO.
package main

import (
	"os"

	"github.com/cloud-byte-consulting/teo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
