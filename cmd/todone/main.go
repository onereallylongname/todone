// Command todone is a fast, cross-platform task manager: a vim-modal TUI
// for "stay and explore", plus plain CLI subcommands for "fast in and out"
// scripting. See docs/PLAN.md for the full spec.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/onereallylongname/todone/internal/config"
	"github.com/onereallylongname/todone/internal/search"
	"github.com/onereallylongname/todone/internal/store"
	"github.com/onereallylongname/todone/internal/ui"
)

const version = "0.1.3"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "todone: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	// Global flags (--file, --version, --help) can appear before or mixed
	// in with a subcommand's own args; pull them out first.
	fileFlag := ""
	var rest []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--file" && i+1 < len(args):
			fileFlag = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--file="):
			fileFlag = strings.TrimPrefix(args[i], "--file=")
		case args[i] == "--version":
			printVersionFull()
			return nil
		case args[i] == "-v":
			printVersionShort()
			return nil
		case args[i] == "--help":
			printUsageFull()
			return nil
		case args[i] == "-h":
			printUsageShort()
			return nil
		default:
			rest = append(rest, args[i])
		}
	}

	cfg := config.Load()
	path := cfg.ResolveStorePath(fileFlag)

	if len(rest) == 0 {
		return runTUI(cfg, path)
	}

	switch rest[0] {
	case "add":
		return cmdAdd(path, rest[1:])
	case "list":
		return cmdList(cfg, path, rest[1:])
	case "toggle":
		return cmdToggle(path, rest[1:])
	case "show":
		return cmdShow(path, rest[1:])
	case "follow":
		return cmdFollow(path, rest[1:])
	case "tags":
		return cmdTags(path)
	case "help":
		printUsageFull()
		return nil
	default:
		printUsageShort()
		return fmt.Errorf("unknown command %q", rest[0])
	}
}

// printVersionShort is the `-v` output: just the version number, nothing
// else.
func printVersionShort() {
	fmt.Println("todone " + version)
}

// printVersionFull is the `--version` output: version plus the Go
// toolchain/OS/arch it was built with, matching avredit's --version
// format.
func printVersionFull() {
	fmt.Printf("todone v%s\n", version)
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// printUsageShort is the `-h` output: the CLI flags/commands reference
// only.
func printUsageShort() {
	fmt.Print(usageText)
}

// printUsageFull is the `--help` output: the same CLI reference plus the
// full TUI keybinding reference, matching avredit's --help format.
func printUsageFull() {
	fmt.Print(usageText)
	fmt.Println()
	fmt.Print(ui.KeybindingsText())
}

const usageText = `todone — a fast, vim-modal task manager

Usage:
  todone                        launch the interactive TUI
  todone add <text...>          add a task (parses #tags, @links and http(s):// links from text), prints id
                                 escape a literal sigil with \#, \|, or \@
  todone list [--all]           print active (or all) tasks
  todone show <id|query>        print a single task, resolved like toggle (any done-state)
  todone follow <id|query>      open a task's link (default: first) without starting the TUI
                                 --index N / -n N picks a link other than the first
                                 a task-id link prints the target task's row (see list/show output flags below)
  todone toggle <id|query>      toggle done by id (or id prefix), or an unambiguous summary match
  todone tags                   list every tag in use, sorted
  todone -v / --version         print the version (--version also prints Go/OS info)
  todone -h / --help            show usage (--help also prints the TUI keybinding reference)

list/show/follow output flags:
  --format table|json|yaml      table: full aligned columns. json/yaml: same field names as todo.yaml
                                 (default: a compact one-line-per-task row)
                                 for follow, only applies when the link is a task id (nothing to render otherwise)
  --fields name|default|all|f1,f2,...
                                 name: summary only. default: id,done,summary,tags (the compact row's info)
                                 all: every field (default when --format is given without --fields)
                                 or a custom comma list of: id,done,summary,date,updated,tags,links

Flags:
  --file <path>                 override the task data file (also: $TODONE_FILE)

Config: ~/.config/todone/config.json
Data:   ~/.config/todone/todo.yaml (default)
`

// cmdAdd mirrors the TUI's quick-add parsing: any #tag tokens in the text
// are pulled out into Tags, the rest becomes Summary.
func cmdAdd(path string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: todone add <text...>")
	}
	text := strings.Join(args, " ")
	summary, tags, links := parseAddText(text)
	if summary == "" {
		return fmt.Errorf("add: summary cannot be empty")
	}

	st, err := store.Load(path)
	if err != nil {
		return err
	}
	t := st.Add(summary, tags, links)
	if err := st.Save(); err != nil {
		return err
	}
	fmt.Println(t.ID)
	return nil
}

