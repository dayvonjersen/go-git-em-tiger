package main

import (
	"io"
	"os"
	"os/exec"
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
	// shoutouts to bradfitz for a post on golang-nuts from 2012
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
