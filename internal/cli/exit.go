package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// exitCodeError carries a specific process exit code out of a RunE function.
// Commands return it instead of calling os.Exit, so deferred cleanup (locks,
// cobra post-runs) still executes; Execute translates it at the very end.
type exitCodeError struct {
	code int
}

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

// exitWithCode returns an error that makes the process exit with the given
// code. The command is expected to have already printed its verdict, so
// cobra's own error printing is silenced.
func exitWithCode(cmd *cobra.Command, code int) error {
	cmd.SilenceErrors = true
	return exitCodeError{code: code}
}

// resolveExitCode maps an Execute error to the process exit code.
func resolveExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ec exitCodeError
	if errors.As(err, &ec) {
		return ec.code
	}
	return 1
}
