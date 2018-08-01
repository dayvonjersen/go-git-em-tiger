package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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

func git(args ...string) string {
	stdout, stderr, err := cmd("git", args...)
	if err != nil {
		fmt.Println(Red+"ERROR:"+Reset, err)
	}
	return stdout + stderr
}

func prompt() {
	fmt.Println(git("status", "-s", "-uall"))
	fmt.Print("git> ")
}

func main() {
	prompt()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		args := strings.Split(scanner.Text(), " ")
		fmt.Println(git(args...))
		prompt()
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("error reading stdin:", err)
	}
}
