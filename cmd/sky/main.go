package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mickyj70/sky/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, cli.New(), os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, command *cobra.Command, args []string, stdout, stderr io.Writer) int {
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)
	err := cli.Execute(ctx, command)
	if err != nil {
		fmt.Fprintln(stderr, "sky:", err)
	}
	return cli.ExitCode(err)
}
