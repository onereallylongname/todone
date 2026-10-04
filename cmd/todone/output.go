package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/onereallylongname/todone/internal/linkkind"
	"github.com/onereallylongname/todone/internal/opener"
	"github.com/onereallylongname/todone/internal/store"
	"gopkg.in/yaml.v3"
)

// openFunc opens a resolved link with the OS's default handler. It's a
// package variable (rather than calling opener.Open directly) so tests can
// stub it out instead of actually shelling out to xdg-open/open/start.
var openFunc = opener.Open

// allFieldKeys is every selectable task field, in the canonical order used
// for table columns and custom --fields lists, regardless of the order the
// user typed them in.
var allFieldKeys = []string{"id", "done", "summary", "date", "updated", "tags", "links"}

// presetFields resolves a --fields preset name to its field list: "name"
// is just the summary (closest to "what is this task"), "default" mirrors
// today's compact list row (id/done/summary/tags, no links/dates),
// "all"/"" is every field.
func presetFields(name string) ([]string, bool) {
	switch name {
	case "name":
		return []string{"summary"}, true
	case "default":
		return []string{"id", "done", "summary", "tags"}, true
	case "all", "":
		return append([]string(nil), allFieldKeys...), true
	}
	return nil, false
}

// parseFields parses a --fields value (a preset name, or a comma-separated
// custom field list) into a canonically-ordered, validated field list.
func parseFields(raw string) ([]string, error) {
	if preset, ok := presetFields(raw); ok {
		return preset, nil
	}
	want := map[string]bool{}
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		valid := false
		for _, vk := range allFieldKeys {
			if vk == k {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("unknown field %q (valid: %s, or a preset: name, default, all)", k, strings.Join(allFieldKeys, ", "))
		}
		want[k] = true
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("--fields: no valid fields given")
	}
	var out []string
	for _, k := range allFieldKeys {
		if want[k] {
			out = append(out, k)
		}
	}
	return out, nil
}

// parseFormat validates a --format value.
func parseFormat(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "table", "json", "yaml":
		return strings.ToLower(raw), nil
	default:
		return "", fmt.Errorf("unknown --format %q (valid: table, json, yaml)", raw)
	}
}

// resolveFormatAndFields turns the raw --format/--fields flag values into
// a format ("" meaning the legacy compact-row default) and a resolved
// field list. --fields without an explicit --format implies --format=table
// (the legacy row has fixed columns and no field-selection concept).
func resolveFormatAndFields(formatRaw, fieldsRaw string) (format string, fields []string, err error) {
	if formatRaw != "" {
		format, err = parseFormat(formatRaw)
		if err != nil {
			return "", nil, err
		}
	} else if fieldsRaw != "" {
		format = "table"
	}

	if fieldsRaw != "" {
		fields, err = parseFields(fieldsRaw)
		if err != nil {
			return "", nil, err
		}
	} else {
		fields, _ = presetFields("all")
	}
	return format, fields, nil
}

// renderTasks prints tasks in the requested format. format=="" is the
// legacy default: one formatTaskRow line per task, unaffected by fields.
// single indicates a one-task `show` lookup, so json/yaml marshal a single
// object instead of a one-element array.
func renderTasks(tasks []store.Task, format string, fields []string, single bool) error {
	switch format {
	case "":
		for _, t := range tasks {
			fmt.Println(formatTaskRow(t))
		}
		return nil
	case "table":
		return renderTable(tasks, fields)
	case "json":
		return renderJSON(tasks, fields, single)
	case "yaml":
		return renderYAML(tasks, fields, single)
	default:
		return fmt.Errorf("unknown format %q", format)
	}
}

func fieldHeader(f string) string {
	switch f {
	case "id":
		return "ID"
	case "done":
		return "DONE"
	case "summary":
		return "SUMMARY"
	case "date":
		return "DATE"
	case "updated":
		return "UPDATED"
	case "tags":
		return "TAGS"
	case "links":
		return "LINKS"
	default:
		return strings.ToUpper(f)
	}
}

func fieldValue(t store.Task, f string) string {
	switch f {
	case "id":
		return t.ID
	case "done":
		if t.Done {
			return "x"
		}
		return ""
	case "summary":
		return t.Summary
	case "date":
		return t.Date
	case "updated":
		return t.Updated
	case "tags":
		if len(t.Tags) == 0 {
			return ""
		}
		return "#" + strings.Join(t.Tags, " #")
	case "links":
		return strings.Join(t.Links, ", ")
	default:
		return ""
	}
}

