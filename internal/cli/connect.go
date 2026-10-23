package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/capability"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/discovery"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// connect resolves the camera endpoint (explicit --host or USB discovery)
// and returns a configured Camera. No request is made for an explicit host.
func (a *App) connect(ctx context.Context) (*gopro.Camera, error) {
	ep, err := a.endpoint(ctx)
	if err != nil {
		return nil, err
	}
	opts := []gopro.Option{gopro.WithTimeout(a.opts.Timeout), gopro.WithUserAgent("gopro9-cli")}
	if a.opts.Verbose {
		opts = append(opts, gopro.WithLogger(slog.New(slog.NewTextHandler(a.stdio.Err, &slog.HandlerOptions{Level: slog.LevelDebug}))))
	}
	return gopro.New(ep, opts...), nil
}

func (a *App) endpoint(ctx context.Context) (transport.Endpoint, error) {
	if a.opts.Host != "" {
		return transport.Endpoint{Host: a.opts.Host, Port: a.opts.Port}, nil
	}
	dctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	found, err := discovery.Discover(dctx, discovery.Options{Port: a.opts.Port, ProbeTimeout: 1500 * time.Millisecond})
	if err != nil {
		return transport.Endpoint{}, &cameraError{err: err, code: exitcode.Unavailable}
	}
	if len(found) > 1 && !a.opts.Quiet {
		_, _ = fmt.Fprintf(a.stdio.Err, "warning: %d cameras found, using %s (pass --host to choose)\n", len(found), found[0].Address)
	}
	return found[0].Endpoint(), nil
}

// requireCapability refuses operations the matrix marks unsupported unless --force was given.
func (a *App) requireCapability(cmd *cobra.Command, op capability.Op) error {
	if err := capability.Check(op); err != nil {
		force, _ := cmd.Flags().GetBool("force")
		if force {
			return nil
		}
		return &cameraError{err: fmt.Errorf("%w (use --force to try anyway)", err), code: exitcode.Unavailable}
	}
	return nil
}

// cameraError pairs a camera failure with its exit code.
type cameraError struct {
	err  error
	code exitcode.Code
}

func (e *cameraError) Error() string           { return e.err.Error() }
func (e *cameraError) Unwrap() error           { return e.err }
func (e *cameraError) ExitCode() exitcode.Code { return e.code }

// wrapCameraErr maps SDK sentinel errors to exit codes.
func wrapCameraErr(err error) error {
	if err == nil {
		return nil
	}
	var coded interface{ ExitCode() exitcode.Code }
	if errors.As(err, &coded) {
		return err
	}
	switch {
	case errors.Is(err, capability.ErrUnsupported), errors.Is(err, transport.ErrNotFound):
		return &cameraError{err: err, code: exitcode.Unavailable}
	case errors.Is(err, transport.ErrUnreachable), errors.Is(err, transport.ErrTimeout), errors.Is(err, discovery.ErrNotFound):
		return &cameraError{err: err, code: exitcode.Unavailable}
	case errors.Is(err, transport.ErrServer):
		return &cameraError{err: fmt.Errorf("camera refused the command (busy or invalid in current mode): %w", err), code: exitcode.TempFail}
	case errors.Is(err, transport.ErrForbidden), errors.Is(err, transport.ErrBadRequest):
		return &cameraError{err: err, code: exitcode.DataErr}
	case errors.Is(err, transport.ErrBadResponse):
		return &cameraError{err: err, code: exitcode.DataErr}
	default:
		return err
	}
}

// addForceFlag registers --force on commands guarded by the capability matrix.
func addForceFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("force", false, "attempt the operation even if the capability matrix marks it unsupported")
}
