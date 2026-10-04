# todone

A fast, cross-platform task manager with a vim-modal TUI for "stay and
explore," plus plain CLI subcommands for "fast in and out" scripting.

This is another AI assisted tool creation. An yes it is just another TODO App.
This one doesn't even sync to the cloud, but that is by design.
It is AI friendly, the database(s) is(are) just yaml files. Easy to read and
fix manually if needed.

## Install / Build

```sh
go install  github.com/onereallylongname/todone/cmd/todone@latest
```

or

```sh
go build -o todone ./cmd/todone
```

Requires Go 1.23+. No other runtime dependencies — a single static binary.

## Usage

### CLI (fast in/out)

```sh
todone                        # launch the interactive TUI
todone add <text...>          # add a task (parses #tags/@links from text, \#/\|/\@ escape literals), prints its id
todone list [--all] [--format table|json|yaml] [--fields <preset|list>]
                               # print active (or all) tasks, exits
todone show <id|query>        # print exactly one task (any-state match), same --format/--fields as list
todone follow <id|query> [--index N] [--format table|json|yaml] [--fields <preset|list>]
                               # open a task's link without starting the TUI (task-id links print the target task instead,
                               # honoring --format/--fields; path/URL links ignore them — there's nothing to render)
todone toggle <id|query>      # toggle done by id (or id prefix), or an unambiguous summary match
todone tags                   # list every tag in use, sorted, exits
todone -v / --version         # -v: version only. --version: version + Go version + OS/Arch
todone -h / --help            # -h: usage only. --help: usage + the full TUI keybinding reference
```

`--format` is `table` (default once either flag is given), `json`, or
`yaml`. `--fields` is a preset (`name`, `default`, `all`) or a custom
comma-separated list (e.g. `id,summary,tags`); output order is always the
canonical `id, done, summary, date, updated, tags, links` order. With
neither flag, `list`/`follow` keep their original compact row output.

```sh
$ todone add "Fix the pricing page #web #bug @https://example.com/ticket/42"
6e4033d056
$ todone list
[ ] 6e4033d056  Fix the pricing page  #web #bug
$ todone show pricing
[ ] 6e4033d056  Fix the pricing page  #web #bug
$ todone show pricing --format json --fields id,summary,links
{"id":"6e4033d056","links":["https://example.com/ticket/42"],"summary":"Fix the pricing page"}
$ todone follow pricing
opened link: https://example.com/ticket/42
$ todone toggle pricing
done: Fix the pricing page
$ todone tags
bug
web
```

### TUI (stay and explore)

Running `todone` with no arguments launches the full-screen interactive
app. Interactive-only features (yank-to-clipboard, open-link, live filter,
undo) are TUI-only by design — scripting those doesn't make sense.

## Keybindings

### Global

| Key | Action |
| --- | --- |
| `q` | Quit |
| `?` | Help overlay |
| `Esc` | Cancel current mode / close overlay |
| `Ctrl+C` | No-op — hints "use q or :q to quit" (never force-quits, like vim) |
| `/` | Enter search |
| `:` | Enter command mode |

The status bar is always pinned to the last line:
`mode | file | counts/sort/filter ... help/quit`. The file segment shows
the in-use data file abbreviated under `$HOME` (`~/...`), degrading to
just the basename if the terminal is too narrow for the full path.

### List (Normal mode)

| Key | Action |
| --- | --- |
| `j`/`k`, `↓`/`↑` | Move selection |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D`/`Ctrl+U`, `f`/`b` | Half-page down/up |
| `Space` | Toggle done |
| `Enter` | Open detail/edit view for selected task |
| `a` | Add task (inline prompt; `#tag`, `@link`, and `http(s)://` tokens become tags/links; `\#`/`\|`/`\@` escape a literal character) |
| `d` | Delete selected (instant, `u` undoes) |
| `u` / `Ctrl+R` | Undo / redo |
| `y` then `y`/`l`/`i` | Yank summary / links / id to clipboard |
| `o` | Open link — see **Link resolution** below |
| `Ctrl+O` / `Ctrl+I` | Jump back / forward after a task-id link jump |
| `A` | Toggle show-done / active-only |
| `s` | Cycle sort: updated → date → alphabetical |

### Detail view (opened with `Enter`)

Tags and Links are real editable lists here.

| Key | Action |
| --- | --- |
| `j`/`k` | Move between rows |
| `H`/`L` | Next / prev task (respects active filter + sort) |
| `J`/`K` | Move selected Tags/Links item up/down |
| `e`/`Enter` | Edit selected row |
| `a` | Add a Tag or Link item |
| `d` | Delete selected item (instant, `u` undoes) |
| `y` | Yank row value |
| `o` | Open link — see **Link resolution** below |
| `Ctrl+O` / `Ctrl+I` | Jump back / forward after a task-id link jump |
| `u` / `Ctrl+R` | Undo / redo |
| `Esc`/`q` | Back to list |

