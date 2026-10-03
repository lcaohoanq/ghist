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
		if (err != nil) != tc.bad || got.path != tc.want {
			t.Fatalf("%v => %+v, %v", tc.args, got, err)
		}
	}
	var out bytes.Buffer
	_, err := parseArgs([]string{"--help"}, &out)
	if !errors.Is(err, flag.ErrHelp) || out.Len() == 0 {
		t.Fatal("help", err)
	}
}

func TestFollowOption(t *testing.T) {
	for _, args := range [][]string{{"--follow"}, {"--follow", "file"}, {"--follow", "--", "-file"}} {
		got, err := parseArgs(args, &bytes.Buffer{})
		if err != nil || !got.follow {
			t.Fatalf("%v: %+v %v", args, got, err)
		}
	}
	got, err := parseArgs([]string{"file"}, &bytes.Buffer{})
	if err != nil || got.follow {
		t.Fatal("fast mode must be the default")
	}
}