// parseAddText splits free text into a summary, any #tag tokens, and any
// links — either a bare http(s):// URL or an explicit @token (any kind of
// link: URL, filesystem path, or another task's id; classification happens
// lazily when the link is opened, see ui.classifyLink). A word can escape
// its leading sigil with a backslash (\#, \|, \@) to be treated as plain
// summary text instead. Matches the TUI's insert-mode add behavior (see
// ui.parseAddInput).
func parseAddText(text string) (summary string, tags []string, links []string) {
	var words []string
	for _, w := range strings.Fields(text) {
		if lit, escaped := unescapeSigil(w); escaped {
			words = append(words, lit)
			continue
		}
		switch {
		case strings.HasPrefix(w, "#") && len(w) > 1:
			tags = append(tags, w[1:])
		case strings.HasPrefix(w, "@") && len(w) > 1:
			links = append(links, w[1:])
		case strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://"):
			links = append(links, w)
		default:
			words = append(words, w)
		}
	}
	return strings.Join(words, " "), tags, links
}

// unescapeSigil reports whether word starts with a backslash-escaped sigil
// (\#, \|, or \@) and, if so, returns the literal word with the backslash
// stripped — so e.g. "\#1" is treated as plain text "#1" instead of a tag.
// Any other use of "\" passes through unchanged (no support yet for
// escaping a literal backslash itself).
func unescapeSigil(word string) (string, bool) {
	if len(word) >= 2 && word[0] == '\\' {
		switch word[1] {
		case '#', '|', '@':
			return word[1:], true
		}
	}
	return word, false
}

func cmdList(cfg config.Config, path string, args []string) error {
	all := false
	var formatRaw, fieldsRaw string
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--all" || a == "-a":
			all = true
		default:
			rest = append(rest, a)
		}
	}
	var err error
	formatRaw, fieldsRaw, rest, err = extractFormatFieldFlags(rest)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("list: unknown argument %q", rest[0])
	}

	format, fields, err := resolveFormatAndFields(formatRaw, fieldsRaw)
	if err != nil {
		return err
	}

	st, err := store.Load(path)
	if err != nil {
		return err
	}

	q := search.Query{}
	if !all {
		f := false
		q.Done = &f
	}
	sortMode := search.ParseSortMode(cfg.DefaultSort)
	idxs := search.Apply(st.Tasks, q)
	search.SortIndexes(st.Tasks, idxs, sortMode)

	if len(idxs) == 0 && format == "" {
		fmt.Println("No tasks.")
		return nil
	}

	tasks := make([]store.Task, len(idxs))
	for i, idx := range idxs {
		tasks[i] = st.Tasks[idx]
	}
	return renderTasks(tasks, format, fields, false)
}

// formatTaskRow renders one task as a single summary line: "[ ] id  summary  #tag1 #tag2".
// Shared by cmdList and printMatches so an ambiguous-match report looks
// exactly like a `list` row.
func formatTaskRow(t store.Task) string {
	mark := " "
	if t.Done {
		mark = "x"
	}
	tags := ""
	if len(t.Tags) > 0 {
		tags = "  #" + strings.Join(t.Tags, " #")
	}
	return fmt.Sprintf("[%s] %s  %s%s", mark, t.ID, t.Summary, tags)
}

// printMatches prints one formatTaskRow line per index in idxs, used
// whenever a toggle lookup is ambiguous so the user can see exactly what
// matched and retry with a more specific id/query.
func printMatches(st *store.Store, idxs []int) {
	for _, i := range idxs {
		fmt.Println("  " + formatTaskRow(st.Tasks[i]))
	}
}

// cmdToggle toggles a task's done state, resolved by (in order): exact id,
// then an id-prefix match (any done-state — ambiguous prefixes print every
// match and ask for more characters), then an exact case-insensitive
// summary match (any done-state), then an unambiguous substring summary
// match restricted to pending tasks (the original `done` command's
// behavior, preserved as the last, loosest tier).
func cmdToggle(path string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: todone toggle <id|query>")
	}
	needle := strings.Join(args, " ")

	st, err := store.Load(path)
	if err != nil {
		return err
	}

	idx, err := resolveToggleTarget(st, needle)
	if err != nil {
		return err
	}

	if _, ok := st.ToggleDone(idx); !ok {
		return fmt.Errorf("could not toggle task")
	}
	if err := st.Save(); err != nil {
		return err
	}
	state := "done"
	if !st.Tasks[idx].Done {
		state = "pending"
	}
	fmt.Printf("%s: %s\n", state, st.Tasks[idx].Summary)
	return nil
}

