package ui

import "testing"

func TestParseAddInputExtractsTagsAndLinks(t *testing.T) {
	summary, tags, links := parseAddInput("Fix kafka #backend #infra https://x.test/issue @./notes.md @3964bffd95")
	if summary != "Fix kafka" {
		t.Fatalf("expected summary 'Fix kafka', got %q", summary)
	}
	wantTags := []string{"backend", "infra"}
	if len(tags) != len(wantTags) || tags[0] != wantTags[0] || tags[1] != wantTags[1] {
		t.Fatalf("expected tags %v, got %v", wantTags, tags)
	}
	wantLinks := []string{"https://x.test/issue", "./notes.md", "3964bffd95"}
	if len(links) != len(wantLinks) {
		t.Fatalf("expected links %v, got %v", wantLinks, links)
	}
	for i := range wantLinks {
		if links[i] != wantLinks[i] {
			t.Fatalf("expected links %v, got %v", wantLinks, links)
		}
	}
}

func TestParseAddInputEscapedSigilsStayLiteral(t *testing.T) {
	summary, tags, links := parseAddInput(`Ticket \#1 \@mention \|pipe #real`)
	wantSummary := `Ticket #1 @mention |pipe`
	if summary != wantSummary {
		t.Fatalf("expected summary %q, got %q", wantSummary, summary)
	}
	if len(tags) != 1 || tags[0] != "real" {
		t.Fatalf("expected only the unescaped #real tag, got %v", tags)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links from escaped tokens, got %v", links)
	}
}

func TestParseAddInputBareAtSignIsIgnored(t *testing.T) {
	// A lone "@" (len == 1) has nothing to link to, so it passes through
	// as plain text, mirroring the existing "#" len>1 guard.
	summary, _, links := parseAddInput("ping @ here")
	if summary != "ping @ here" {
		t.Fatalf("expected lone '@' to stay in summary, got %q", summary)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links, got %v", links)
	}
}

func TestUnescapeSigil(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		escaped bool
	}{
		{`\#1`, "#1", true},
		{`\|pipe`, "|pipe", true},
		{`\@mention`, "@mention", true},
		{`#tag`, "#tag", false}, // not escaped, unrelated sigil check
		{`\x`, `\x`, false},     // backslash before a non-sigil char is untouched
		{`\`, `\`, false},       // lone backslash, too short to be an escape
		{`plain`, "plain", false},
	}
	for _, c := range cases {
		got, escaped := unescapeSigil(c.in)
		if got != c.want || escaped != c.escaped {
			t.Fatalf("unescapeSigil(%q) = (%q, %v), want (%q, %v)", c.in, got, escaped, c.want, c.escaped)
		}
	}
}
