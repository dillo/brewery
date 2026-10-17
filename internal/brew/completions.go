package brew

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Completions describes brew's commands for completing a command line. It
// comes from the fish completion script brew ships, which lists every
// command, subcommand and flag with a description, and names the kind of
// argument each command takes.
type Completions struct {
	Commands []Command // sorted by name
}

// Command is one brew command (or a subcommand of one).
type Command struct {
	Name  string
	Desc  string
	Flags []Flag
	Args  []string  // kinds of argument, e.g. "formulae_installed", "services"
	Subs  []Command // subcommands, sorted by name
}

// Flag is a long option such as --dry-run.
type Flag struct {
	Name string // with the leading "--"
	Desc string
}

// Command looks up a command by name.
func (c Completions) Command(name string) (Command, bool) {
	return find(c.Commands, name)
}

// Sub looks up a subcommand by name.
func (c Command) Sub(name string) (Command, bool) {
	return find(c.Subs, name)
}

func find(cmds []Command, name string) (Command, bool) {
	i, ok := slices.BinarySearchFunc(cmds, name, func(c Command, n string) int { return strings.Compare(c.Name, n) })
	if !ok {
		return Command{}, false
	}
	return cmds[i], true
}

// suggestPrefix starts the fish functions that list argument values.
const suggestPrefix = "(__fish_brew_suggest_"

// ParseFish reads brew's fish completion script. Lines it doesn't
// recognise are skipped; when a command is described twice (a tap can
// append its own block), the first description wins. Options only attach
// to commands declared by a __fish_brew_complete_cmd line (developer-only
// commands are declared another way and are left out). brew lists every
// subcommand alias on its own line with its canonical name's description;
// a subcommand whose description matches an earlier one's is skipped, so
// the first-declared name wins. Subcommands are not told apart from aliases
// by anything else in the script, so brew describing `bundle upgrade` the
// same as `bundle install` means `bundle upgrade` is not offered in
// completion (it still runs when typed).
func ParseFish(r io.Reader) (Completions, error) {
	f := fishScript{
		cmds:     map[string]*Command{},
		subs:     map[string]map[string]*Command{},
		seenDesc: map[string]map[string]string{},
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		tok := fishTokens(sc.Text())
		if len(tok) < 2 {
			continue
		}
		switch tok[0] {
		case "__fish_brew_complete_cmd":
			if c := f.cmd(tok[1]); c.Desc == "" && len(tok) > 2 {
				c.Desc = tok[2]
			}
		case "__fish_brew_complete_arg":
			// The first argument is the command, maybe followed by a fish
			// condition ("upgrade; and not __fish_seen_argument …").
			f.cmdArgs = append(f.cmdArgs, tok)
		case "__fish_brew_complete_sub_cmd":
			f.addSub(tok)
		case "__fish_brew_complete_sub_arg":
			f.subArgs = append(f.subArgs, tok)
		}
	}
	if err := sc.Err(); err != nil {
		return Completions{}, err
	}
	return f.completions(), nil
}

// fishScript is what ParseFish has read so far. Options are kept until the
// whole script is read, because they can come before the command they're for.
type fishScript struct {
	cmds     map[string]*Command
	subs     map[string]map[string]*Command // command → subcommand
	seenDesc map[string]map[string]string   // command → description → first name
	cmdArgs  [][]string                     // applied once every command is known
	subArgs  [][]string                     // applied once every subcommand is known
}

// cmd is the command called name, added if it's new.
func (f *fishScript) cmd(name string) *Command {
	if f.cmds[name] == nil {
		f.cmds[name] = &Command{Name: name}
	}
	return f.cmds[name]
}

