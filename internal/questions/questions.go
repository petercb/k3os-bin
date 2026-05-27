// Package questions provides interactive terminal prompting utilities.
package questions

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
)

// PromptFormattedOptions displays numbered options and prompts the user to select one.
func PromptFormattedOptions(text string, def int, options ...string) (int, error) {
	var newOptions []string
	for i := range options {
		newOptions = append(newOptions, fmt.Sprintf("%d. %s\n", i+1, options[i]))
	}
	return PromptOptions(text+"\n", def, newOptions...)
}

// PromptOptions displays a list of options and prompts the user to select one by number.
func PromptOptions(text string, def int, options ...string) (int, error) {
	if len(options) == 1 {
		return 0, nil
	}

	PrintToTerm(text)
	for _, option := range options {
		PrintToTerm(option)
	}

	defString := ""
	if def >= 0 {
		defString = strconv.Itoa(def + 1)
	}

	for {
		answer, err := Prompt(fmt.Sprintf("Select Number [%s]: ", defString), defString)
		if err != nil {
			return 0, err
		}
		num, err := strconv.Atoi(answer)
		if err != nil {
			PrintfToTerm("Invalid number: %s\n", answer)
			continue
		}

		num--
		if num < 0 || num >= len(options) {
			PrintlnToTerm("Select a number between 1 and", +len(options))
			continue
		}

		return num, nil
	}
}

// PromptBool prompts the user for a yes/no answer and returns a boolean.
func PromptBool(text string, def bool) (bool, error) {
	msg := text + " [y/N]: "
	defStr := "n"
	if def {
		msg = text + " [Y/n]: "
		defStr = "y"
	}

	for {
		yn, err := Prompt(msg, defStr)
		if err != nil {
			return false, err
		}

		switch strings.ToLower(yn) {
		case "y":
			return true, nil
		case "n":
			return false, nil
		default:
			fmt.Println("Enter y or n")
		}
	}
}

// PrintToTerm prints text to the terminal, using stderr if stdout is not a TTY.
func PrintToTerm(text ...interface{}) {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Print(text...)
	} else {
		fmt.Fprint(os.Stderr, text...)
	}
}

// PrintlnToTerm prints text with a newline to the terminal, using stderr if stdout is not a TTY.
func PrintlnToTerm(text ...interface{}) {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println(text...)
	} else {
		fmt.Fprintln(os.Stderr, text...)
	}
}

// PrintfToTerm prints formatted text to the terminal, using stderr if stdout is not a TTY.
func PrintfToTerm(msg string, format ...interface{}) {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf(msg, format...)
	} else {
		fmt.Fprintf(os.Stderr, msg, format...)
	}
}

// Prompt displays text and reads a required response from stdin.
func Prompt(text, def string) (string, error) {
	for {
		PrintToTerm(text)
		answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", err
		}

		answer = strings.TrimSpace(answer)
		if answer == "" {
			answer = def
		}

		if answer == "" {
			continue
		}

		return answer, nil
	}
}

// PromptOptional displays text and reads an optional response from stdin, returning the default if empty.
func PromptOptional(text, def string) (string, error) {
	PrintToTerm(text)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}

	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = def
	}

	return answer, nil
}
