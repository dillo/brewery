package brew

import (
	"context"
	"errors"
	"testing"
)

func TestParseVersion(t *testing.T) {
	for _, tt := range []struct {
		out  string
		want Version
		ok   bool
	}{
		{"Homebrew 7.0.9-108-gf63eb29\n", Version{7, 0, "7.0.9"}, true},
		{"Homebrew 7.1.0\nHomebrew/homebrew-core (git revision abc)\n", Version{7, 1, "7.1.0"}, true},
		{"Homebrew 6.4.2\n", Version{6, 4, "6.4.2"}, true},
		{"Homebrew 10.0.0\n", Version{10, 0, "10.0.0"}, true},
		{"Homebrew 7.2\n", Version{7, 2, "7.2"}, true},
		{"Warning: something\nHomebrew 7.0.1\n", Version{7, 0, "7.0.1"}, true},
		{"Homebrew >=4.3.0 (shallow or no git repository)\n", Version{}, false},
		{"garbage", Version{}, false},
		{"", Version{}, false},
	} {
		got, ok := ParseVersion(tt.out)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseVersion(%q) = %+v, %v; want %+v, %v", tt.out, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTooOld(t *testing.T) {
	for _, tt := range []struct {
		v    Version
		want bool
	}{
		{Version{6, 9, "6.9.0"}, true},
		{Version{7, 0, "7.0.0"}, false},
		{Version{7, 3, "7.3.1"}, false},
		{Version{8, 0, "8.0.0"}, false},
	} {
		if got := tt.v.TooOld(); got != tt.want {
			t.Errorf("%+v.TooOld() = %v, want %v", tt.v, got, tt.want)
		}
	}
}

func TestClientBrewVersion(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{"--version": []byte("Homebrew 7.0.9-108-gf63eb29\n")}})
	if v, err := c.BrewVersion(context.Background()); err != nil || v != (Version{7, 0, "7.0.9"}) {
		t.Errorf("got %+v, %v", v, err)
	}
	c = NewClient(fakeRunner{out: map[string][]byte{"--version": []byte("odd\n")}})
	if _, err := c.BrewVersion(context.Background()); err == nil {
		t.Error("unparseable output: want an error")
	}
	broke := errors.New("exit status 1")
	c = NewClient(fakeRunner{err: map[string]error{"--version": broke}})
	if _, err := c.BrewVersion(context.Background()); !errors.Is(err, broke) {
		t.Errorf("got %v, want brew's failure", err)
	}
}
