package main

import (
	"fmt"
	"os"

	goamxbackend "github.com/pawnkit/pawndebug/backend/goamx"
	"github.com/pawnkit/pawndebug/dap"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Println(version)

		return
	}

	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: pawndebug [--version]")
		os.Exit(2)
	}

	if err := (&dap.Server{Backend: goamxbackend.New()}).Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "pawndebug:", err)
		os.Exit(1)
	}
}
