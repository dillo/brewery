package main

import (
	"bytes"
	"strings"
	"testing"

	"brewery/internal/brew"
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