// renderTable prints a fully aligned, headered table of tasks restricted
// to the selected fields, via text/tabwriter (stdlib, no new dependency).
func renderTable(tasks []store.Task, fields []string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	headers := make([]string, len(fields))
	for i, f := range fields {
		headers[i] = fieldHeader(f)
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, t := range tasks {
		row := make([]string, len(fields))
		for i, f := range fields {
			row[i] = fieldValue(t, f)
		}
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

// taskFieldMap builds a map of only the selected fields, keyed exactly
// like todo.yaml's own field names so JSON/YAML CLI output needs no
// translation layer. nil Tags/Links marshal as "[]" rather than "null".
func taskFieldMap(t store.Task, fields []string) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		switch f {
		case "id":
			m["id"] = t.ID
		case "done":
			m["done"] = t.Done
		case "summary":
			m["summary"] = t.Summary
		case "date":
			m["date"] = t.Date
		case "updated":
			m["updated"] = t.Updated
		case "tags":
			tags := t.Tags
			if tags == nil {
				tags = []string{}
			}
			m["tags"] = tags
		case "links":
			links := t.Links
			if links == nil {
				links = []string{}
			}
			m["links"] = links
		}
	}
	return m
}

func renderJSON(tasks []store.Task, fields []string, single bool) error {
	var v any
	if single {
		if len(tasks) == 0 {
			return fmt.Errorf("renderJSON: no task to render")
		}
		v = taskFieldMap(tasks[0], fields)
	} else {
		out := make([]map[string]any, len(tasks))
		for i, t := range tasks {
			out[i] = taskFieldMap(t, fields)
		}
		v = out
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func renderYAML(tasks []store.Task, fields []string, single bool) error {
	var v any
	if single {
		if len(tasks) == 0 {
			return fmt.Errorf("renderYAML: no task to render")
		}
		v = taskFieldMap(tasks[0], fields)
	} else {
		out := make([]map[string]any, len(tasks))
		for i, t := range tasks {
			out[i] = taskFieldMap(t, fields)
		}
		v = out
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Print(string(b))
	return nil
}

// cmdShow prints a single task, resolved the same way as toggle (exact id
// -> id prefix -> exact summary -> substring), except the loosest tier
// matches tasks in any done-state — `show` is a read-only lookup, not
// "find my next pending task".
func cmdShow(path string, args []string) error {
	formatRaw, fieldsRaw, words, err := extractFormatFieldFlags(args)
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return fmt.Errorf("usage: todone show <id|query> [--format table|json|yaml] [--fields name|default|all|f1,f2,...]")
	}
	needle := strings.Join(words, " ")

	format, fields, err := resolveFormatAndFields(formatRaw, fieldsRaw)
	if err != nil {
		return err
	}

	st, err := store.Load(path)
	if err != nil {
		return err
	}
	idx, err := resolveShowTarget(st, needle)
	if err != nil {
		return err
	}
	return renderTasks([]store.Task{st.Tasks[idx]}, format, fields, true)
}

// extractFormatFieldFlags pulls --format/--format=.../--fields/--fields=...
// out of args (in any position), returning their raw values plus the
// remaining positional words untouched and in order.
func extractFormatFieldFlags(args []string) (formatRaw, fieldsRaw string, words []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--format" && i+1 < len(args):
			formatRaw = args[i+1]
			i++
		case strings.HasPrefix(a, "--format="):
			formatRaw = strings.TrimPrefix(a, "--format=")
		case a == "--fields" && i+1 < len(args):
			fieldsRaw = args[i+1]
			i++
		case strings.HasPrefix(a, "--fields="):
			fieldsRaw = strings.TrimPrefix(a, "--fields=")
		case a == "--format" || a == "--fields":
			return "", "", nil, fmt.Errorf("%s: missing value", a)
		default:
			words = append(words, a)
		}
	}
	return formatRaw, fieldsRaw, words, nil
}

// cmdFollow resolves a task (same rules as show) and opens one of its
// links without starting the TUI — the CLI equivalent of the 'o'
// keybinding. --index/-n (1-based) picks a link other than the first when
// a task has several; the default mirrors the TUI's "default link" (index
// 0 / first). --format/--fields are accepted like `show`, but only affect
// a task-id link's target-task print — a path/URL link has no task to
// render, so the flags are simply unused (not an error) for those.
func cmdFollow(path string, args []string) error {
	formatRaw, fieldsRaw, rest, err := extractFormatFieldFlags(args)
	if err != nil {
		return err
	}
	format, fields, err := resolveFormatAndFields(formatRaw, fieldsRaw)
	if err != nil {
		return err
	}

	index := 1
	var words []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case (a == "--index" || a == "-n") && i+1 < len(rest):
			n, convErr := strconv.Atoi(rest[i+1])
			if convErr != nil || n < 1 {
				return fmt.Errorf("follow: --index must be a positive integer")
			}
			index = n
			i++
		case strings.HasPrefix(a, "--index="):
			n, convErr := strconv.Atoi(strings.TrimPrefix(a, "--index="))
			if convErr != nil || n < 1 {
				return fmt.Errorf("follow: --index must be a positive integer")
			}
			index = n
		case a == "--index" || a == "-n":
			return fmt.Errorf("%s: missing value", a)
		default:
			words = append(words, a)
		}
	}
	if len(words) == 0 {
		return fmt.Errorf("usage: todone follow <id|query> [--index N] [--format table|json|yaml] [--fields name|default|all|f1,f2,...]")
	}
	needle := strings.Join(words, " ")

	st, err := store.Load(path)
	if err != nil {
		return err
	}
	idx, err := resolveShowTarget(st, needle)
	if err != nil {
		return err
	}
	t := st.Tasks[idx]
	if len(t.Links) == 0 {
		return fmt.Errorf("follow: %q has no links", t.Summary)
	}
	if index > len(t.Links) {
		return fmt.Errorf("follow: %q only has %d link(s), --index %d is out of range", t.Summary, len(t.Links), index)
	}
	link := t.Links[index-1]

	kind, resolved := linkkind.Classify(link, st)
	switch kind {
	case linkkind.TaskID:
		target := st.Index(resolved)
		if target < 0 {
			return fmt.Errorf("follow: linked task %q not found", resolved)
		}
		// No TUI to "jump" in — print the linked task, same as `show`,
		// honoring --format/--fields the same way.
		return renderTasks([]store.Task{st.Tasks[target]}, format, fields, true)
	case linkkind.Path:
		if err := openFunc(resolved); err != nil {
			return fmt.Errorf("follow: open failed: %w", err)
		}
		fmt.Println("opened path: " + resolved)
		return nil
	case linkkind.URL:
		if err := openFunc(resolved); err != nil {
			return fmt.Errorf("follow: open failed: %w", err)
		}
		fmt.Println("opened link: " + resolved)
		return nil
	default:
		return fmt.Errorf("follow: %q is not a recognizable link", link)
	}
}
