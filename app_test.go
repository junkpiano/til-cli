package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/github"
)

// Titles taken from real issues in junkpiano/til that produced empty titles
// because the bare YAML scalar broke front matter parsing.
const (
	titleWifi   = `Ubuntu 24.04: "No Wi-Fi Adapter Found" on Intel Core Ultra (ILL) – fixed by forcing OEM kernel via GRUB`
	titleClaude = `'Claude Code Permission Modes: Manual vs Accept Edits vs Plan vs Auto'`
)

func TestYamlString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Hello World", `"Hello World"`},
		{"colon", "Go: a tour", `"Go: a tour"`},
		{"double quotes", `say "hi"`, `"say \"hi\""`},
		{"single quotes", `'quoted'`, `"'quoted'"`},
		{"backslash", `C:\path`, `"C:\\path"`},
		{"newline", "a\nb", `"a\nb"`},
		{"tab", "a\tb", `"a\tb"`},
		{"carriage return", "a\rb", `"a\rb"`},
		{"leading hash", "#hashtag", `"#hashtag"`},
		{"leading dash", "- item", `"- item"`},
		{"non-ascii preserved", "en – dash 日本語", `"en – dash 日本語"`},
		{"empty", "", `""`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := yamlString(c.in); got != c.want {
				t.Errorf("yamlString(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

// yamlString must always produce something that decodes back to the original,
// so no title can silently lose characters on the way into a post.
func TestYamlStringRoundTrip(t *testing.T) {
	titles := []string{
		titleWifi,
		titleClaude,
		"Plain title",
		`He said "no": then left`,
		`back\slash and "quote"`,
		"multi\nline",
	}

	for _, title := range titles {
		got, err := strconv.Unquote(yamlString(title))
		if err != nil {
			t.Fatalf("yamlString(%q) produced an undecodable scalar: %v", title, err)
		}
		if got != title {
			t.Errorf("round trip of %q gave %q", title, got)
		}
	}
}

func TestYamlValueLeavesEmptyBare(t *testing.T) {
	if got := yamlValue(""); got != "" {
		t.Errorf(`yamlValue("") = %q, want "" so the key stays nil rather than an empty string`, got)
	}
	if got := yamlValue("swift"); got != `"swift"` {
		t.Errorf(`yamlValue("swift") = %s, want "swift" quoted`, got)
	}
}

// parseFrontMatter splits a generated front matter block into key/value pairs,
// mirroring what a YAML parser does for a flat mapping of scalars. It fails the
// test on anything a real parser would reject.
func parseFrontMatter(t *testing.T, fm string) map[string]string {
	t.Helper()

	if !strings.HasPrefix(fm, "---\n") {
		t.Fatalf("front matter does not open with ---:\n%s", fm)
	}
	body := strings.TrimPrefix(fm, "---\n")
	end := strings.Index(body, "---\n")
	if end < 0 {
		t.Fatalf("front matter is not closed with ---:\n%s", fm)
	}
	body = body[:end]

	out := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		sep := strings.Index(line, ": ")
		if sep < 0 {
			// A key with an empty value renders as "key:" with no trailing space.
			if !strings.HasSuffix(line, ":") {
				t.Fatalf("line %q is not a key/value mapping", line)
			}
			out[strings.TrimSuffix(line, ":")] = ""
			continue
		}
		key, value := line[:sep], line[sep+2:]
		if strings.HasPrefix(value, `"`) {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				t.Fatalf("value for %q is not a decodable quoted scalar: %q (%v)", key, value, err)
			}
			value = unquoted
		} else if strings.Contains(value, ": ") {
			// This is the exact failure mode the quoting fix exists to prevent:
			// a bare scalar containing ": " is a YAML mapping error.
			t.Fatalf("value for %q is a bare scalar containing %q, which YAML rejects: %q", key, ": ", value)
		}
		out[key] = value
	}

	return out
}

func TestFrontMatterParsesRealIssueTitles(t *testing.T) {
	date := time.Date(2025, 11, 4, 9, 30, 0, 0, time.UTC)

	cases := []struct {
		name     string
		title    string
		category string
	}{
		{"colon and double quotes", titleWifi, "linux"},
		{"colon and wrapping single quotes", titleClaude, "ai"},
		{"plain title", "Formatting dates in Swift", "swift"},
		{"no category", "A title with no label", ""},
		{"category with a colon", "Some title", "meta: notes"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm := parseFrontMatter(t, frontMatter("post", c.title, c.category, date))

			if fm["title"] != c.title {
				t.Errorf("title = %q, want %q", fm["title"], c.title)
			}
			if fm["category"] != c.category {
				t.Errorf("category = %q, want %q", fm["category"], c.category)
			}
			if fm["layout"] != "post" {
				t.Errorf("layout = %q, want %q", fm["layout"], "post")
			}
			if want := "2025-11-04 09:30:00 +0000"; fm["date"] != want {
				t.Errorf("date = %q, want %q", fm["date"], want)
			}
		})
	}
}

