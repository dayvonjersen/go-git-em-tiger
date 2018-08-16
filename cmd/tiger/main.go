/*
TODO(tso):
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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
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
	checkErr(err)
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
	return strings.TrimPrefix(strings.TrimSpace(strings.Split(stdout, "\n")[0]), "worktree "), nil
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

func ignoreFile() (*os.File, string, error) {
	dir, err := gitDir()
	if err != nil {
		return nil, "", err
	}
	path := dir + PATH_SEPARATOR + ".gitignore"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, os.ModePerm)
	checkErr(err)
	return f, path, nil
}

// current branch/tag to display in prompt()
func head() string {
	dir, err := gitDir()
	if err != nil {
		return ""
	}

	head := fileGetContents(dir + PATH_SEPARATOR + ".git" + PATH_SEPARATOR + "HEAD")
	checkErr(err)

	if strings.HasPrefix(head, "ref: refs/heads/") {
		return strings.TrimSpace(strings.TrimPrefix(head, "ref: refs/heads/"))
	}

	stdout, _, err := git("rev-parse", "--symbolic-full-name", "HEAD").Output()
	checkErr(err)

	revParse := strings.TrimSpace(stdout)

	if revParse == "HEAD" {
		stdout, _, err = git("name-rev", "HEAD").Output()
		checkErr(err)

		nameRev := strings.TrimSpace(stdout)
		nameRev = strings.TrimPrefix(nameRev, "HEAD ")
		nameRev = strings.TrimPrefix(nameRev, "tags/")
		return nameRev
	}
	revParse = strings.TrimPrefix(revParse, "refs/heads/")

	return revParse
}

func status() {
	stat, _, err := git("status", "--porcelain").Output()
	stat = strings.TrimSuffix(stat, "\n")
	if err != nil || stat == "" { // (not a git repo) or "on working directory clean"
		return
	}

	type statusDiff struct {
		renamed, deleted   bool
		untracked, ignored bool
		plus, minus        int
	}

	staged := map[string]statusDiff{}
	unstaged := map[string]statusDiff{}

	for _, ln := range strings.Split(stat, "\n") {
		name := ln[3:]

		x := ln[0]
		y := ln[1]

		diff := statusDiff{
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

	parseDiff := func(_diff string, m map[string]statusDiff) map[string]statusDiff {
		if _diff == "" {
			return m
		}
		for _, ln := range strings.Split(_diff, "\n") {
			parts := strings.Split(ln, "\t")
			name := parts[2]
			s, ok := m[name]
			if !ok {
				continue
			}
			s.plus, _ = strconv.Atoi(parts[0])
			s.minus, _ = strconv.Atoi(parts[1])
			m[name] = s
		}
		return m
	}

	if len(unstaged) > 0 {
		diff, _, err := git("diff", "--numstat").Output()
		if err == nil {
			diff = strings.TrimSuffix(diff, "\n")
			unstaged = parseDiff(diff, unstaged)
		}
	}

	if len(staged) > 0 {
		diffHEAD, _, err := git("diff", "--numstat", "HEAD").Output()
		if err == nil {
			diffHEAD = strings.TrimSuffix(diffHEAD, "\n")
			staged = parseDiff(diffHEAD, staged)
		}
	}

	sortMapKeys := func(m map[string]statusDiff) []string {
		names := []string{}
		for name := range m {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}

	println := func(color, delcolor, name string, s statusDiff) {
		diff := ""
		if !(s.plus == 0 && s.minus == 0) {
			diff = fmt.Sprintf("%s+%d%s/%s-%d%s", Green, s.plus, Reset, Red, s.minus, Reset)
		}
		if s.deleted {
			color = delcolor
		}
		if s.untracked {
			color = "untracked: " + color
		}
		fmt.Println(color+name+Reset, diff)
	}

	for _, name := range sortMapKeys(staged) {
		println(Green, Red, name, staged[name])
	}

	for _, name := range sortMapKeys(unstaged) {
		println(Black+BgGrey, BgRed, name, unstaged[name])
	}
}

func prompt() {
	cwd, err := os.Getwd()
	checkErr(err)
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

	fmt.Print(Grey, "git@", Reset, Yellow, head(), Reset, " ", Cyan, repo, cwd, Reset, " % ")
}

func summary() {
	lastCommit, _, err := git("log", "-1", "--pretty=%h %an: %s %cr").Output()
	if err != nil {
		return
	}
	commits, _, err := git("rev-list", "HEAD").Output()
	checkErr(err)
	branches, _, err := git("branch").Output()
	checkErr(err)
	authors, _, err := git("shortlog", "-s").Output()
	checkErr(err)

	tw := newTabwriter(os.Stdout)
	fmt.Fprint(tw, lastCommit)
	fmt.Fprintf(tw,
		"commits: %d branches: %d contributors: %d\n",
		strings.Count(commits, "\n"),
		strings.Count(branches, "\n"),
		strings.Count(authors, "\n"),
	)
	hasL, _, err := newCmd("which", "l").Output()
	if hasL != "" && err == nil {
		l := []struct {
			Language string
			Percent  float64
		}{}
		data, _, err := newCmd("l", "-json", "-limit", "3").Output()
		checkErr(err)
		checkErr(json.Unmarshal([]byte(data), &l))
		for _, l := range l {
			fmt.Fprintf(tw, "%s: %.2f%% ", l.Language, l.Percent)
		}
	}
	tw.Flush()
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

	lastCwd, err := os.Getwd()
	checkErr(err)

	gwd, err := gitDir()
	if err == nil {
		go watch.AddWithSubdirs(gwd)
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

		case "summary": // github-style summary
			summary()

		// reinventing coreutils poorly
		case "cd":
			if len(args) > 1 {
				cd := strings.Join(args[1:], " ")
				if cd == "-" {
					cd = lastCwd
				}
				lastCwd, err = os.Getwd()
				checkErr(err)
				err := os.Chdir(cd)
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
							go watch.AddWithSubdirs(gwd)
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
				treeish = head()
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
			stdout, _, err := git("ls-tree", head()).Output()
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

		case "mkdir": // doesn't do bash shell expansion e.g. mkdir -p go/{bin,pkg,src}/
			if len(args) != 2 {
				println("", "", fmt.Errorf("mkdir takes exactly 1 argument"))
				break
			}
			checkErr(os.MkdirAll(args[1], os.ModePerm))

			// "improved" git rm
			//  - don't fail just because .* matches . and ..
			//  - don't choke just because [glob pattern] matches untracked or ignored files
			// accomplishing this for now by globbing first
			// and doing each rm one at a time
			// so if one or more operations fail the other operations don't fail
		case "rm":
			flags := []string{}
			paths := []string{}
			for _, arg := range args[1:] {
				switch arg {
				case "-h", "--help", "-f", "--force", "-n", "-r", "--cached", "--ignore-unmatch", "--quiet", "--":
					flags = append(flags, arg)
				default:
					glob, err := filepath.Glob(arg)
					if err != nil {
						fmt.Println(err)
						break somewhere
					}
					paths = append(paths, glob...)
				}
			}
			if len(paths) == 0 {
				git(append([]string{"rm"}, flags...)...).Attach()
			}
			for _, path := range paths {
				git(append([]string{"rm"}, append(flags, path)...)...).Attach()
			}

		// feature: naked "git config" pretty-prints git config --list
		case "config":
			if len(args) != 1 {
				git(args...).Attach()
				break
			}
			system, _, _ := git("config", "--list", "--system").Output()
			global, _, _ := git("config", "--list", "--global").Output()
			local, _, _ := git("config", "--list", "--local").Output()
			out := &buf{}

			align := func(input string) string {
				lines := strings.Split(input, "\n")
				max := 0
				for _, ln := range lines {
					idx := strings.Index(ln, "=")
					if idx > max {
						max = idx
					}
				}
				for i, ln := range lines {
					idx := strings.Index(ln, "=")
					if idx < 0 {
						continue
					}
					key := ln[:idx]
					value := ln[idx+1:]
					lines[i] = key + strings.Repeat(" ", max-len(key)) + " = " + value
				}
				return strings.Join(lines, "\n")
			}

			fmt.Fprintln(out, "system:")
			fmt.Fprintln(out, align(system))
			fmt.Fprintln(out, "global:")
			fmt.Fprintln(out, align(global))
			fmt.Fprintln(out, "local:")
			fmt.Fprintln(out, align(local))

			less := pager()
			less.Stdin = out
			less.Stdout = os.Stdout
			less.Stderr = os.Stderr
			less.Run()

			// feature: keep: ignore a subdir's contents but keep the dir in the working tree
		case "keep":
			if len(args) != 2 {
				println("", "", fmt.Errorf("keep takes exactly 1 argument"))
				break
			}

			// make sure we're in a git repo
			_, err := gitDir()
			if err != nil {
				println("", "", fmt.Errorf("keep only works in a git repo!!"))
				break
			}

			dir := strings.TrimRight(args[1], "\\/")
			if !fileExists(dir) {
				checkErr(os.MkdirAll(dir, os.ModePerm))
			}
			if !isDir(dir) {
				println("", "", fmt.Errorf("%s is not a directory", dir))
				break
			}
			keepFile := dir + PATH_SEPARATOR + ".keep"
			if !fileExists(keepFile) {
				f, err := os.Create(keepFile)
				checkErr(err)
				f.Close()
			}

			f, abspath, err := ignoreFile()
			checkErr(err)
			io.WriteString(f, dir+"/\n!"+dir+"/.keep\n")
			f.Close()
			git("add", "-f", keepFile, abspath).Attach()

		// feature: ignore/unignore: add lines to .gitignore
		case "unignore":
			for i := 1; i < len(args); i++ {
				args[i] = "!" + args[i]
			}
			fallthrough
		case "ignore":
			f, abspath, err := ignoreFile()
			if err != nil {
				println("", "", err)
				break
			}
			for _, arg := range args[1:] {
				io.WriteString(f, arg+"\n")
			}
			f.Close()
			git("add", abspath).Attach()

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
			flags := []string{}
		here:
			for n, arg := range args {
				switch arg {
				case "-m": // NOTE(tso): -m eats everything to end-of-line and uses it as commit message!
					// enhanced behavior: accomodate one-liner commit message
					//     ∗ always --allow-empty-message
					flags = append(flags, "--allow-empty-message")
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
