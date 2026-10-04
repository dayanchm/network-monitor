package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"network-monitor/internal/diagnosticcli"
	"os"
)

func main() {
	args := append([]string{"diagnose"}, os.Args[1:]...)
	if err := diagnosticcli.RunDiagnose(context.Background(), args, os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