// An absent category must stay nil rather than becoming "", which Liquid would
// treat as truthy.
func TestFrontMatterEmptyCategoryStaysBare(t *testing.T) {
	fm := frontMatter("post", "Title", "", time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC))
	if !strings.Contains(fm, "\ncategory: \n") {
		t.Errorf("expected a bare empty category line, got:\n%s", fm)
	}
	if strings.Contains(fm, `category: ""`) {
		t.Errorf("empty category was quoted, got:\n%s", fm)
	}
}

func TestFrontMatterExactOutput(t *testing.T) {
	got := frontMatter("post", titleWifi, "linux", time.Date(2025, 11, 4, 9, 30, 0, 0, time.UTC))
	want := "---\n" +
		"layout: post\n" +
		`title: "Ubuntu 24.04: \"No Wi-Fi Adapter Found\" on Intel Core Ultra (ILL) – fixed by forcing OEM kernel via GRUB"` + "\n" +
		"date: 2025-11-04 09:30:00 +0000\n" +
		"category: \"linux\"\n" +
		"---\n\n"

	if got != want {
		t.Errorf("frontMatter() =\n%s\nwant\n%s", got, want)
	}
}

func newIssue(id int64, created time.Time, labels ...string) *github.Issue {
	issue := &github.Issue{ID: &id, CreatedAt: &created}
	for i := range labels {
		issue.Labels = append(issue.Labels, github.Label{Name: &labels[i]})
	}
	return issue
}

func TestFileLink(t *testing.T) {
	issue := newIssue(4242, time.Date(2025, 3, 9, 12, 0, 0, 0, time.UTC))
	if got, want := fileLink(issue), "2025-03-09-4242.md"; got != want {
		t.Errorf("fileLink() = %q, want %q", got, want)
	}
}

func TestParmaLink(t *testing.T) {
	created := time.Date(2025, 3, 9, 12, 0, 0, 0, time.UTC)

	if got, want := parmaLink(newIssue(4242, created, "swift")), "swift/2025/03/09/4242.html"; got != want {
		t.Errorf("parmaLink() with label = %q, want %q", got, want)
	}
	if got, want := parmaLink(newIssue(4242, created)), "2025/03/09/4242.html"; got != want {
		t.Errorf("parmaLink() without label = %q, want %q", got, want)
	}
}

func TestFindReserved(t *testing.T) {
	cases := map[string]string{
		"iOS":     "iOS",
		"swift":   "Swift",
		"go lang": "GoLang",
	}

	for in, want := range cases {
		if got := findReserved(in); got != want {
			t.Errorf("findReserved(%q) = %q, want %q", in, got, want)
		}
	}
}

// pagedLister returns a lister that serves the given pages in order, recording
// the Page value it was asked for each time.
func pagedLister(pages [][]*github.Issue, requested *[]int) issueLister {
	return func(opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
		*requested = append(*requested, opts.Page)

		index := opts.Page
		if index > 0 {
			index-- // page 0 and page 1 both mean the first page
		}
		if index >= len(pages) {
			return nil, &github.Response{}, nil
		}

		resp := &github.Response{}
		if index+1 < len(pages) {
			resp.NextPage = index + 2
		}

		return pages[index], resp, nil
	}
}

func TestListAllIssuesFollowsEveryPage(t *testing.T) {
	created := time.Date(2025, 3, 9, 12, 0, 0, 0, time.UTC)
	pages := [][]*github.Issue{
		{newIssue(1, created), newIssue(2, created)},
		{newIssue(3, created), newIssue(4, created)},
		{newIssue(5, created)},
	}

	var requested []int
	opts := &github.IssueListByRepoOptions{State: "closed"}

	issues, err := listAllIssues(pagedLister(pages, &requested), opts)
	if err != nil {
		t.Fatalf("listAllIssues() returned an error: %v", err)
	}

	if len(issues) != 5 {
		t.Fatalf("got %d issues across %d pages, want 5", len(issues), len(pages))
	}
	for i, issue := range issues {
		if want := int64(i + 1); *issue.ID != want {
			t.Errorf("issue %d has ID %d, want %d (pages must stay in order)", i, *issue.ID, want)
		}
	}

	wantRequested := []int{0, 2, 3}
	if len(requested) != len(wantRequested) {
		t.Fatalf("requested pages %v, want %v", requested, wantRequested)
	}
	for i, page := range wantRequested {
		if requested[i] != page {
			t.Errorf("requested pages %v, want %v", requested, wantRequested)
			break
		}
	}
}

