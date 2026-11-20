// Package prompt reads interactive input from the terminal.
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/diezo/openpaas/internal/ui"
)

var reader = bufio.NewReader(os.Stdin)

// ErrAborted is returned when input ends (Ctrl+D) or isn't available.
var ErrAborted = errors.New("input aborted")

func readLine() (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", ErrAborted
	}
	return strings.TrimSpace(line), nil
}

// Ask prompts for a value. If def is non-empty, Enter keeps it; shown is what
// to display for the default (lets callers mask secrets).
func Ask(label, def, shown string) (string, error) {
	if def != "" {
		if shown == "" {
			shown = def
		}
		fmt.Printf("%s %s: ", ui.Bold(label), ui.Dim("["+shown+"]"))
	} else {
		fmt.Printf("%s: ", ui.Bold(label))
	}
	v, err := readLine()
	if err != nil {
		return "", err
	}
	if v == "" {
		return def, nil
	}
	return v, nil
}

// AskSecret prompts without echoing input. Enter keeps def (shown masked).
func AskSecret(label, def, shown string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return Ask(label, def, shown)
	}
	if def != "" {
		fmt.Printf("%s %s: ", ui.Bold(label), ui.Dim("["+shown+"]"))
	} else {
		fmt.Printf("%s: ", ui.Bold(label))
	}
	b, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", ErrAborted
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return def, nil
	}
	return v, nil
}

// Choice asks for one of the given single-letter options; Enter picks def.
func Choice(label string, options []string, def string) (string, error) {
	for {
		fmt.Printf("%s %s: ", label, ui.Dim("["+strings.Join(options, "/")+"]"))
		v, err := readLine()
		if err != nil {
			return "", err
		}
		v = strings.ToLower(v)
		if v == "" {
			return def, nil
		}
		for _, o := range options {
			if strings.EqualFold(v, o) || (len(v) > 0 && strings.EqualFold(v[:1], o)) {
				return strings.ToLower(o), nil
			}
		}
	}
}

// PressEnter waits for the user to press Enter.
func PressEnter(msg string) error {
	fmt.Print(msg)
	_, err := readLine()
	return err
}