// addSub records the subcommand on a __fish_brew_complete_sub_cmd line,
// unless it's been seen or an earlier one has its description.
func (f *fishScript) addSub(tok []string) {
	if len(tok) < 3 {
		return
	}
	cmd, name := tok[1], tok[2]
	if f.subs[cmd] == nil {
		f.subs[cmd] = map[string]*Command{}
	}
	if f.subs[cmd][name] != nil {
		return
	}
	desc := ""
	if len(tok) > 3 {
		desc = tok[3]
	}
	if desc != "" && !f.firstWith(cmd, desc, name) {
		return // same description as the first: an alias, or a real subcommand folded into it (bundle upgrade)
	}
	f.subs[cmd][name] = &Command{Name: name, Desc: desc}
}

// firstWith reports whether name is the first subcommand of cmd described
// as desc, and remembers it if it is.
func (f *fishScript) firstWith(cmd, desc, name string) bool {
	if f.seenDesc[cmd] == nil {
		f.seenDesc[cmd] = map[string]string{}
	}
	if first, dup := f.seenDesc[cmd][desc]; dup && first != name {
		return false
	}
	f.seenDesc[cmd][desc] = name
	return true
}

// completions attaches the options to their commands and subcommands and
// sorts everything by name.
func (f *fishScript) completions() Completions {
	for _, tok := range f.cmdArgs {
		name, _, _ := strings.Cut(tok[1], ";")
		if c := f.cmds[strings.TrimSpace(name)]; c != nil {
			addOptions(c, tok[2:])
		}
	}
	for _, tok := range f.subArgs {
		if len(tok) < 3 {
			continue
		}
		// One line can apply to several subcommands and their aliases:
		// 'start launch load s l'. Only described subcommands are kept.
		names, _, _ := strings.Cut(tok[2], ";")
		for name := range strings.FieldsSeq(names) {
			if s := f.subs[tok[1]][name]; s != nil {
				addOptions(s, tok[3:])
			}
		}
	}
	var out Completions
	for name, c := range f.cmds {
		for _, s := range f.subs[name] {
			c.Subs = append(c.Subs, *s)
		}
		sortByName(c.Subs)
		out.Commands = append(out.Commands, *c)
	}
	sortByName(out.Commands)
	return out
}

// addOptions reads "-l flag -d desc" and "-a '(__fish_brew_suggest_kind)'"
// from the rest of a completion line.
func addOptions(c *Command, opts []string) {
	var long, desc, arg string
	for i := 0; i+1 < len(opts); i++ {
		switch opts[i] {
		case "-l":
			long = opts[i+1]
		case "-d":
			desc = opts[i+1]
		case "-a":
			arg = opts[i+1]
		default:
			continue
		}
		i++
	}
	if long != "" && !slices.ContainsFunc(c.Flags, func(f Flag) bool { return f.Name == "--"+long }) {
		c.Flags = append(c.Flags, Flag{Name: "--" + long, Desc: desc})
	}
	if kind, ok := strings.CutPrefix(arg, suggestPrefix); ok {
		kind = strings.TrimSuffix(kind, ")")
		if !slices.Contains(c.Args, kind) {
			c.Args = append(c.Args, kind)
		}
	}
}

func sortByName(cmds []Command) {
	slices.SortFunc(cmds, func(a, b Command) int { return strings.Compare(a.Name, b.Name) })
}

// fishTokens splits a fish command line into words, undoing quotes and
// backslash escapes.
func fishTokens(line string) []string {
	var toks []string
	var cur strings.Builder
	inTok, quote, escaped := false, rune(0), false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped, inTok = true, true
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote, inTok = r, true
		case r == ' ' || r == '\t':
			if inTok {
				toks = append(toks, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	if inTok {
		toks = append(toks, cur.String())
	}
	return toks
}

// Completions reads the fish completion script from brew's repository.
func (c *Client) Completions(ctx context.Context) (Completions, error) {
	out, err := c.query(ctx, "--repository")
	if err != nil {
		return Completions{}, err
	}
	f, err := os.Open(filepath.Join(strings.TrimSpace(string(out)), "completions", "fish", "brew.fish"))
	if err != nil {
		return Completions{}, err
	}
	defer f.Close()
	return ParseFish(f)
}
