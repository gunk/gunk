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
		Printf("%s %s", command, strings.Join(args, " "))
	}
	cmd := exec.Command(command, args...)
	if Verbose {
		cmd.Stderr = Out
	}
	return cmd
}