// resolveToggleTarget finds the single task needle refers to, or returns
// an error after printing every candidate when the match is ambiguous.
// Tier 4 (loosest) is restricted to pending tasks, matching the original
// `done` command's behavior — toggling is almost always about an active
// task, and this keeps a short query from accidentally reopening an old
// done one.
func resolveToggleTarget(st *store.Store, needle string) (int, error) {
	return resolveTaskTarget(st, needle, true)
}

// resolveShowTarget finds the single task needle refers to, same as
// resolveToggleTarget but with tier 4 matching tasks in any done-state —
// `show`/`follow` are read-only lookups, not "find my next pending task",
// so there's no reason to hide done tasks from the loosest match tier.
func resolveShowTarget(st *store.Store, needle string) (int, error) {
	return resolveTaskTarget(st, needle, false)
}

// resolveTaskTarget is the shared 4-tier resolver behind resolveToggleTarget
// and resolveShowTarget: exact id, then id-prefix (case-insensitive, any
// done-state), then exact summary (case-insensitive, any done-state), then
// substring summary (case-insensitive) — the last tier optionally
// restricted to pending tasks via pendingOnlySubstring. Ambiguous matches
// at any tier print every candidate and return an error so the caller can
// retry with more characters.
func resolveTaskTarget(st *store.Store, needle string, pendingOnlySubstring bool) (int, error) {
	// Tier 1: exact id.
	if idx := st.Index(needle); idx >= 0 {
		return idx, nil
	}

	// Tier 2: id prefix, case-insensitive.
	lower := strings.ToLower(needle)
	var idPrefixMatches []int
	for i, t := range st.Tasks {
		if strings.HasPrefix(strings.ToLower(t.ID), lower) {
			idPrefixMatches = append(idPrefixMatches, i)
		}
	}
	switch len(idPrefixMatches) {
	case 0:
		// fall through to summary matching
	case 1:
		return idPrefixMatches[0], nil
	default:
		fmt.Printf("%q matches %d task ids; use more characters:\n", needle, len(idPrefixMatches))
		printMatches(st, idPrefixMatches)
		return -1, fmt.Errorf("ambiguous id prefix %q", needle)
	}

	// Tier 3: exact summary match, case-insensitive, any done-state.
	var exactSummaryMatches []int
	for i, t := range st.Tasks {
		if strings.EqualFold(t.Summary, needle) {
			exactSummaryMatches = append(exactSummaryMatches, i)
		}
	}
	switch len(exactSummaryMatches) {
	case 0:
		// fall through to substring matching
	case 1:
		return exactSummaryMatches[0], nil
	default:
		fmt.Printf("%q matches %d tasks exactly:\n", needle, len(exactSummaryMatches))
		printMatches(st, exactSummaryMatches)
		return -1, fmt.Errorf("ambiguous summary %q", needle)
	}

	// Tier 4: substring summary match, optionally pending-only.
	var substringMatches []int
	for i, t := range st.Tasks {
		if pendingOnlySubstring && t.Done {
			continue
		}
		if strings.Contains(strings.ToLower(t.Summary), lower) {
			substringMatches = append(substringMatches, i)
		}
	}
	switch len(substringMatches) {
	case 0:
		return -1, fmt.Errorf("no task matches %q", needle)
	case 1:
		return substringMatches[0], nil
	default:
		fmt.Printf("%q matches %d tasks; be more specific or use the id:\n", needle, len(substringMatches))
		printMatches(st, substringMatches)
		return -1, fmt.Errorf("ambiguous query %q", needle)
	}
}

// cmdTags prints every distinct tag in use, one per line, sorted.
func cmdTags(path string) error {
	st, err := store.Load(path)
	if err != nil {
		return err
	}
	tags := store.UniqueSortedTags(st.Tasks)
	if len(tags) == 0 {
		fmt.Println("No tags.")
		return nil
	}
	for _, t := range tags {
		fmt.Println(t)
	}
	return nil
}

func runTUI(cfg config.Config, path string) error {
	st, err := store.Load(path)
	if err != nil {
		return err
	}
	p := tea.NewProgram(ui.New(cfg, st))
	_, err = p.Run()
	return err
}
