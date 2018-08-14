package main

import (
	"io"
	"os"
	"strings"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func fileExists(filename string) bool {
	f, err := os.Open(filename)
	f.Close()
	if os.IsNotExist(err) {
		return false
	}
	checkErr(err)
	return true
}

func fileGetContents(filename string) string {
	contents := &buf{}
	f, err := os.Open(filename)
	checkErr(err)
	_, err = io.Copy(contents, f)
	f.Close()
	if err != io.EOF {
		checkErr(err)
	}
	return contents.String()
}

// I'm getting both / and \ as path separators using Git Bash for Windows...
func normalizePathSeparators(path string) string {
	return strings.Replace(path, "\\", "/", -1)
}

func isDir(filename string) bool {
	finfo, err := os.Stat(filename)
	checkErr(err)
	return finfo.IsDir()
}
