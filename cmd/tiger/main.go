/*
TODO(tso):
 - config (no args): --list
    - separate global, local, system
    - align around =
    - use pager
 - ignore:
    - echo $filename >> .gitignore && git add .gitignore
 - unignore:
    - echo "!"$filename >> .gitignore && git add .gitignore
 - ignored:
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
     - ignore: echo $filename >> .gitignore && git add .gitignore
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
 - password prompts

NOTE(tso): things that are possible thanks to one stackoverflow and their use of stty
           and a bradfitz post on golang-nuts from 2012 that i forgot about
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

TODO(tso): special-case submodules
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

type cmd struct {
	cmd *exec.Cmd
}

func newCmd(command string, args ...string) *cmd {
	return &cmd{exec.Command(command, args...)}
}

func (c *cmd) Output() (stdout, stderr string, err error) {
	o, e := &buf{}, &buf{}
	c.cmd.Stdout = o
	c.cmd.Stderr = e
	err = c.cmd.Run()
	return o.String(), e.String(), err
}

func (c *cmd) Attach() (err error) {
	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr
	return c.cmd.Run()
}

func (c *cmd) AttachWithPipe(pipe *exec.Cmd) (err error) {
	r, w := io.Pipe()
	c.cmd.Stdout = w
	pipe.Stdin = r
	pipe.Stdout = os.Stdout

	// NOTE(tso): yes I know about errWriter and stickyErr but I don't remember how to do them
	err = c.cmd.Start()
	if err != nil {
		return err
	}
	err = pipe.Start()
	if err != nil {
		return err
	}
	go func() {
		c.cmd.Wait()
		w.Close()
	}()
	return pipe.Wait()
}

func git(args ...string) *cmd {
	return newCmd("git", args...)
}

func pager() *exec.Cmd {
	p, err := config("core.pager")
	if err != nil {
		panic(err)
	}
	// NOTE(tso): core.pager can have any arbitrary shell syntax
	//            e.g.(mine right now): diff-so-fancy | less -RFX
	//            rather than try to reinvent bash just to be able to
	//            create an epic Pipe() abstraction
	//            let's just do this for now, consequences be damned:
	// -tso 2018-08-03 00:59:23a
	return exec.Command("sh", "-c", "cat - | "+p)
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
	stdout, _, err := git("worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(
		strings.TrimSpace(
			strings.Split(stdout, "\n")[0]), "worktree "), nil
}

func config(param string) (string, error) {
	stdout, _, err := git("config", param).Output()
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
	stdout, _, err := git("branch").Output()
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
	cwd, err := os.Getwd()
	if err != nil {
		panic("unexpected error: " + err.Error())
	}
	cwd = normalizePathSeparators(cwd)

	gwd, err := gitDir()
	if err != nil {
		// not a git repository
		fmt.Print(Red, "(not a git repository)", Reset, " ", path.Base(cwd), " % ")
		return
	}
	gwd = normalizePathSeparators(gwd)

	// always show working tree status first
	git("status", "-s", "-uall").Attach()

	repo := path.Base(gwd)
	// show cwd with respect to GIT_DIR
	cwd = strings.TrimPrefix(cwd, gwd)

	fmt.Print(Grey, "git@", Reset, Yellow, branch(), Reset, " ", Cyan, repo, cwd, Reset, " % ")
}

func main() {
	// for great justice
	zig := make(chan os.Signal, 1)
	signal.Notify(zig, os.Interrupt)
	go func() { <-zig; fmt.Println(); os.Exit(0) }()

	events := make(chan *event)
	watch, err := newWatcher(events)
	checkErr(err)
	go dispatch(
		events,
		func(filename string) bool {
			if path.Base(filename) == ".git" {
				return false
			}

			// TODO(tso): check if file is ignored before returning true
			return true
		},
		func() {
			// TODO(tso): only show if working tree has actually changed
			fmt.Println()
			prompt()
			// 	log.Println(BgMagenta + "[status update here]" + Reset)
		},
	)

	watchPaths := []string{}
	gwd, err := gitDir()
	if err == nil {
		watchPaths = watchAddWithSubdirs(watch, gwd)
	}
	// this is where you would put an annoying welcome message
	// TODO(tso): annoying welcome message
	prompt()

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

	somewhere:
		switch strings.TrimSpace(args[0]) {
		case "": // do nothing
		case "exit", "quit":
			break everywhere

		// reinventing coreutils poorly
		case "cd":
			if len(args) > 1 {
				err := os.Chdir(strings.Join(args[1:], " "))
				if err != nil {
					fmt.Println(Red, err, Reset)
				} else {
					currentGwd, err := gitDir()
					if currentGwd != gwd {
						watchRemove(watch, watchPaths)
					}
					if err == nil {
						gwd = currentGwd
						watchPaths = watchAddWithSubdirs(watch, gwd)
					}
				}
			}
		case "cat":
			// TODO(tso): this could use a lot of improvements
			// - don't rely on branch() cause it can print "(HEAD detached at bad1dea)"
			// - resolve relative filepaths
			// - make it clear somehow that this is not real cat

			if len(args) < 2 {
				fmt.Println(Red+"usage:"+Reset, "cat [branch (optional)] [filename]")
				break
			}

			var treeish, filename string
			if len(args) >= 3 {
				treeish = args[1]
				filename = strings.Join(args[2:], " ")
			} else {
				treeish = branch()
				filename = strings.Join(args[1:], " ")
			}

			stdout, _, err := git("ls-tree", treeish).Output()
			if err == nil {
				// we're in a git repository
				gitFiles := strings.Split(strings.TrimSpace(stdout), "\n")
				for _, ln := range gitFiles {
					var (
						mode              int
						thing, hash, name string
					)
					fmt.Sscanf(ln, "%d %s %s    %s", &mode, &thing, &hash, &name)

					if filename == name {
						git("cat-file", thing, hash).AttachWithPipe(pager())
						break somewhere
					}
				}
				fmt.Println(Red+"file:"+Reset, filename, Red+"not found @ revision:"+Reset, treeish)
			} else {
				if !fileExists(filename) {
					fmt.Println(Red+"file not found:"+Reset, filename)
					break
				}
				newCmd("cat", filename).AttachWithPipe(pager())
			}
		case "ls":
			// TODO(tso): this could use a lot of improvements
			// - don't rely on branch() cause it can print "(HEAD detached at bad1dea)"
			// - columns?
			// - sorting
			// - list directories first
			// - merge the two lists and use colors or a [x] to show
			//   which files are in the index, which are untracked, ignored...
			// - diff stats
			stdout, _, err := git("ls-tree", branch()).Output()
			if err == nil {
				// we're in a git repository
				gitFiles := strings.Split(strings.TrimSpace(stdout), "\n")
				for i, ln := range gitFiles {
					var (
						mode              int
						thing, hash, name string
					)
					fmt.Sscanf(ln, "%d %s %s    %s", &mode, &thing, &hash, &name)
					if thing == "tree" {
						name += "/"
					} else if thing != "blob" {
						name = "(" + thing + ") " + name
					}
					gitFiles[i] = name
				}
				fmt.Println("files known to git:")
				for _, f := range gitFiles {
					fmt.Print("\t", f, "\n")
				}
				fmt.Println()
			}

			cwd, err := os.Open(".")
			checkErr(err)
			files, err := cwd.Readdir(-1)
			checkErr(err)

			fmt.Println("current directory contents:")
			for _, f := range files {
				name := f.Name()
				if f.IsDir() {
					name += "/"
				}
				fmt.Print("\t", name, "\n")
			}

			fmt.Println()

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
			newCmd(ed, draft).Attach()

		case "commit":
			draft, err := draftFile()
			if err == nil {
				if fileExists(draft) {
					if len(args) == 1 {
						msg := fileGetContents(draft)
						println(git("commit", "-m", msg).Output())
						os.Remove(draft)
					} else {
						ch := make(chan struct{}, 1)
						go func() { println(git("commit", "-t", draft).Output()); ch <- struct{}{} }()
						<-time.After(time.Millisecond * 100)
						os.Remove(draft)
						<-ch
					}
					break
				}
			}

			if len(args) == 1 {
				// standard behavior (open editor, abort due to empty message)
				println(git("commit").Output())
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
			println(git(append([]string{"commit"}, flags...)...).Output())

			// feature: checkin: add everything, commit, and push
		case "ci", "checkin":
			// TODO(tso): this is a massive hack right now, make it sane
			stdout, _, err := git("status", "--porcelain").Output()
			if err != nil {
				println("", "", err)
				break
			}

			modified := false
			for _, ln := range strings.Split(stdout, "\n") {
				if len(ln) > 0 && ln[0] != ' ' && ln[0] != '?' {
					modified = true
					break
				}
			}

			if !modified {
				fmt.Println("nothing has been added to the index, run \"git add .\" first?")
				scanner.Scan()
				answer := scanner.Text()
				if len(answer) > 0 && (answer[0] == 'y' || answer[0] == 'Y') { // CS101 *looks at diploma in hand*
					if println(git("add", ".").Output()) != nil {
						fmt.Println("[ OK ]")
						break
					} else {
						fmt.Println("[ abort ]")
					}
				} else {
					fmt.Println("[ abort ]")
					break
				}
			}

			msg := strings.Join(args[1:], " ")
			if msg == "" {
				fmt.Println("enter commit message (optional):")
				scanner.Scan()
				msg = scanner.Text()
			}
			if println(git("commit", "--allow-empty", "--allow-empty-message", "-m", msg).Output()) == nil {
				_, _, err := git("remote", "show", "origin").Output()
				if err == nil {
					println(git("push").Output())
				}
			}
		case "log", "diff", "show": // things that use the pager XXX INCOMPLETE
			args = append(args[:1], append([]string{"--color"}, args[1:]...)...)
			git(args...).AttachWithPipe(pager())
		default: // treat all other git commands as usual
			git(args...).Attach()
		}
	there:
		prompt()
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("error reading stdin:", err)
	}
	fmt.Println()
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
