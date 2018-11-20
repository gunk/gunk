package log

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	PrintCommands = false
	Verbose       = false
)

func Command(command string, args ...string) {
	if !PrintCommands {
		return
	}
	fmt.Printf("%s %s\n", command, strings.Join(args, " "))
}

func Log(format string, args ...interface{}) {
	if !Verbose {
		return
	}
	fmt.Printf(format, args...)
}

func PackageGenerated(pkgPath string) {
	if !Verbose {
		return
	}
	fmt.Println(pkgPath)
}

func ExecCommand(command string, args ...string) *exec.Cmd {
	Command(command, args...)
	cmd := exec.Command(command, args...)
	if Verbose {
		cmd.Stderr = os.Stdout
	}
	return cmd
}
