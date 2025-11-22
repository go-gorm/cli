package utils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ConfirmWrite prompts before creating or overwriting path unless auto is true.
func ConfirmWrite(path string, auto bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) && auto {
		return true, nil
	}
	action := "create"
	if err == nil && info.Mode().IsRegular() {
		action = "overwrite"
	}
	if auto {
		return true, nil
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stdout, "%s %s? [y/N]: ", strings.Title(action), path)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	response := strings.TrimSpace(strings.ToLower(line))
	return response == "y" || response == "yes", nil
}
