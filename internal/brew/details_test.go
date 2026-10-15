package brew

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestClientDetails(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"info --json=v2 --formula ripgrep": fixture(t, "details-ripgrep.json"),
		"deps --formula ripgrep":           []byte("pcre2\n"),
	}})
	d, err := c.Details(context.Background(), "ripgrep", KindFormula)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "ripgrep" || d.Latest != "15.2.0" || d.Version != "" || d.License != "Unlicense" || d.Homepage != "https://github.com/BurntSushi/ripgrep" {
		t.Errorf("got %+v", d.Package)
	}
	if !reflect.DeepEqual(d.Needs, []string{"pcre2"}) {
		t.Errorf("needs %v", d.Needs)
	}
}

func TestClientDetailsCask(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{
		"info --json=v2 --cask firefox": []byte(`{"formulae":[],"casks":[{"token":"firefox","full_token":"firefox","tap":"homebrew/cask","name":["Mozilla Firefox"],"desc":"Web browser","homepage":"https://www.mozilla.org/firefox/","version":"158.0","installed":null}]}`),
	}})
	d, err := c.Details(context.Background(), "firefox", KindCask)
	if err != nil || d.Kind != KindCask || d.Latest != "158.0" || len(d.Needs) != 0 {
		t.Errorf("got %+v, %v", d, err)
	}
}

func TestClientDetailsFails(t *testing.T) {
	broke := errors.New("exit status 1")
	c := NewClient(fakeRunner{
		out: map[string][]byte{"info --json=v2 --formula nope": []byte(`{"formulae":[],"casks":[]}`)},
		err: map[string]error{"deps --formula gone": broke, "info --json=v2 --formula gone": broke},
	})
	if _, err := c.Details(context.Background(), "nope", KindFormula); err == nil {
		t.Error("brew found nothing: want an error")
	}
	if _, err := c.Details(context.Background(), "gone", KindFormula); !errors.Is(err, broke) {
		t.Errorf("got %v, want brew's failure", err)
	}
}
