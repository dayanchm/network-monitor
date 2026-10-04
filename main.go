package main

import (
	"context"
	"flag"
	"fmt"
	"network-monitor/internal/diagnosticcli"
	"network-monitor/internal/server"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		if err := diagnosticcli.RunDiagnose(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
			if err == flag.ErrHelp {
				return
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	server.Run()
}
