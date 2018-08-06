/*
TODO(tso):
 - rm [-r] [WILDCARD] that doesn't fail miserably for hidden files
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

 - periodically ping origin with fetch --dry-run

      origin(git@github.com:octocat/octoverse) 1 new commit! 2018-08-01 02:30:43a

 see README.txt for more features to implement

NOTE(tso): things that will lead to trouble so we shouldn't do right now/ever:
 - password prompts

NOTE(tso): things that are possible thanks to one stackoverflow and their use of stty

 ...we can read stdin 1 char at a time now! which means
    - delete non-printing characters
    - buffer line currently being typed and reprint if interrupted by status/fetch update (async event)
    - tab-complete without hitting enter
    - ctrl+d, bash/emacs bindings ctrl+a ctrl+e ctrl+u ctrl+l
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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	PATH_SEPARATOR = string(os.PathSeparator)
)

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

// current branch to display in prompt()
// TODO(tso): tags???
func branch() string {
	stdout, _, err := git("rev-parse", "--symbolic-full-name", "HEAD").Output()
	if err != nil {
		return ""
	}

	head := strings.TrimSpace(stdout)

	if head == "HEAD" {
		stdout, _, err := git("rev-parse", "--short", "HEAD").Output()
		if err == nil {
			return strings.TrimSpace(stdout)
		} else {
			log.Println(Red + "Error: couldn't determine branch ..." + Reset)
			return ""
		}
	}

	return strings.TrimPrefix(head, "refs/heads/")
}

func status() {
	stat, _, err := git("status", "--porcelain").Output()
	if err != nil {
		return
	}
	diff, _, err := git("diff", "--numstat").Output()
	if err != nil {
		return
	}
	diffHEAD, _, err := git("diff", "--numstat", "HEAD").Output()
	if err != nil {
		return
	}
	stat = strings.TrimSuffix(stat, "\n\n")
	diff = strings.TrimSuffix(diff, "\n\n")
	diffHEAD = strings.TrimSuffix(diffHEAD, "\n\n")

	type statusDiff struct {
		renamed, deleted   bool
		untracked, ignored bool
		plus, minus        int
	}
	staged := map[string]*statusDiff{}
	unstaged := map[string]*statusDiff{}
	for _, ln := range strings.Split(stat, "\n") {
		if ln == "" {
			continue
		}
		name := ln[3:]

		x := ln[0]
		y := ln[1]

		diff := &statusDiff{
			renamed:   x == 'R',
			deleted:   x == 'D',
			untracked: x == '?' || y == '?',
			ignored:   x == '!' || y == '!',
		}

		if x == '?' || y != ' ' {
			unstaged[name] = diff
		}
		if x != '?' && x != ' ' {
			staged[name] = diff
		}
	}

	for _, ln := range strings.Split(diff, "\n") {
		if ln == "" {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) != 3 {
			log.Printf("couldn't parse line: %#v", ln)
			return
		}
		name := parts[2]
		s, ok := unstaged[name]
		if !ok {
			continue
		}
		s.plus, _ = strconv.Atoi(parts[0])
		s.minus, _ = strconv.Atoi(parts[1])
	}

	for _, ln := range strings.Split(diffHEAD, "\n") {
		if ln == "" {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) != 3 {
			log.Printf("couldn't parse line: %#v", ln)
			return
		}
		name := parts[2]
		s, ok := staged[name]
		if !ok {
			continue
		}
		s.plus, _ = strconv.Atoi(parts[0])
		s.minus, _ = strconv.Atoi(parts[1])
	}

	// log.Printf("raw: %#v", stat)
	// log.Printf("staged: %#v", staged)
	// log.Printf("unstaged: %#v", unstaged)
	if len(staged) > 0 {
		// gee I sure love maps
		names := []string{}
		for name := range staged {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if s, ok := staged[name]; ok {
				diff := ""
				if s.plus != 0 && s.minus != 0 {
					diff = fmt.Sprintf("%s+%d%s/%s-%d%s", Green, s.plus, Reset, Red, s.minus, Reset)
				}
				color := Green
				if s.deleted {
					color = Red
				}
				fmt.Println(color+name+Reset, diff)
			}
		}
	}
	if len(unstaged) > 0 {
		// gee I sure love maps
		names := []string{}
		for name := range unstaged {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if s, ok := unstaged[name]; ok {
				diff := ""
				if s.plus != 0 && s.minus != 0 {
					diff = fmt.Sprintf("%s+%d%s/%s-%d%s", Green, s.plus, Reset, Red, s.minus, Reset)
				}
				color := Black + BgGrey
				if s.deleted {
					color = BgRed
				}
				if s.untracked {
					color = "untracked: " + color
				}
				fmt.Println(color+name+Reset, diff)
			}
		}
	}
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

	repo := path.Base(gwd)
	// show cwd with respect to GIT_DIR
	cwd = strings.TrimPrefix(cwd, gwd)

	// always show working tree status first
	status()

	fmt.Print(Grey, "git@", Reset, Yellow, branch(), Reset, " ", Cyan, repo, cwd, Reset, " % ")
}

func main() {
	// for great justice
	zig := make(chan os.Signal, 1)
	signal.Notify(zig, os.Interrupt)
	go func() { <-zig; fmt.Println(); os.Exit(0) }()

	difflast := ""
	stdout, _, err := git("diff", "--numstat").Output()
	if err == nil {
		difflast = strings.TrimSpace(stdout)
	}

	watch, err := newWatcher(
		func(filename string) bool {
			if path.Base(filename) == ".git" {
				return false
			}

			// TODO(tso): check if file is ignored before returning true
			return true
		},
		func() {
			stdout, _, err := git("diff", "--numstat").Output()
			diff := strings.TrimSpace(stdout)
			if err != nil || diff == difflast {
				return
			}
			difflast = diff
			fmt.Println()
			prompt()
			// 	log.Println(BgMagenta + "[status update here]" + Reset)
		},
	)
	checkErr(err)

	gwd, err := gitDir()
	if err == nil {
		watch.AddWithSubdirs(gwd)
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
					difflast = ""
					stdout, _, err := git("diff", "--numstat").Output()
					if err == nil {
						difflast = strings.TrimSpace(stdout)
					}
					currentGwd, err := gitDir()
					if currentGwd != gwd {
						watch.RemoveAll()
						if err == nil {
							gwd = currentGwd
							watch.AddWithSubdirs(gwd)
						}
					}
				}
			}
		case "cat":
			// TODO(tso): this could use a lot of improvements
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

			var answer string
			if !modified {
				fmt.Println(Cyan + "git add ." + Reset + " first? [if you don't type \"no\" I'm going to do it anyway]")
				scanner.Scan()
				answer = scanner.Text()
				if strings.ToLower(answer) == "no" {
					fmt.Println("[" + BgRed + " abort " + Reset + "]")
					break
				} else {
					if println(git("add", ".").Output()) == nil {
						fmt.Println("[ " + Green + "OK" + Reset + " ]")
					} else {
						fmt.Println("[" + BgRed + " abort " + Reset + "]")
						break
					}
				}
			}

			msg := strings.Join(args[1:], " ")
			if msg == "" {
				msg = answer
			}
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
