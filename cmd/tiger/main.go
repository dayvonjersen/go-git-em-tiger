/*
TODO(tso):
 - ls => ls-files
 - cat <branch> <file> => cat-file blob <derived hash>

 see README.txt for more features to implement

NOTE(tso): things that will lead to trouble so we shouldn't do right now/ever:
 - support for shell commands
 - support for piping/indirection
 - password prompts

NOTE(tso): things that are impossible without manipulating the terminal:
 - tab-complete without hitting enter
 - paging
 - password prompts
*/
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

const (
	Red   = "\033[31m"
	Green = "\033[32m"
	Reset = "\033[0m"
)

type buf []byte

func (b *buf) Write(p []byte) (n int, err error) {
	*b = append(*b, p...)
	return len(p), nil
}

func (b *buf) String() string { return string(*b) }

func cmd(command string, args ...string) (stdout, stderr string, err error) {
	o, e := &buf{}, &buf{}
	cmd := exec.Command(command, args...)
	cmd.Stdout = o
	cmd.Stderr = e
	err = cmd.Run()
	return o.String(), e.String(), err
}

func git(args ...string) (stdout, stderr string, err error) {
	return cmd("git", args...)
}

// overriding built-in functions because I can't think of a better name
func println(stdout, stderr string, err error) error {
	if err != nil {
		fmt.Println(Red+"ERROR:"+Reset, err)
	}

	stdout = strings.TrimSpace(stdout)
	if stdout != "" {
		fmt.Println(stdout)
	}
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		fmt.Println(stderr)
	}

	return err
}

// current branch to display in prompt()
func branch() string {
	stdout, _, err := git("branch")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(ln, "* ") {
			return "(" + strings.TrimPrefix(ln, "* ") + ")"
		}
	}
	return ""
}

func prompt() {
	// always show working tree status
	// TODO:
	// - add colors
	// - truncate extra-long file-lists
	// - columns?
	//      - staged | unstaged | untracked OR
	//      - wrapped columns for long file list
	// - number and store in a slice so we can have "add 1 2 3" as a command
	stdout, _, err := git("status", "-s", "-uall")
	if err == nil {
		// don't show "on working tree clean"
		status := strings.TrimSpace(stdout)
		if status != "" {
			fmt.Println(status)
		}
	} else {
		fmt.Println(Red+"(not a git repository)"+Reset, "\n")
		fmt.Println("Type \"init\" to get started!")
	}
	fmt.Print("git", branch(), "> ")
}

func main() {
	// for great justice
	zig := make(chan os.Signal, 1)
	signal.Notify(zig, os.Interrupt)
	go func() { <-zig; fmt.Println(); os.Exit(0) }()

	// this is where you would put an annoying welcome message
	// TODO(tso): annoying welcome message
	prompt()

	// NOTE(tso): the downside to this approach is we can't easily have tab-complete
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		args := strings.Split(scanner.Text(), " ")

		// typing "git <command>" out of habit
		if args[0] == "git" {
			args = args[1:]
		}

		if len(args) == 0 {
			goto there
		}

		switch strings.TrimSpace(args[0]) {
		case "": // do nothing
		case "commit":
			if len(args) == 1 {
				// standard behavior (open editor, abort due to empty message)
				println(git("commit"))
				break
			}
			args = args[1:]
			// enhanced behavior: accomodate one-liner commit message
			//     ∗ always --allow-empty-message
			flags := []string{"--allow-empty-message"}
		here:
			for n, arg := range args {
				switch arg {
				case "-m": // NOTE(tso): -m eats everything to end-of-line and uses it as commit message!
					msg := ""
					if len(args) > n+1 {
						msg = strings.Join(args[n+1:], " ")
					} else {
						fmt.Println("enter commit message (optional):")
						scanner.Scan()
						msg = scanner.Text()
					}
					flags = append(flags, "-m", msg)
					break here
				default:
					flags = append(flags, arg)
				}
			}
			println(git(append([]string{"commit"}, flags...)...))

			// feature: checkin: add everything, commit, and push
		case "ci", "checkin":
			if println(git("add", ".")) != nil {
				break
			}
			msg := strings.Join(args[1:], " ")
			if msg == "" {
				fmt.Println("enter commit message (optional):")
				scanner.Scan()
				msg = scanner.Text()
			}
			if println(git("commit", "--allow-empty-message", "-m", msg)) == nil {
				println(git("push"))
			}
		default: // treat all other git commands as usual
			println(git(args...))
		}
	there:
		prompt()
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("error reading stdin:", err)
	}
	fmt.Println()
}
