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

func Command(command string, args ...string) {
	if !PrintCommands {
		return
	}
	fmt.Fprintf(Out, "%s %s\n", command, strings.Join(args, " "))
}

func Log(format string, args ...interface{}) {
	if !Verbose {
		return
	}
	fmt.Fprintf(Out, format, args...)
}

func PackageGenerated(pkgPath string) {
	if !Verbose {
		return
	}
	fmt.Fprintln(Out, pkgPath)
}

func ExecCommand(command string, args ...string) *exec.Cmd {
	Command(command, args...)
	cmd := exec.Command(command, args...)
	if Verbose {
		cmd.Stderr = Out
	}
	return cmd
}
