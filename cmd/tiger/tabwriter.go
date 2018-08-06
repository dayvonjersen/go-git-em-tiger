package main

import (
	"fmt"
	"io"
	"strings"
)

type tabwriter struct {
	out io.Writer
	buf []byte
}

func newTabwriter(out io.Writer) *tabwriter {
	return &tabwriter{
		out: out,
		buf: []byte{},
	}
}

func (tw *tabwriter) Write(p []byte) (n int, err error) {
	tw.buf = append(tw.buf, p...)
	return len(p), nil
}

func (tw *tabwriter) Flush() {
	text := string(tw.buf)
	lines := strings.Split(text, "\n")
	max := 0
	for _, ln := range lines {
		if len(ln) > max {
			max = len(ln)
		}
	}
	for _, ln := range lines {
		fmt.Fprint(tw.out, strings.Repeat(" ", (max-len(ln))/2), ln, "\n")
	}
}
