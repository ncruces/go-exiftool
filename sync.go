package exiftool

import (
	"io"
	"os/exec"
)

// Command runs an ExifTool command with the given arguments and stdin and returns its stdout.
func Command(stdin io.Reader, arg ...string) (stdout []byte, err error) {
	var args []string

	if Arg1 != "" {
		args = append(args, Arg1)
	}
	if Config != "" {
		args = append(args, "-config", Config)
	}

	args = append(args, "-charset", "filename=utf8")
	args = append(args, arg...)

	cmd := exec.Command(Exec, args...)
	cmd.Stdin = stdin
	return cmd.Output()
}
