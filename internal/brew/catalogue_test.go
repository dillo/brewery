package brew

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCatalogue(t *testing.T) {
	es := ParseCatalogue(string(fixture(t, "catalogue.txt")))
	if len(es) != 17 {
		t.Fatalf("got %d entries, want 17", len(es))
	}
	want := map[string]Entry{
		"abcl":           {"abcl", "Armed Bear Common Lisp: a full implementation of Common Lisp", KindFormula},
		"wget":           {"wget", "Internet file retriever", KindFormula},
		"0-ad":           {"0-ad", "Real-time strategy game", KindCask},
		"ape":            {"ape", "Software for DNA sequence analysis and annotation", KindCask},
		"font-fira-code": {"font-fira-code", "Fira Code", KindCask}, // no description: the display name says what it is
	}
	for _, e := range es {
		if w, ok := want[e.Name]; ok && e != w {
			t.Errorf("got %+v, want %+v", e, w)
		}
	}
}

func TestParseCatalogueSkipsNoise(t *testing.T) {
	out := "Warning: something\nno-colon line\n==> Formulae\nfd: Find\n\nbad name: x\n"
	if got, want := ParseCatalogue(out), []Entry{{"fd", "Find", KindFormula}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestClientCatalogue(t *testing.T) {
	c := NewClient(fakeRunner{out: map[string][]byte{"search --desc ": fixture(t, "catalogue.txt")}})
	es, err := c.Catalogue(context.Background())
	if err != nil || len(es) != 17 {
		t.Errorf("got %d entries, %v", len(es), err)
	}
}

func TestNewSinceUpdate(t *testing.T) {
	cache := t.TempDir()
	write := func(name, body string) {
		if err := os.MkdirAll(filepath.Join(cache, "api"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, "api", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("formula_names.before.txt", "fd\ngit\nopenssl@3.5\n")
	write("formula_names.txt", "fd\ngit\nopenssl@3.6\nportop\n")
	write("cask_names.before.txt", "firefox\nold-app\n")
	write("cask_names.txt", "firefox\nnew-app\n")
	c := NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(cache + "\n")}})
	got, err := c.NewSinceUpdate(context.Background())
	want := []Entry{{Name: "openssl@3.6", Kind: KindFormula}, {Name: "portop", Kind: KindFormula}, {Name: "new-app", Kind: KindCask}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, %v; want %+v", got, err, want)
	}

	// Before brew's first update with the lists, there is nothing to compare.
	c = NewClient(fakeRunner{out: map[string][]byte{"--cache": []byte(t.TempDir())}})
	if got, err := c.NewSinceUpdate(context.Background()); err != nil || len(got) != 0 {
		t.Errorf("without the lists: got %+v, %v", got, err)
	}
}
