package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	for _, test := range []struct {
		input    string
		expected []string
	}{
		{
			input:    `git commit -m 'this is a test'`,
			expected: []string{"git", "commit", "-m", "this is a test"},
		},
		{
			input:    `git commit -m 'this isn\'t a test'`,
			expected: []string{"git", "commit", "-m", "this isn't a test"},
		},
		{
			input:    `\`,
			expected: []string{`\`},
		},
		{
			input:    `\\`,
			expected: []string{`\`},
		},
		{
			input:    `\\\`,
			expected: []string{`\\`},
		},
		{
			input:    `\\\\`,
			expected: []string{`\\`},
		},
		{
			input:    `\\\\\`,
			expected: []string{`\\\`},
		},
		{
			input:    `\\\\\\`,
			expected: []string{`\\\`},
		},

		{
			input:    `\\\\\\\`,
			expected: []string{`\\\\`},
		},
		{
			input:    `\\\\\\\\`,
			expected: []string{`\\\\`},
		},
	} {
		actual := splitArgs(test.input)
		if !reflect.DeepEqual(test.expected, actual) {
			fmt.Printf("input:    %#v\n", test.input)
			fmt.Printf("expected: %#v\n", test.expected)
			fmt.Printf("actual:   %#v\n", actual)
			t.FailNow()
		}
	}
}
