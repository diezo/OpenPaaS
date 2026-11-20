// Package ui prints friendly terminal output: colors, steps with spinners, boxes.
package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

var color = term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""

func wrap(code, s string) string {
	if !color {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func Bold(s string) string   { return wrap("1", s) }
func Dim(s string) string    { return wrap("2", s) }
func Green(s string) string  { return wrap("32", s) }
func Red(s string) string    { return wrap("31", s) }
func Yellow(s string) string { return wrap("33", s) }
func Cyan(s string) string   { return wrap("36", s) }

func OK(format string, a ...any)   { fmt.Printf("%s %s\n", Green("✓"), fmt.Sprintf(format, a...)) }
func Fail(format string, a ...any) { fmt.Printf("%s %s\n", Red("✗"), fmt.Sprintf(format, a...)) }
func Warn(format string, a ...any) { fmt.Printf("%s %s\n", Yellow("!"), fmt.Sprintf(format, a...)) }
func Info(format string, a ...any) { fmt.Printf("%s %s\n", Cyan("›"), fmt.Sprintf(format, a...)) }

// Header prints a section title.
func Header(s string) { fmt.Printf("\n%s\n", Bold(s)) }

// Step runs fn while showing a spinner, then prints ✓ or ✗.
func Step(title string, fn func() error) error {
	if !color {
		fmt.Printf("  … %s\n", title)
		err := fn()
		if err != nil {
			Fail(title)
		} else {
			OK(title)
		}
		return err
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(80 * time.Millisecond)
		defer t.Stop()
		for i := 0; ; i++ {
			fmt.Printf("\r%s %s", Cyan(frames[i%len(frames)]), title)
			select {
			case <-done:
				return
			case <-t.C:
			}
		}
	}()
	err := fn()
	close(done)
	wg.Wait()
	fmt.Print("\r\033[K")
	if err != nil {
		Fail(title)
	} else {
		OK(title)
	}
	return err
}

// Box prints lines inside a simple rounded box.
func Box(lines ...string) {
	width := 0
	for _, l := range lines {
		if n := len([]rune(stripANSI(l))); n > width {
			width = n
		}
	}
	fmt.Println("╭" + strings.Repeat("─", width+2) + "╮")
	for _, l := range lines {
		pad := width - len([]rune(stripANSI(l)))
		fmt.Println("│ " + l + strings.Repeat(" ", pad) + " │")
	}
	fmt.Println("╰" + strings.Repeat("─", width+2) + "╯")
}

func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '\033':
			in = true
		case in && r == 'm':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
