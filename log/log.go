package log

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var (
	Out io.Writer = os.Stderr

	PrintCommands = false
	Verbose       = false
)

func Printf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(Out, format, args...)
}

func Verbosef(format string, args ...interface{}) {
	if Verbose {
		Printf(format, args...)
	}
}

func ExecCommand(command string, args ...string) *exec.Cmd {
	if PrintCommands {
		Printf(formatCommand(command, args...))
	}
	cmd := exec.Command(command, args...)
	if Verbose {
		cmd.Stderr = Out
	}
	return cmd
}

// formatCommand formats the command output
func formatCommand(name string, params ...string) string {
	paramstr := " " + strings.Join(params, " ")
	if (len(paramstr) + len(name)) >= 40 {
		paramstr = ""
		for _, p := range params {
			paramstr += " \\\n  " + p
		}
	}
	return name + paramstr
}

// ExecError looks for plugin error if present in stderr.
// In any case, formats and create an error.
func ExecError(command string, err error) error {
	if xerr, ok := err.(*exec.ExitError); ok && len(xerr.Stderr) > 0 {
		// If the error contains some stderr, include it.
		// If we're running in verbose mode, stderr was already written
		// directly to os.Stderr, so it may not be here.
		err = fmt.Errorf("%v: %s", xerr.ProcessState, xerr.Stderr)
	}
	return fmt.Errorf("error executing %q: %v", command, err)
}
