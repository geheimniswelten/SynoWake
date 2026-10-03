package main

import (
	"errors"
	"fmt"
	"os"
	"synowake/internal/synowake"
)

func main() {
	if err := synowake.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, synowake.ErrorText(err))
		if errors.Is(err, synowake.ErrStopped) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}
