package exiftool

import (
	"context"
	"io"
	"os/exec"
)

func commandArgs(arg []string) []string {
	var args []string

	if Arg1 != "" {
		args = append(args, Arg1)
	}
	if Config != "" {
		args = append(args, "-config", Config)
	}

	args = append(args, "-charset", "filename=utf8")
	args = append(args, arg...)
	return args
}

// Command runs an ExifTool command with the given arguments and stdin and returns its stdout.
func Command(stdin io.Reader, arg ...string) (stdout []byte, err error) {
	cmd := exec.Command(Exec, commandArgs(arg)...)
	cmd.Stdin = stdin
	return cmd.Output()
}

// CommandContext is like Command but includes a context.
func CommandContext(ctx context.Context, stdin io.Reader, arg ...string) (stdout []byte, err error) {
	cmd := exec.CommandContext(ctx, Exec, commandArgs(arg)...)
	cmd.Stdin = stdin
	return cmd.Output()
}
