// Command teo converts JSON/YAML to Token-Efficient Output and validates TEO.
package main

import (
	"os"

	"truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
