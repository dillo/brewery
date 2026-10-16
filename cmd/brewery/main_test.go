package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/brew"
)

func TestVersionFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--version"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if got := out.String(); got != "brewery dev\n" {
		t.Errorf("got %q", got)
	}
}

func TestBrewMissing(t *testing.T) {
	locateBrew = func() (string, error) { return "", brew.ErrNotFound }
	t.Cleanup(func() { locateBrew = brew.Locate })
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "https://brew.sh") {
		t.Errorf("stderr should point to brew.sh: %q", errb.String())
	}
}

func TestUnknownFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--nope"}, &out, &errb); code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
}

func TestWantIcons(t *testing.T) {
	for _, tt := range []struct {
		flag bool
		env  string
		want bool
	}{
		{false, "", true},
		{true, "", false},
		{false, "1", false},
	} {
		if got := wantIcons(tt.flag, tt.env); got != tt.want {
			t.Errorf("wantIcons(%v, %q) = %v", tt.flag, tt.env, got)
		}
	}
}

func TestNoIconsFlagParses(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--no-icons", "--version"}, &out, &errb); code != 0 {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
}

func TestBrewTooOld(t *testing.T) {
	locateBrew = func() (string, error) { return "/opt/homebrew/bin/brew", nil }
	brewVersion = func(string) (brew.Version, error) { return brew.Version{Major: 6, Minor: 4, Text: "6.4.2"}, nil }
	t.Cleanup(func() { locateBrew, brewVersion = brew.Locate, readBrewVersion })
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if want := "brewery needs Homebrew 7.0 or newer (you have 6.4.2). Update it with `brew update`.\n"; errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
}

func TestVersionProblem(t *testing.T) {
	t.Cleanup(func() { brewVersion = readBrewVersion })
	for _, tt := range []struct {
		v    brew.Version
		err  error
		want string
	}{
		{brew.Version{Major: 7, Minor: 0, Text: "7.0.9"}, nil, ""},
		{brew.Version{Major: 8, Minor: 1, Text: "8.1.0"}, nil, ""},
		{brew.Version{}, errors.New("can't read the version"), ""}, // unknown: start anyway
		{brew.Version{Major: 6, Minor: 9, Text: "6.9.1"}, nil, "brewery needs Homebrew 7.0 or newer (you have 6.9.1). Update it with `brew update`."},
	} {
		brewVersion = func(string) (brew.Version, error) { return tt.v, tt.err }
		if got := versionProblem("/opt/homebrew/bin/brew"); got != tt.want {
			t.Errorf("%+v, %v: got %q, want %q", tt.v, tt.err, got, tt.want)
		}
	}
}
