package main

import (
	"bytes"
	"errors"
	"flag"
	"testing"
)

func TestParseArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		bad  bool
	}{
		{[]string{"file.go"}, "file.go", false},
		{[]string{"--", "-file"}, "-file", false},
		{nil, "", false}, {[]string{"--"}, "", false}, {[]string{""}, "", true}, {[]string{"a", "b"}, "", true}, {[]string{"--unknown"}, "", true},
	} {
		var out bytes.Buffer
		got, err := parseArgs(tc.args, &out)
		if (err != nil) != tc.bad || got != tc.want {
			t.Fatalf("%v => %q, %v", tc.args, got, err)
		}
	}
	var out bytes.Buffer
	_, err := parseArgs([]string{"--help"}, &out)
	if !errors.Is(err, flag.ErrHelp) || out.Len() == 0 {
		t.Fatal("help", err)
	}
}