func TestListAllIssuesSinglePage(t *testing.T) {
	created := time.Date(2025, 3, 9, 12, 0, 0, 0, time.UTC)
	pages := [][]*github.Issue{{newIssue(1, created)}}

	var requested []int
	issues, err := listAllIssues(pagedLister(pages, &requested), &github.IssueListByRepoOptions{})
	if err != nil {
		t.Fatalf("listAllIssues() returned an error: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("got %d issues, want 1", len(issues))
	}
	if len(requested) != 1 {
		t.Errorf("made %d requests, want 1 when NextPage is 0", len(requested))
	}
}

// A response whose NextPage does not advance must terminate rather than spin.
func TestListAllIssuesStopsOnNonAdvancingNextPage(t *testing.T) {
	calls := 0
	lister := func(opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
		calls++
		if calls > 10 {
			t.Fatal("listAllIssues did not terminate on a non-advancing NextPage")
		}
		return nil, &github.Response{NextPage: opts.Page}, nil
	}

	opts := &github.IssueListByRepoOptions{ListOptions: github.ListOptions{Page: 3}}
	if _, err := listAllIssues(lister, opts); err != nil {
		t.Fatalf("listAllIssues() returned an error: %v", err)
	}
	if calls != 1 {
		t.Errorf("made %d requests, want 1", calls)
	}
}

func TestListAllIssuesPropagatesError(t *testing.T) {
	created := time.Date(2025, 3, 9, 12, 0, 0, 0, time.UTC)
	wantErr := errors.New("rate limited")

	calls := 0
	lister := func(opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
		calls++
		if calls == 1 {
			return []*github.Issue{newIssue(1, created)}, &github.Response{NextPage: 2}, nil
		}
		return nil, nil, wantErr
	}

	issues, err := listAllIssues(lister, &github.IssueListByRepoOptions{})
	if err != wantErr {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}
	if issues != nil {
		t.Errorf("got %v issues alongside an error, want nil", issues)
	}
}

func TestCheckIssue(t *testing.T) {
	login := "junkpiano"
	other := "someone-else"
	title := "A title"
	body := "A body"
	url := "https://github.com/junkpiano/til/issues/1"
	created := time.Date(2025, 3, 9, 12, 0, 0, 0, time.UTC)

	valid := func() *github.Issue {
		return &github.Issue{
			User:      &github.User{Login: &login},
			Title:     &title,
			Body:      &body,
			HTMLURL:   &url,
			CreatedAt: &created,
		}
	}

	if !checkIssue(valid()) {
		t.Error("expected a complete issue authored by junkpiano to be valid")
	}

	notMine := valid()
	notMine.User = &github.User{Login: &other}
	if checkIssue(notMine) {
		t.Error("expected an issue from another author to be skipped")
	}

	pr := valid()
	pr.PullRequestLinks = &github.PullRequestLinks{}
	if checkIssue(pr) {
		t.Error("expected a pull request to be skipped")
	}

	noBody := valid()
	noBody.Body = nil
	if checkIssue(noBody) {
		t.Error("expected an issue without a body to be skipped")
	}
}

func TestMkheader(t *testing.T) {
	if got, want := mkheader(2, "Category"), "## Category\n\n"; got != want {
		t.Errorf("mkheader(2, ...) = %q, want %q", got, want)
	}
	if got, want := mkheader(1, "Title"), "# Title\n\n"; got != want {
		t.Errorf("mkheader(1, ...) = %q, want %q", got, want)
	}
}

func TestMklink(t *testing.T) {
	got := mklink("discussion", "https://github.com/junkpiano/til/issues/1")
	want := "[discussion](https://github.com/junkpiano/til/issues/1)"
	if got != want {
		t.Errorf("mklink() = %q, want %q", got, want)
	}
}

// chdir moves into a scratch directory for the duration of the test. Written
// out by hand rather than using t.Chdir, which needs a newer Go than go.mod
// declares.
func chdir(t *testing.T, dir string) {
	t.Helper()

	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to %s: %v", dir, err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("chdir back to %s: %v", old, err)
		}
	})
}

func TestGenerateReadme(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.MkdirAll("dist", os.ModePerm); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}

	items := map[string][]IssueItem{
		"swift": {
			{category: "swift", path: "Swift/2025/03/09/1.html", title: "Swift: value types"},
		},
		"iOS": {
			{category: "iOS", path: "iOS/2025/03/10/2.html", title: "Background tasks"},
			{category: "iOS", path: "iOS/2025/03/11/3.html", title: "Widgets"},
		},
	}

	generateReadme(3, items)

	raw, err := os.ReadFile("dist/archives.md")
	if err != nil {
		t.Fatalf("read archives.md: %v", err)
	}
	content := string(raw)

	if !strings.HasPrefix(content, "---\nlayout: page\ntitle: \"Archives\"\n") {
		t.Errorf("archives.md front matter =\n%s", content)
	}
	if !strings.Contains(content, "*3 TILs, and counting...*") {
		t.Error("expected the TIL count in the tagline")
	}

	// Reserved category names keep their casing, others are camel cased, and
	// the anchor stays lower case so the in-page link resolves.
	for _, want := range []string{
		"* [iOS](#ios)\n",
		"* [Swift](#swift)\n",
		"## iOS\n",
		"## Swift\n",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in archives.md", want)
		}
	}

	// Categories are emitted in sorted key order: "iOS" before "swift".
	if strings.Index(content, "## iOS\n") > strings.Index(content, "## Swift\n") {
		t.Error("expected categories sorted by key")
	}

	// Paths are lower cased in links, titles are left alone.
	if !strings.Contains(content, "* [Background tasks](ios/2025/03/10/2.html)") {
		t.Errorf("expected a lower cased item link, got\n%s", content)
	}
}
