experimenting with trying to improve my personal git workflow with an
interactive git shell

---

first I should say that many very good solutions already exist including:
 ∗ git-sh (both shell and ruby versions iirc)
 ∗ fish shell can wrap commands like git iirc
 ∗ hub

other git workflow improvement tools include
 ∗ vim-fugitive
 ∗ Atlassian SourceTree
 ∗ github desktop for windows
 ∗ git integration in Atom
 ∗ GitKraken
 ∗ gitk

(and of course shell scripts, aliases and functions)

Objectively, there is nothing wrong with any of these tools and in fact
they're pretty nifty. But I couldn't get as much use out of them as I'd have
liked.

Adding yet another git wrapper to the list for the sake of not-invented-here
is not my goal.

Rather, I want to experiment with finding solutions that will let me do the
most common things in a simpler way / with a better user experience:

    # edit/draft a commit message while staging or
    # stage/unstage while drafting a commit message
    ???

    # simple check-in
    git status
    git add .
    git commit -m 'one liner message'
    git push

    # selective add/remove(#footnote-1)
    git status
    git add file_1
    git status
    git diff file_2
    git add -p file_2
    git status
    ...

    # undo all changes to a file, staged or not
    git checkout some_file # tab-completion breaks here
        # can't do this during a merge/revert/rebase,
        # have to do this instead:
    git ls-tree [branch or sub-tree @ revision]
        # (copy the hash of some_file)
    git cat-file blob [hash] > some_file
    git add some_file

    # things that should be automated somehow
    # 
    # (provided you're not offline and github isn't down)
    # 
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

    # remove/unstage 
    git rm --cached .*  # tries to remove . and .. 
                        #   fails with error
                        #   doesn't unstage anything

git cli has an unintuitive ui but you get used to it the more you work with it

git cli actually has features that come close to what I want including
 ∗ git add -i
 ∗ git revert --no-commit
 ∗ git commit --allow-empty-message -m ""
 ∗ git ls-files
 ∗ git cat-file blob [hash of file @ revision] > file_with_merge_conflict

and git cli has features I want to have exactly as-is
 ∗ git add -p
 ∗ git log -p

---
 
#footnote-1:
    yes commits should be "atomic" and represent a single unit of work, ergo
    you should commit after every single atomic change you make
    
    but in practice sometimes you end up doing two or more things at once and
    still want to commit them separately. this requires adding files one at a time
    (or using git add -p to split the work accordingly)

---

Additional nice-to-have features:

 - up-to-date github-style summary:

(master)
3a24901   tso: update todo                   32 minutes ago
commits: 331   branches: 12   releases: 0   contributors: 1
                        Go: 100%

 - better -h and --help
    - should still do the default behavior so subcommand "help" instead
      presents custom docs (of course after you make things like this you tend
      to never need them yourself because you remember...)

help log
     (* list of PRETTY FORMATS first, with less verbosity)
     %h     3a24901         short commit hash
     %cr    3 minutes ago   time
    ...

help remote
    how to list remotes and branches that are tracked
    how to remove remotes
    how to update remote urls and their aliases
    (* I don't actually know how to do any of these)

etc...

 - git grep -n always
