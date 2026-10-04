# todone — Plan & Knowledge Base (v1)

## 🎯 Goal

Replace the day-to-day interactive parts of the Nushell `todo.nu` module with
`todone`: a single, fast, cross-platform binary (Go + Bubble Tea v2 +
Lipgloss v2) that can be used two ways:

1. **Fast in/out** — `todone add "..."`, `todone done <id>`, `todone list`
   run as plain CLI commands, print, and exit. No TUI startup cost.
2. **Stay and explore** — running `todone` with no args launches a
   lazygit-style TUI: live fuzzy filter, tag/status prefixes, inline edit,
   vim-like navigation, yank-to-clipboard, open links.

`todo.nu` is left untouched (`parallel` strategy, see
[Decisions](#technical-decisions)) — this is a new, independent tool that
happens to be able to read/write the same record shape.

### Prior art consulted

| Project | Relevance | Takeaway |
|---|---|---|
| [`avredit`](../../avredit) (same author) | Go + Bubble Tea v2 + Lipgloss v2 TUI, vim-modal, `~/.config` conventions | Reused directly: `Editor`, `Confirm`, `StatusBar` components, `AppMode` dispatch pattern, `config.go` shape, `clipboard.go` shelling-out approach |
| [`todui`](https://github.com/Develonaut/todui) | Near-identical pitch (Go+Bubbletea TUI+CLI over a plain file) found *after* scoping this | Confirms the approach is sound; we differ by keeping YAML (today's schema), vim-modal keys + avredit-style search prefixes instead of arrow/section navigation, and single-level undo instead of a goal/section system — kept intentionally smaller in scope |

---

## 🔧 Technical Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Language/framework | Go + `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` | Matches `avredit`; trivial cross-compilation (`GOOS`/`GOARCH`, no cross toolchain); Elm architecture fits modal vim-like UI |
| Go version | 1.23+ | Matches avredit |
| Binary / module name | `todone` — `github.com/onereallylongname/todone` | Avoids collision with existing `tuido` and `todui` projects |
| Data format | YAML, same shape as `todo.nu` (`id, done, summary, date, updated, tags, links`) | Keeps the door open to point `todone` at the same file as `todo.nu` later, without a migration step |
| Default data location | `~/.config/todone/todo.yaml` | User chose **migrate**, not reusing `$TODO_MAIN_FILE` by default (see [Config](#configuration)) |
| Override order | `--file` flag > `$TODONE_FILE` env > `store` in config.json > default path | Lets a user still point it at the legacy WSL path if desired |
| `todo.nu` fate | **Parallel** — left completely untouched | User's explicit choice; no coupling, no risk to the existing script |
| Clipboard | `tea.SetClipboard` (OSC52, zero deps, works over SSH) **+** best-effort native tool (`pbcopy`/`xclip`/`xsel`/`clip`) in parallel | OSC52 alone can silently no-op on terminals that don't support it; native tool alone fails over SSH/remote. Doing both costs nothing and maximizes the chance one lands. No `image`/clipboard crate, no CGO |
| IDs | `crypto/rand`, 5 bytes → 10 hex chars | Stdlib only; avoids an extra `uuid`/`hash` dependency for something that only needs to be locally unique |
| Fuzzy matching | `github.com/sahilm/fuzzy` | Small, dependency-light, good-enough scoring for a personal task list (hundreds, not millions, of rows) |
| Undo | Single-level (last mutation only), no redo | Suckless: a real safety net for "oops, deleted/toggled the wrong one" without the complexity of a full command/undo stack (which `avredit` needs because schema edits are structural; task mutations here are flat field writes) |
| Theming | **Deferred to v2** | One sensible built-in palette (adaptive to light/dark terminal bg via lipgloss) is enough for v1; a full JSON theme system is scope creep for a todo list |

---

## 🧱 Suckless Philosophy — how it's applied here

- **Do one thing well**: manage a flat list of tasks. No projects, no due-date
  reminders, no sync, no plugins.
- **Small, readable source**: every package is a single short file or two;
  no framework-within-a-framework. If a feature needs >~150 lines to wire up,
  it's probably out of scope for v1.
- **Text-based config and data**: `~/.config/todone/config.json` (JSON) and
  `~/.config/todone/todo.yaml` (YAML) — both hand-editable, both greppable.
- **No hidden daemons, no network calls, no telemetry.**
- **Sensible defaults, zero required setup**: first run creates its own
  config + empty store and just works.
- **Composable**: `todone list`/`todone show` support `--format table|json|yaml`
  (plus `--fields`) and plain CLI subcommands make it scriptable from
  shell/Nushell if ever wanted, without requiring the TUI.

---

## 📂 Package Layout

```
todo-app/
├── cmd/todone/
│   ├── main.go               # flag/subcommand parsing, version/help, launches CLI or TUI
│   └── output.go             # --format/--fields parsing + table/json/yaml rendering; show/follow commands
├── internal/
│   ├── config/
│   │   └── config.go        # ~/.config/todone/config.json load/save (mirrors avredit/internal/config)
│   ├── store/
│   │   ├── task.go          # Task struct (yaml tags matching todo.nu schema)
│   │   ├── store.go         # Load/Save (atomic write: tmp file + rename), mutations
│   │   └── id.go            # crypto/rand short id generator
│   ├── clipboard/
│   │   └── clipboard.go     # OSC52 Cmd + best-effort native tool (ported from avredit)
│   ├── linkkind/
│   │   └── linkkind.go      # link classification (task-id/path/url/garbage), shared by the TUI's `o` and the CLI's `follow`
│   ├── opener/
│   │   └── opener.go        # xdg-open / open / start, for the `o` keybinding and `todone follow`
│   ├── search/
│   │   └── query.go         # prefix parser (`t:`, `d:`/`s:`, `#tag`, `@link`, free text) + fuzzy rank
│   └── ui/
│       ├── app.go           # root bubbletea Model: AppMode dispatch, Update/View
│       ├── list.go          # main list panel: render + navigation state
│       ├── detail.go        # detail/edit view for one task (summary/tags/links)
│       ├── editor.go         # single-line inline text editor (ported from avredit)
│       ├── confirm.go        # y/n confirm dialog (ported from avredit)
│       ├── statusbar.go      # bottom status bar (mode, file, counts, hints)
│       ├── help.go           # `?` overlay + plain-text KeybindingsText() for --help
│       ├── tags.go           # `:tags` overlay listing every distinct tag in use
│       ├── tagcomplete.go    # tag Tab-completion engine (search/insert fields)
│       └── style.go          # lipgloss styles / palette
├── docs/
│   └── PLAN.md               # this document
├── go.mod / go.sum
└── README.md
```

This is intentionally a fraction of `avredit`'s size (~15 files vs ~40) —
there's no tree/projection/command-pattern layer because task mutations are
flat field writes on a slice, not structural edits on a nested schema.

---

## 🗃️ Data Model

```go
type Task struct {
    ID      string   `yaml:"id"`
    Done    bool     `yaml:"done"`
    Summary string   `yaml:"summary"`
    Date    string   `yaml:"date"`              // creation time, RFC3339
    Updated string   `yaml:"updated"`           // RFC3339, or "" if never updated
    Tags    []string `yaml:"tags"`
    Links   []string `yaml:"links"`
}
```

- Same field names/order as `todo.nu` so a file can be shared if the user
  later points `--file`/`$TODONE_FILE` at the WSL path.
- Dates are **read** leniently (tries RFC3339 first, then `todo.nu`'s
  `"%Y-%m-%d %H:%M:%S%.f %:z"` layout as a fallback) but **written** as
  RFC3339 — standard library only, still parses fine with Nushell's
  `into datetime`. This is a deliberate, documented simplification rather
  than byte-for-byte round-tripping `todo.nu`'s exact timestamp format.
- Writes are atomic: marshal → write to `todo.yaml.tmp` → `os.Rename` over
  the real file. No partial-write corruption risk (flaw #6 in the legacy
  script, see [Appendix](#appendix-ux-flaws-found-in-todonu)).

---

## ⌨️ UX / Keybindings (vim-like, `avredit`-inspired)

### Modes

| Mode | Activation | Purpose |
|---|---|---|
| Normal | default / `Esc` | navigate list, trigger actions |
| Insert | `a` (add), `e`/`Enter` on a field (edit) | typing into a single-line field |
| Search | `/` | live filter, prefix-aware |
| Command | `:` | ex-commands (`:sort`, `:all`, `:q`, `:wq`, `:tags`) |

### Status bar

Always pinned to the terminal's last line, laid out as
`mode | file | counts/sort/filter ... help/quit hint`. The file segment
shows the in-use data file abbreviated under `$HOME` as `~/...`; if the
terminal is too narrow to fit the full path it degrades to just the
basename (e.g. `todo.yaml`) rather than truncating mid-path.

### Global

| Key | Action |
|---|---|
| `q` | Quit (prompts if there's an in-flight edit) |
| `?` | Help overlay |
| `Esc` | Cancel current mode / close overlay |
| `Ctrl+C` | No-op — hints "use q or :q to quit" (todone never force-quits on it, like vim) |
| `/` | Enter search |
| `:` | Enter command mode |

### List (Normal mode)

| Key | Action |
|---|---|
| `j`/`k`, `↓`/`↑` | Move selection |
| `gg` / `G` | Jump to top / bottom |
| `Ctrl+D`/`Ctrl+U`, `f`/`b` | Half-page down/up (vim-style aliases) |
| `Space` | Toggle done |
| `Enter` | Open detail/edit view for selected task |
| `a` | Add task (inline prompt; `#tag` tokens and `http(s)://` tokens in the text become tags/links automatically) |
| `d` | Delete selected (instant, `u` undoes — no confirm-prompt friction) |
| `u` / `Ctrl+R` | Undo / redo (deep history, see Configuration) |
| `y` then `y`/`l`/`i` | Yank summary / links / id to clipboard (vim operator+motion feel) |
| `o` | Open link — see **Link resolution** below |
| `Ctrl+O` / `Ctrl+I` | Jump back / forward to wherever a task-id link last took you |
| `A` | Toggle show-done / active-only |
| `s` | Cycle sort: updated → added → alphabetical |

### Detail view (Normal mode, opened with `Enter`)

Tags and Links are real editable lists here, not just display fields.

| Key | Action |
|---|---|
| `j`/`k` | Move between rows (Summary / each Tag / each Link) |
| `H`/`L` | Open the next/previous task, respecting the active filter+sort |
| `J`/`K` | Move the selected Tags/Links row up/down (move a link to slot 0 to make it the `o`/list-view default) |
| `e`/`Enter` | Edit the selected row |
| `a` | Add a new Tag or Link item |
| `d` | Delete the selected item (instant, `u` undoes) |
| `y` | Yank the selected row's value |
| `o` | Open link — see **Link resolution** below |
| `Ctrl+O` / `Ctrl+I` | Jump back / forward to wherever a task-id link last took you |
| `u` / `Ctrl+R` | Undo / redo |
| `Esc`/`q` | Back to list |

### Insert mode (add/edit)

| Key | Action |
|---|---|
| `Enter` | Confirm |
| `Esc` | Cancel |
| `←`/`→`, `Home`/`End`, `Backspace`/`Delete` | Standard line editing |
| `Ctrl+A`/`Ctrl+E`/`Ctrl+U`/`Ctrl+K`/`Ctrl+W` | Readline-style (start/end/clear-before/clear-after/delete-word) |
| `Ctrl+V` / `Shift+Insert` | Paste from the native OS clipboard (plus automatic bracketed-paste support) |
| `Tab` | Complete a tag: whole-value in a Tags-list add/edit prompt, or a trailing `#tag` token in quick-add free text |

### Link resolution (what `o` actually does)

A Links entry is classified in priority order the moment you press `o`:

1. **Task id** (exact match against another task's id) → jump to that
   task, staying in whichever view you're already in (list view just
   moves the cursor there; detail view opens that task's detail directly).
   If the target is hidden by the active filter, the filter is cleared so
   the jump always lands somewhere visible. `Ctrl+O`/`Ctrl+I` retrace these
   jumps, back and forward.
2. **Filesystem path** that currently exists (file or directory, `~`
   expanded, resolved against the cwd if relative) → opened with the OS's
   default handler (`xdg-open`/`open`/`start`).
3. **URL** (anything starting with a `scheme://`) → opened the same way.
4. Anything else → reported as an error; nothing is shelled out to.

To keep path-links from breaking if todone is later launched from a
different directory, **typing or editing a Link that resolves to a real
file/directory is stored as an absolute path immediately** (at the moment
you commit the add/edit, not at open time). URLs, task ids, garbage, and
not-yet-existing paths are left exactly as typed.



### Search (`/`) — prefixes (directly inspired by `avredit`'s `n:`/`t:`/`ns:`)

| Prefix | Filters by | Example |
|---|---|---|
| `t:` or `#` | Tag (substring, tab completes against every tag in use) | `t:pricing` / `#pricing` |
| `d:` or `s:` | Status (`done`/`pending`) | `d:done` |
| `@` | Link (substring match against the raw link text, including task-id links) | `@github.com` |
| *(none)* | Fuzzy match on summary | `kafka client` |
| `a \| b` | OR: matches free text `a` OR `b` (hard filters still AND, regardless of which side of `\|` they're on) | `int \| work #now` |
| `\#` `\|` `\@` | Escapes — treat `#`/`\|`/`@` as a literal character in free text instead of a prefix/separator | `fix \#1 on call` |

Tokens are space-separated and compose (AND'ed): `#kafka d:pending retry`
filters to pending tasks tagged `kafka` whose summary fuzzy-matches "retry".
A bare `|` token splits the free text into OR'd alternatives instead —
`int | work #now` matches tasks tagged `#now` whose summary fuzzy-matches
"int" **or** "work"; a task matching more than one OR branch is ranked by
whichever branch scores it best. Empty branches (leading/trailing/doubled
`|`) are silently dropped rather than acting as an always-match wildcard.
A token starting with a backslash followed by `#`, `|`, or `@` is never
treated as a prefix/separator — it's unescaped to the literal character and
folded into the free-text match instead (so a summary containing a literal
`#1` can still be searched for with `\#1`).

### Command mode (`:`)

| Command | Action |
|---|---|
| `:w` | Force save |
| `:q` / `:q!` | Quit |
| `:wq` | Save and quit |
| `:sort date\|updated\|alpha` | Explicit sort |
| `:all` | Toggle show-done |
| `:tags` | Open a scrollable overlay listing every distinct tag in use |

---

## 🖥️ CLI (fast in/out, no TUI)

```
todone                        # launch TUI
todone add <text...>          # add a task (parses #tags/@links from text, \#/\|/\@ escape literals), prints id, exits
todone list [--all] [--format table|json|yaml] [--fields <preset|comma-list>]
                               # print active (or all) tasks, exits
todone show <id|query>        # print exactly one task (any-state substring match), same --format/--fields as list
todone follow <id|query> [--index N] [--format table|json|yaml] [--fields <preset|comma-list>]
                               # open a task's link (default: first link, or --index N) without starting the TUI;
                               # task-id links print the target task's row instead of shelling out, honoring
                               # --format/--fields; path/URL links ignore those flags (nothing to render)
todone toggle <id|query>      # toggle done, resolved by: exact id → id prefix → exact summary → pending-only substring; ambiguous matches are printed so you can retry with more characters
todone tags                   # list every tag in use, sorted, exits
todone -v / --version         # -v: version only. --version: version + Go version + OS/Arch
todone -h / --help            # -h: CLI usage only. --help: usage + the full TUI keybinding reference
```

`--format` accepts `table` (default once `--format`/`--fields` is given),
`json`, or `yaml`. `--fields` accepts a preset (`name`, `default`, `all`) or
a custom comma-separated list (e.g. `--fields id,summary,tags`); field order
in output is always the canonical `id, done, summary, date, updated, tags,
links` order regardless of how the list was typed. With neither flag, `list`
keeps printing its original compact one-line-per-task rows for backwards
compatibility; giving `--fields` alone implies `--format table`; giving
`--format` alone implies all fields. JSON/YAML keys match `todo.yaml`'s own
field names, so output can be piped straight into other YAML/JSON tooling.

Interactive-only features (yank, open-link, live filter, undo) are TUI-only
by design — scripting those doesn't make sense. `list`/`show`/`follow`/
`add`/`toggle`/`tags` cover the "fast in and out" half of the ask; the TUI
covers "stay and explore."

---

## ⚙️ Configuration

`~/.config/todone/config.json` (mirrors `avredit/internal/config`):

```json
{
  "store": "",
  "default_sort": "updated",
  "show_done": false
}
```

| Field | Description | Default |
|---|---|---|
| `store` | Path to the YAML task file | `~/.config/todone/todo.yaml` |
| `default_sort` | `updated` \| `date` \| `alpha` | `updated` |
| `show_done` | Start with done tasks visible | `false` |

Resolution order for the data file: `--file` flag > `$TODONE_FILE` env >
`store` in config > default.

### Theming

`~/.config/todone/theme.json` (optional, colors only — not present by
default). Any field accepts a hex string (`"#7aa2f7"`) or an ANSI-256
index (`"75"`, matching the built-in defaults), and an absent/empty field
just keeps the built-in color for that slot — there's no multi-theme
registry or structural overrides like `avredit` has, by design (suckless:
one sensible default, small override surface).

```json
{
  "fg": "252",
  "dim": "244",
  "muted": "238",
  "primary": "75",
  "success": "114",
  "warning": "179",
  "error": "203",
  "accent": "183",
  "contrast_fg": "0"
}
```

| Field | Used for |
|---|---|
| `fg` | Base text color |
| `dim` | Subtitles, tag/age lines, dimmed hints |
| `muted` | Done-task strikethrough, muted borders |
| `primary` | Selection highlight, Normal-mode badge, detail border |
| `success` | Insert-mode badge |
| `warning` | Command-mode badge, warnings |
| `error` | Error flashes |
| `accent` | Tag badges, Search-mode badge |
| `contrast_fg` | Text drawn on top of a colored background (mode badges, selected row) |

Eleven ready-made palettes (ported from `avredit`'s theme collection and
converted to this schema) live in [`themes/`](../themes) — copy one to
`~/.config/todone/theme.json` to use it, e.g. `cp themes/dracula.json
~/.config/todone/theme.json`.

---

## 📋 Functional Requirements

### v1 (this pass)

- [x] List, add, edit (summary/tags/links), delete, toggle-done
- [x] Live filter/search with prefixes (`t:`, `d:`/`s:`, free text)
- [x] Sort (updated/added/alpha), show-done toggle
- [x] Vim-like navigation (`j/k/gg/G`, half-page scroll)
- [x] Clipboard yank (OSC52 + native fallback), open-link
- [x] Single-level undo
- [x] `:command` mode, `?` help overlay
- [x] CLI subcommands: `add`, `list`, `done` (non-interactive)
- [x] Config file, atomic YAML persistence

### v0.1.2

- [x] `-v`/`--version` and `-h`/`--help` differentiated: short forms print
      a one-liner; long forms print the full version+Go+OS info or the
      full CLI usage + TUI keybinding reference, respectively
- [x] `done` renamed to `toggle`, with a 4-tier resolver: exact id → id
      prefix (any done-state) → exact summary (any done-state) →
      substring summary (pending-only, the original behavior) — ambiguous
      matches at any tier print every candidate instead of just erroring
- [x] `todone tags` CLI command + `:tags` TUI overlay (sorted, deduped
      list of every tag in use)
- [x] Tag Tab-completion everywhere a tag is typed: the Tags-list
      add/edit prompt, quick-add `#tag` tokens, and search-mode `#`/`t:`
      tokens — all sourced from one shared `store.UniqueSortedTags`
- [x] Status bar shows the in-use data file, width-aware (full `~`-path,
      degrading to basename)
- [x] `Ctrl+C` is a no-op with a "use q or :q to quit" hint, matching
      vim's refusal to be force-quit by habit
- [x] Search gained `|` as an OR-group separator (`int | work #now`),
      ranked by each matching task's best-scoring OR branch

### v0.1.3

- [x] Sigil-escaping: `\#`, `\|`, `\@` are treated as literal characters
      (never a prefix/separator) in both quick-add text and search queries
- [x] `@link` quick-add syntax: an `@token` typed while adding a task is
      appended to its links, same convention as bare `#tag`/`http(s)://`
- [x] `@link` search prefix: substring filter against raw link text
      (including task-id links), composed/AND'ed like `#tag`
- [x] `internal/linkkind` package extracted from `internal/ui/link.go`:
      one shared, dependency-free link-classification implementation
      (task-id / path / url / garbage) used by both the TUI's `o` key and
      the CLI's `follow` command
- [x] `todone show <id|query>` — print exactly one task, resolved with an
      any-done-state resolver (unlike `toggle`'s pending-only substring
      tier, `show`/`follow` never silently skip a matching done task)
- [x] `todone follow <id|query> [--index N]` — open a task's link without
      starting the TUI; defaults to the first link, `--index`/`-n` picks
      another; a task-id link prints the target task's row (no TUI to
      jump into) instead of shelling out; a garbage link is an error
- [x] `--format table|json|yaml` and `--fields <preset|comma-list>` for
      `list`, `show`, and `follow` (the last only when a link resolves to
      a task id — a path/URL link has no task to render, so the flags are
      parsed but simply unused for those); presets are
      `name`/`default`/`all`; JSON/YAML keys match `todo.yaml`'s own field
      names; omitting both flags keeps `list`'s original compact
      one-line-per-task output for backwards compatibility

### v2+ (explicitly deferred)

- Shared-file mode with `todo.nu` documented as a supported workflow (not just possible)
- **Multi-file support** (open several todo files/sources at once) — considered
  during v0.1.2 and explicitly deferred. Pros: unified cross-file search;
  natural fit for `todo.nu`/sync workflows; cross-file task-id link jumps;
  read-only shared/aggregated sources; scripting flexibility. Cons: task-id
  collision risk across files; breaks the current 1-Store↔1-file↔1-atomic-save
  model and complicates undo/redo; directly conflicts with the
  just-added single-file status-bar display; adds real complexity against
  the suckless "do one thing" goal; loses the atomic single-file save
  guarantee. Revisit only if a concrete cross-file workflow need emerges.

---

## ⚠️ Risks / Open Questions

- **OSC52 support varies** by terminal; native-tool fallback mitigates but
  doesn't eliminate silent no-ops in minimal environments (bare Linux tty,
  some SSH jump hosts without either).
- **No redo** means undo is "one free mistake" — acceptable given task
  mutations are low-stakes and the file itself is plain, hand-editable YAML.
- **Fuzzy lib choice** (`sahilm/fuzzy`) is untested at the data volumes this
  particular user already has (hundreds of entries in the real `todo.todo`
  file) — expected to be fine, will confirm with a quick benchmark during
  implementation.

---

## Appendix: UX flaws found in `todo.nu`

Captured for posterity / to make sure `todone` doesn't repeat them:

1. No edit/update command — typos in summary/tags are permanent without
   hand-editing the YAML.
2. No delete command — the list only grows (real file already ~16 KB).
3. No "reopen"/undone command once a task is marked done.
4. `done <id>` uses substring `like` matching **and** silently returns `[]`
   on no match — easy to silently no-op or, worse, match the wrong task.
5. Whole file is read + fully rewritten on every mutation, with no atomic
   write (temp file + rename) — an interrupted `done`/`add` can corrupt the
   store.
6. No confirmation before a (mis-)matched `done` mutates and saves.
7. No combined filters (tag **and** text), no sort control — always
   insertion order.
8. `updated` column mixes types on disk (`''` vs a datetime string),
   fragile for any tooling beyond the script itself.
9. Compact `list` view hides tags and links entirely; only `--full` shows
   them, and even then there's no way to open a link.
10. Requires `$env.TODO_MAIN_FILE` to be set with no default/fallback and
    no validation — fails ungracefully if unset.
11. Every invocation pays the Nushell interpreter's startup cost; no
    persistent session for rapid multi-add workflows.
12. Minor copy typos in the `input list` prompts ("Select tasks wih space
    key, confim wiht enter key").
