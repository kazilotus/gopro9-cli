// Package cli implements the gopro9 command tree. It contains no camera
// logic; every command delegates to pkg/gopro and renders the typed result.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/internal/exitcode"
)

// Stdio bundles the process streams so commands never touch os.Std*.
type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// GlobalOptions are the persistent flags shared by every command.
type GlobalOptions struct {
	Host    string
	Port    int
	Timeout time.Duration
	Output  render.Format
	Verbose bool
	Quiet   bool
	Yes     bool
}

// App holds injected dependencies for the command tree.
type App struct {
	stdio Stdio
	opts  GlobalOptions
	now   func() time.Time
}

// Option configures an App.
type Option func(*App)

// WithClock overrides the time source (tests).
func WithClock(now func() time.Time) Option { return func(a *App) { a.now = now } }

// NewApp constructs an App with defaults.
func NewApp(stdio Stdio, opts ...Option) *App {
	a := &App{stdio: stdio, now: time.Now}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Main parses args, runs the command, and returns the process exit code.
func Main(ctx context.Context, args []string, stdio Stdio) int {
	return NewApp(stdio).Run(ctx, args)
}

// Run executes the command tree with the given arguments.
func (a *App) Run(ctx context.Context, args []string) int {
	root := a.newRootCommand()
	root.SetArgs(args)
	root.SetIn(a.stdio.In)
	root.SetOut(a.stdio.Out)
	root.SetErr(a.stdio.Err)

	err := root.ExecuteContext(ctx)
	if err == nil {
		return int(exitcode.OK)
	}
	code := exitCodeFor(ctx, err)
	// Cobra already printed usage errors; only emit our own line for the rest.
	if code != exitcode.Usage {
		_ = render.PrintError(a.stdio.Err, a.opts.Output, code.String(), int(code), err.Error())
	}
	return int(code)
}

// exitCodeFor maps an error to a process exit code.
func exitCodeFor(ctx context.Context, err error) exitcode.Code {
	var coded interface{ ExitCode() exitcode.Code }
	switch {
	case errors.As(err, &coded):
		return coded.ExitCode()
	case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		return exitcode.Interrupted
	case errors.Is(err, context.DeadlineExceeded):
		return exitcode.Unavailable
	default:
		return exitcode.Software
	}
}

// usageError carries the Usage exit code for argument validation failures.
type usageError struct{ msg string }

func (e usageError) Error() string         { return e.msg }
func (usageError) ExitCode() exitcode.Code { return exitcode.Usage }
func newUsageError(format string, args ...any) error {
	return usageError{msg: fmt.Sprintf(format, args...)}
}

// print renders v using the selected output format.
func (a *App) print(cmd *cobra.Command, v any) error {
	return render.Print(cmd.OutOrStdout(), a.opts.Output, v)
}
