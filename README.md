# go get 'em tiger

> enhanced interactive git shell

![](summary.gif)

---

## Install

```
$ go get github.com/fsnotify/fsnotify
$ go get github.com/generaltso/go-git-em-tiger/cmd/tiger
```

## Usage

```
$ cd my_awesome_repo
$ tiger
```

## Features 

All standard git commands work as usual and probably your custom ones too.

To exit the prompt at any time, use `exit` or `quit` or simply press `CTRL+C`.

### *Enhanced* Prompt

Typing "git" is not necessary, but it's ok if you do it anyway:

![](gitgitgit.gif)

<!--
    git@master go-git-em-tiger % status
    On branch master
    Your branch is up-to-date with 'origin/master'.
    nothing to commit, working tree clean
    git@master go-git-em-tiger % git status
    On branch master
    Your branch is up-to-date with 'origin/master'.
    nothing to commit, working tree clean
-->

Always shows current working tree status first, with a custom format
which combines `git status -s` and `git diff --numstat`.
Additionally, the prompt automatically updates whenever a file changes (using
[fsnotify](https://github.com/fsnotify/fsnotify)):

![](fsnotify.gif)

<!--
    README.txt +159/-15
    git@master go-git-em-tiger %
    README.txt +165/-15
    git@master go-git-em-tiger %
-->

Always shows HEAD as a readable name:

![](head.gif)

<!--
    $ tiger
    git@master go-git-em-tiger % checkout -b new_branch
    Switched to a new branch 'new_branch'
    git@new_branch go-git-em-tiger % checkout master
    git@master go-git-em-tiger % checkout HEAD^
    [...]
    HEAD is now at 515648e... 
    git@master~1 go-git-em-tiger %
-->

Has basic navigation with `cd` and `ls` and always shows current directory as a
relative path within git repo:

![](cd.gif)

<!--
    $ cd $GOPATH/src/github.com/generaltso/go-git-em-tiger/cmd/tiger
    $ tiger
    git@master go-git-em-tiger/cmd/tiger %

 - basic navigation

    git@master go-git-em-tiger/cmd/tiger % cd ..
    git@master go-git-em-tiger/cmd % ls
    files known to git:
        tiger/

    current directory contents:
        tiger/
    git@master go-git-em-tiger/cmd % cd -
    git@master go-git-em-tiger/cmd/tiger %
-->

Works outside of a git repo too:

![](init.gif)

<!--
    $ mkdir example-repo
    $ tiger
    (not a git repository) tmp % cd example-repo
    (not a git repository) example-repo % init
    Initialized empty Git repository in /tmp/example-repo/.git/
    git@master example-repo %
-->

### *Enhanced* Git Functionality

`cat [@revision (optional, default: current HEAD)] [filename]`

Basically `git cat-file blob [hash]` without having to look the blob hash up
yourself with `git ls-tree [treeish]`.

![](cat.gif)

<!--
    git@master go-git-em-tiger/cmd/tiger % cat main.go
    /*
    TODO(tso):
    [...]
    git@master go-git-em-tiger/cmd/tiger % cat master~32 main.go
    package main

    import (
        "bufio"
    [...]
-->

`config` (***with no arguments***)

Pretty prints `git config --list`, separated into categories.

*When arguments are supplied, regular `git config` is invoked*

![](config.gif)

<!--
    git@master go-git-em-tiger/cmd/tiger % config
    system:

    global:
    user.email        = tso@teknik.io
    user.name         = tso
    push.default      = simple
    color.ui          = true
    core.excludesfile = ~/.gitignore
    core.editor       = gvim
    core.quotepath    = false
    core.pager        = diff-so-fancy | less --tabs=4 -RFX
    [...]
-->

`rm` 

Same as `git rm` except it doesn't fail on `.*` or untracked or ignored files.

![](rm.gif)

`commit` 

Same as `git commit` with the following improvements:

   - `-m` flag allows you to write a one liner message without having to quote
     or escape like you would in bash (or whatever you use)
   - *always* passes `--allow-empty-message`
   - see also `draft`

![](commit.gif)

#### Custom Features / New Commands

`draft` (no args)
 
Write a commit message while staging.

```
NOTE(tso): 
    draft only makes sense if you can have the editor and the shell
    open simultaneously, so terminal editor users will have to have
    each in separate panes/tabs/windows/screen or tmux sessions

    also it's harder to record this in action than I thought ...

    this is really useful when you're selectively staging things AND
    trying to craft a meaningful commit message at the same time.
```

![](draft-1.gif)
![](draft-2.gif)

<!--
    git draft                     #                   *start writing commit message*
    git add something             # "oh right..."     *write about something*
    git reset HEAD something_else # "hm, next commit" *delete line describing something_else*
    git mv yet_another thing      # "..."             *:%s/yet_another/thing/g*
    # :wq
    git commit
    [master b1251f2] I wrote this commit message while I was staging :D
     1 file changed, 1 insertion(+)

    or at the end
    git commit --edit # same as git commit -t $GIT_DIR/COMMIT_DRAFTMSG
                      # except you don't have to change the "template" for it 
                      # to count
-->
   
`checkin` or `ci`

Does the equivalent of `git add . && git commit && git push`.

 - only does `git add .` when there's nothing staged, and prompts you first

 - you can specify commit message with `ci (commit message goes here)`, otherwise
   it will prompt you

 - only pushes if remotes are setup

**`checkin` should be used with caution!**

```
NOTE(tso):
   some people like to review their commits and possibly rebase before pushing
   rather than pushing on every commit (you might also need to commit --amend
   or something else could happen that would require a push -f later)

   I understand that and respect it (I do that too sometimes) but some people
   (myself most of the time) like to commit and push everything as they go
   especially when you're "in the zone" so it's a convenience mostly and 
   not a recommended best practice at all.
```

![](checkin.gif)

`summary`

   github style summary with language statistics if you have my "l" command
   installed (optional): go get [github.com/generaltso/linguist/cmd/l](https://github.com/generaltso/linguist/tree/master/cmd/l)

![](summary.gif)

<!--
    git@master go-git-em-tiger % summary
    b6cb848 tso: update readme: document current features 8 minutes ago
              commits: 35 branches: 2 contributors: 1
                           Go: 100.00%
-->

---

## *Coming Soon*&trade;:

> See also [main.go](https://github.com/generaltso/go-git-em-tiger/blob/master/cmd/tiger/main.go#L1-L84) 
> which has my current TODO list at the top.

 - `git grep -n` always
    
 - undo all changes to a file, staged or not

```
git checkout some_file # tab-completion breaks here
    # can't do this during a merge/revert/rebase,
    # have to do this instead:
git ls-tree [branch or sub-tree @ revision]
    # (copy the hash of some_file)
git cat-file blob [hash] > some_file
git add some_file
```

 - things that should be automated somehow (provided you're not offline and github isn't down)

```
# these things should be up-to-date before you:
#   create a branch
#   create a tag
#   push
#
git branch -a | grep -v HEAD | perl -ne 'chomp($_); s|^\*?\s*||; if (m|(.+)/(.+)| && not $d{$2}) {print qq(git branch --track $2 $1/$2\n)} else {$d{$_}=1}' | csh -xfs
    # (that sets up remote tracking for all branches)
git fetch --all
git fetch --tags
cp -r .githooks/* .git/hooks
    # not enough people use git hooks because there's no agreed upon way
    # to distribute and install them. over-engineered solutions exist but
    # a .githooks/ folder seems like the simplest and best way provided
    # you always keep it up-to-date with .git/hooks which you could write
    # a hook for itself...
```

 - better help

    > `-h` and `--help` should still do the default behavior so subcommand
    >  `help` instead presents custom documentation, e.g:

    - `help log` (list of PRETTY FORMATS first, with less verbosity)

    ```
    git log
    Pretty Formats :: --pretty=" ... "
         %h     3a24901         short commit hash
         %cr    3 minutes ago   time
    ...
    ```

    - `help remote`

    ```
    git remote
    How to list remotes and branches that are tracked:
    ...
    How to remove remotes:
    ...
    How to update remote urls and their aliases:
    ...

    NOTE(tso): I don't actually know how to do any of these things :)
    ```

## Disclaimer

I am aware that many very good solutions already exist including 
but not limited to:

 - git-sh (both shell and ruby versions iirc)
 - fish shell can wrap commands like git iirc
 - hub
 - vim-fugitive
 - Atlassian SourceTree
 - github desktop for windows
 - git integration in Atom
 - GitKraken
 - gitk
 - *and of course your own personal shell scripts, aliases and functions...*

Objectively, there is nothing wrong with any of these tools and in fact
I think they're all pretty nifty. 

But I personally couldn't get as much use out of the ones I've used as I'd
hoped.

Adding yet another git wrapper to the list for the sake of not-invented-here
is not my goal.

Rather, I want to experiment with finding solutions that will let me do the
most common things in a simpler way / with a better user experience.