### Insert mode (add/edit)

| Key | Action |
| --- | --- |
| `Enter` | Confirm |
| `Esc` | Cancel |
| `←`/`→`, `Home`/`End`, `Backspace`/`Delete` | Standard line editing |
| `Ctrl+A`/`Ctrl+E`/`Ctrl+U`/`Ctrl+K`/`Ctrl+W` | Readline-style editing |
| `Ctrl+V` / `Shift+Insert` | Paste from clipboard |
| `Tab` | Complete a tag (Tags-list field, or a trailing `#tag` token in free text) |

### Link resolution (`o` / `todone follow`)

A Link is classified in order: **task id** (jumps to that task in the TUI —
stays in list or detail view, whichever you're in; clears the filter if the
target is hidden) → **existing filesystem path** (opened with the OS's
default handler) → **URL** → otherwise reported as an error, nothing is
shelled out. Typing/editing a link that resolves to a real file/directory
is stored as an absolute path immediately, so it can't break later just
because todone was launched from a different directory. The CLI's `todone
follow` command uses this same classification (`internal/linkkind`): since
there's no TUI to jump into, a task-id link just prints the target task's
row instead.

### Search (`/`) prefixes

| Prefix | Filters by | Example |
| --- | --- | --- |
| `t:` or `#` | Tag (substring, tab-completes) | `t:pricing` / `#pricing` |
| `d:` or `s:` | Status (`done`/`pending`) | `d:done` |
| `@` | Link (substring, including task-id links) | `@example.com` |
| *(none)* | Fuzzy match on summary | `kafka client` |
| `a \| b` | OR: matches `a` OR `b` (filters still AND) | `int \| work #now` |
| `\#` `\|` `\@` | Escape — literal `#`/`\|`/`@` in free text | `fix \#1 on call` |

Tokens are space-separated and compose (AND'ed): `#kafka d:pending retry`.
A bare `|` splits free text into OR'd alternatives instead — a task
matching more than one branch is ranked by whichever scores it best.

### Command mode (`:`)

| Command | Action |
| --- | --- |
| `:w` | Force save |
| `:q` / `:q!` | Quit |
| `:wq` | Save and quit |
| `:sort date\|updated\|alpha` | Explicit sort |
| `:all` | Toggle show-done |
| `:tags` | Overlay listing every distinct tag in use |

## Configuration

`~/.config/todone/config.json`:

```json
{
  "store": "",
  "default_sort": "updated",
  "show_done": false
}
```

| Field | Description | Default |
| --- | --- | --- |
| `store` | Path to the YAML task file | `~/.config/todone/todo.yaml` |
| `default_sort` | `updated` \| `date` \| `alpha` | `updated` |
| `show_done` | Start with done tasks visible | `false` |

Data file resolution order: `--file` flag > `$TODONE_FILE` env var > `store`
in config > default (`~/.config/todone/todo.yaml`).

### Theming (optional)

`~/.config/todone/theme.json` — colors only, any field left out keeps its
built-in default:

```json
{
  "primary": "#7aa2f7",
  "accent": "183"
}
```

Fields: `fg`, `dim`, `muted`, `primary`, `success`, `warning`, `error`,
`accent`, `contrast_fg`. Values are hex (`"#7aa2f7"`) or ANSI-256 index
(`"75"`). See [`docs/PLAN.md`](docs/PLAN.md) for what each field controls.

Eleven ready-made palettes (ported from `avredit`) are in
[`themes/`](themes) — just copy one:

```sh
cp themes/dracula.json ~/.config/todone/theme.json
```

## Data format

Tasks are stored as a flat YAML list, compatible with the original
`todo.nu` record shape:

```yaml
- id: 6e4033d056
  done: false
  summary: Fix the pricing page
  date: "2024-01-15T10:30:00Z"
  updated: ""
  tags: [web, bug]
  links: []
```

Writes are atomic (temp file + rename) to avoid the whole-file-rewrite
corruption risk present in the original script.

## Exporting and importing data

To export just copy the todo.yaml file. To import just point to another todo.yaml, using the `--file` switch, or just concat the new file to your todo.yaml.

---

# Development

```sh
go build ./...
go vet ./...
go test ./...
```

`todo.nu` is left untouched — `todone` is an independent, parallel tool.

## Some more details

Built in Go with [Bubble Tea v2](https://charm.land/bubbletea) +
[Lipgloss v2](https://charm.land/lipgloss), sharing conventions with its
sibling project [`avredit`](../avredit). See [`docs/PLAN.md`](docs/PLAN.md)
for the full design rationale, prior-art comparison, and a catalog of the
UX flaws found in the original `todo.nu` Nushell script that motivated this
rewrite.
