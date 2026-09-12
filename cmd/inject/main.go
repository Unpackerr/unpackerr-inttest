// Command inject POSTs fake Starr queue rows into a running cmd/faker.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Unpackerr/unpackerr-inttest/internal/inject"
)

func main() {
	if err := inject.Main(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
