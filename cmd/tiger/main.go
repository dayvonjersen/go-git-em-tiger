/*
TODO(tso):
 - ls => ls-files
 - cat [<branch>] <file> => cat-file blob <derived hash>
 - fix log --pretty="format string with spaces!"
    - and all other such args
 - add 1 2 3, rm 1 2 3, checkout 1 2 3
 - rm [WILDCARD] that doesn't fail miserably
 - auto-update status using inotify/fswatch
    - we could also periodically ping origin with fetch --dry-run but let's not get ahead of ourselves
    - any of this automatic stuff should not interrupt the user while typing
      but that's unavoidable without manipulating the terminal to insert a line
      and reprint what the user has already typed in e.g.

      git(master)> commit -m add feature foo to
      origin(git@github.com:octocat/octoverse) 1 new commit! 2018-08-01 02:30:43a
      git(master)> commit -m add feature foo to wait ^C
      git(master)> pull
      blabla your branch is now even with origin/master
      git(master)> commit ...

 - add diff --stat to status

 see README.txt for more features to implement

NOTE(tso): things that will lead to trouble so we shouldn't do right now/ever:
 - support for shell commands
 - support for piping/indirection
 - password prompts

NOTE(tso): things that are _impossible_ without _completely_ manipulating the terminal:
 - password prompts
 - tab-complete without hitting enter
 - paging: not only essential for log, diff BUT ALSO:
    - this means no add/checkout -p!
 - checklist for selecting files interactively
 - ctrl+d, bash/emacs bindings ctrl+a ctrl+e ctrl+u
    - I don't even know all of them I just know those ._.
 - add -i
 - prevent ctrl+c from exiting immediately
    - but maybe this is actually a good feature?
 - be able to print to screen for async events
   without disrupting what a user is currently typing
*/
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"
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

func dir() (string, error) {
	stdout, _, err := git("worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(
		strings.TrimSpace(
			strings.Split(stdout, "\n")[0]), "worktree ") + string(os.PathSeparator) + ".git", nil
}

func config(param string) (string, error) {
	stdout, _, err := git("config", param)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func draftFile() (string, error) {
	dir, err := dir()
	if err != nil {
		return "", err
	}
	return dir + string(os.PathSeparator) + "COMMIT_DRAFTMSG", nil
}

func fileExists(filename string) bool {
	f, err := os.Open(filename)
	f.Close()
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		panic(err)
	}
	return true
}

func fileGetContents(filename string) string {
	contents := &buf{}
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(contents, f)
	f.Close()
	if err != nil && err != io.EOF {
		panic(err)
	}
	return contents.String()
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
everywhere:
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
		case "exit", "quit":
			break everywhere

		// feature: draft: edit commit message while staging
		case "draft":
			draft, err := draftFile()
			if err != nil {
				println("", "", err)
				break
			}
			ed, err := config("core.editor")
			if err != nil {
				println("", "", err)
				break
			}
			cmd("cmd.exe", "/c", "start", ed, draft)

		case "commit":
			draft, err := draftFile()
			if err == nil {
				if fileExists(draft) {
					if len(args) == 1 {
						msg := fileGetContents(draft)
						println(git("commit", "-m", msg))
						os.Remove(draft)
					} else {
						ch := make(chan struct{}, 1)
						go func() { println(git("commit", "-t", draft)); ch <- struct{}{} }()
						<-time.After(time.Millisecond * 100)
						os.Remove(draft)
						<-ch
					}
					break
				}
			}

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
