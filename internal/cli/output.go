package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Output format is a property of the consumer, not the project: it is
// resolved from the --output flag, then the METIS_OUTPUT env var, and never
// from .metis/project.yaml — a project-level default would let one consumer's
// preference leak into another's parsing.
const envOutput = "METIS_OUTPUT"

// outputFlag holds the value of the persistent --output flag.
var outputFlag string

// outputFormat resolves the output format: --output flag, then METIS_OUTPUT,
// then plain text.
func outputFormat() string {
	if outputFlag != "" {
		return outputFlag
	}
	if v := os.Getenv(envOutput); v != "" {
		return v
	}
	return "text"
}

// validateOutputFormat rejects anything other than text or json.
func validateOutputFormat() error {
	switch outputFormat() {
	case "text", "json":
		return nil
	default:
		return fmt.Errorf("invalid output format %q (valid: text, json)", outputFormat())
	}
}

// jsonOutput reports whether structured output was requested.
func jsonOutput() bool {
	return outputFormat() == "json"
}

// printJSON writes v as indented JSON to the command's stdout. Note
// cmd.Println is NOT used: cobra's Print helpers fall back to stderr when no
// output writer is set, which would put JSON on the wrong stream in real use.
func printJSON(cmd *cobra.Command, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
