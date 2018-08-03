/*
TODO(tso):
 - attach io to subcommands that can launch editor
    - commit, revert, merge, rebase, add -i
 - attach pager to subcommands that use it
    - diff, log, show, cat
 - cd => cwd
    - add GIT_DIR to prompt
 - ls => ls-files
 - cat [<branch>] <file> => cat-file blob <derived hash>
 - config (no args): --list
    - separate global, local, system
    - align around =
    - use pager
 - ignore:
    - ls files currently ignored
    - cat .gitignore and .git/info/exclude
 - interactively setup remotes when push/pull fails
 - fix log --pretty="format string with spaces!"
    - and all other such args
 - stage: interactive staging
    ONE-BY-ONE: yes | git stage
       -OR-
    SELECT BY: git stage *.go
     - index
     - range
     - wildcard
     - file extension (include hidden)

    APPLY:
     - skip (do nothing)
     - ignore: echo $filename >> .gitignore
     - add
     - add -p
     - reset HEAD
     - checkout -f
         - fallback: cat-file blob [current branch] [hash] > file
     - checkout -p
     - rm --cached
     - rm -rf --no-preserve-root (os.Remove)

    PREVIEW CHANGES:
     - diff HEAD
     - diff HEAD --stat

    ABORTABORTABORT:
     - [q]uit o_k: done.
     - [u]ndo last action
     - [U]ndo like it never even happened (restore index)

    COMMIT:
     - [d]raft
     - [c]ommit now
     - [checkin]

    INVOKE:
    stage: add/remove/checkout...
    add: add only
    rm: remove only
    unstage: alias for reset HEAD and/or interactive remove/checkout

    maybe numbered options in addition to letters?

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

NOTE(tso): things that are possible thanks to one stackoverflow and their use of stty
           and a bradfitz post on golang-nuts from 2012 that i forgot about
 - support for shell commands
 - support for piping/indirection
 - password prompts

 ...we can read stdin 1 char at a time now! which means
    - delete non-printing characters
    - buffer line currently being typed and reprint if interrupted by status/fetch update (async event)

 and the following are now possible:

read -n 1
 - tab-complete without hitting enter
 - ctrl+d, bash/emacs bindings ctrl+a ctrl+e ctrl+u
    - I don't even know all of them I just know those ._.
 - be able to print to screen for async events
   without disrupting what a user is currently typing

cmd.Std(.*) = os.Std\1
 - paging: not only essential for log, diff BUT ALSO:
    - this means yes add/checkout -p!
 - add -i

NOTE(tso): still not possible:
 - prevent ctrl+c from exiting immediately
    - but maybe this is actually a good feature?
 - checklist but who needs that really

TODO(tso): options
    - fetch
        - ping interval
        - disable
    - status-inotify
        - disable
    - draft (go immediately into draft, for terminal editor users)
*/
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"time"
)

const (
	Black     = "\033[30m"
	Red       = "\033[31m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Magenta   = "\033[35m"
	Cyan      = "\033[36m"
	Grey      = "\033[37m"
	BgBlack   = "\033[30m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgGrey    = "\033[47m"
	Reset     = "\033[0m"

	PATH_SEPARATOR = string(os.PathSeparator)
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

func gitDir() (string, error) {
	stdout, _, err := git("worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(
		strings.TrimSpace(
			strings.Split(stdout, "\n")[0]), "worktree "), nil
}

func config(param string) (string, error) {
	stdout, _, err := git("config", param)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func draftFile() (string, error) {
	dir, err := gitDir()
	if err != nil {
		return "", err
	}
	return dir + PATH_SEPARATOR + ".git" + PATH_SEPARATOR + "COMMIT_DRAFTMSG", nil
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

// I'm getting both / and \ as path separators using Git Bash for Windows...
func normalizePathSeparators(path string) string {
	return strings.Replace(path, "\\", "/", -1)
}

// current branch to display in prompt()
func branch() string {
	stdout, _, err := git("branch")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(ln, "* ") {
			return strings.TrimPrefix(ln, "* ")
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
		fmt.Println("\nType \"" + Blue + "init" + Reset + "\" to get started with git!")
	}
	// show cwd with respect to GIT_DIR
	cwd, err := os.Getwd()
	if err != nil {
		panic("unexpected error: " + err.Error())
	}
	cwd = normalizePathSeparators(cwd)
	gwd, err := gitDir()
	if err != nil {
		// not a git repository
		fmt.Print(Red, "(not a git repository)", Reset, " ", path.Base(cwd), " % ")
	} else {
		gwd = normalizePathSeparators(gwd)

		repo := path.Base(gwd)
		dir := strings.TrimPrefix(cwd, gwd)

		fmt.Print(Grey, "git@", Reset, Yellow, branch(), Reset, " ", Cyan, repo, dir, Reset, " % ")
	}
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
		case "cd":
			if len(args) > 1 {
				err := os.Chdir(strings.Join(args[1:], " "))
				if err != nil {
					fmt.Println(Red, err, Reset)
				}
			}
		case "test":
			cmd := exec.Command("git", "diff", "--color", "HEAD^^", "main.go")
			// p, _ := config("core.pager")
			diff := exec.Command("perl", "Z:\\Users\\Leek\\bin\\diff-so-fancy")

			r, w := io.Pipe()
			cmd.Stdout = w
			diff.Stdin = r

			pager := exec.Command("less", "--tabs=4", "-R", "-F", "-X")
			r2, w2 := io.Pipe()
			diff.Stdout = w2
			pager.Stdin = r2
			// pager.Stdin = r
			pager.Stdout = os.Stdout

			cmd.Start()
			diff.Start()
			pager.Start()
			cmd.Wait()
			w.Close()
			diff.Wait()
			w2.Close()
			pager.Wait()
		case "test2":
			cmd := exec.Command("git", "diff", "--color", "HEAD^^", "main.go")
			p, _ := config("core.pager")
			pager := exec.Command("sh", "-c", "cat - | "+p)

			r, w := io.Pipe()
			cmd.Stdout = w
			pager.Stdin = r
			pager.Stdout = os.Stdout

			cmd.Start()
			pager.Start()
			cmd.Wait()
			w.Close()
			pager.Wait()
		case "test3":
			cmd := exec.Command("git", "commit", "--amend")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
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
