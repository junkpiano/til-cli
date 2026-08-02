package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/iancoleman/strcase"
)

type IssueItem struct {
	category string
	path     string
	title    string
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func fileLink(issue *github.Issue) string {
	l := ""
	t := (*issue.CreatedAt).Format("2006-01-02")
	filename := strings.Join([]string{t, strconv.FormatInt(*issue.ID, 10)}, "-")
	l += filename + ".md"

	return l
}

func parmaLink(issue *github.Issue) string {
	l := ""
	if len(issue.Labels) > 0 {
		l += *issue.Labels[0].Name + "/"
	}
	t := (*issue.CreatedAt).Format("2006/01/02")
	filename := strings.Join([]string{t, strconv.FormatInt(*issue.ID, 10)}, "/")
	l += filename + ".html"

	return l
}

// defaultPreservedCase keeps the historical behaviour for labels whose casing
// camel casing would mangle. Overridden by the preserve-case input.
var defaultPreservedCase = []string{"iOS"}

// findReserved camel cases a label for display, except for labels whose exact
// casing is spelled out in preserved.
func findReserved(key string, preserved []string) string {
	for _, p := range preserved {
		if p == key {
			return p
		}
	}

	return strcase.ToCamel(key)
}

// resolvePreservedCase reads the comma separated preserve-case input, falling
// back to the built-in list when it is unset.
//
// The runner uppercases an input id and replaces spaces with underscores, but
// leaves hyphens alone, so preserve-case arrives as INPUT_PRESERVE-CASE. Both
// spellings are accepted so the input works however it is passed.
func resolvePreservedCase(lookup func(string) string) []string {
	raw := ""
	for _, key := range []string{"INPUT_PRESERVE-CASE", "INPUT_PRESERVE_CASE"} {
		if v := lookup(key); v != "" {
			raw = v
			break
		}
	}

	if raw == "" {
		return defaultPreservedCase
	}

	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func generateReadme(numberOfIssues int, items map[string][]IssueItem, preserved []string) {
	title := frontMatter("page", "Archives", "", time.Now())
	tagline := fmt.Sprintf("*%d TILs, and counting...*", numberOfIssues)
	categories := mkheader(2, "Category")
	table := ""

	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		key := findReserved(k, preserved)
		categories += "* [" + key + "](#" + strings.ToLower(k) + ")\n"
		table += mkheader(2, key)
		for _, item := range items[k] {
			line := fmt.Sprintf("* [%s](%s) \n", item.title, strings.ToLower(item.path))
			table += line
		}
		table += "\n"
	}
	content := fmt.Sprintf("%s%s\n\n%s\n\n%s\n", title, tagline, categories, table)
	check(os.WriteFile("dist/archives.md", []byte(content), 0644))

	fmt.Println(content)
}

func mkheader(level int, str string) string {
	hashes := strings.Repeat("#", level)
	hashes += " " + str + "\n\n"
	return hashes
}

func mklink(title string, url string) string {
	return fmt.Sprintf("[%s](%s)", title, url)
}

// yamlString renders s as a YAML double-quoted scalar. Issue titles regularly
// contain ':', quotes or leading metacharacters, which break front matter when
// emitted as a bare scalar.
func yamlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// yamlValue quotes s unless it is empty, so an absent category stays nil rather
// than becoming an empty string (which Liquid treats as truthy).
func yamlValue(s string) string {
	if s == "" {
		return ""
	}
	return yamlString(s)
}

func frontMatter(layout string, title string, category string, date time.Time) string {
	return fmt.Sprintf("---\nlayout: %s\ntitle: %s\ndate: %s\ncategory: %s\n---\n\n", layout, yamlValue(title), date.Format("2006-01-02 15:04:05 +0000"), yamlValue(category))
}

// authorAllowed reports whether the issue's author is in authors. A nil slice
// accepts anyone; a missing author is never accepted.
func authorAllowed(issue *github.Issue, authors []string) bool {
	// An issue with no author is skipped either way; the old code dereferenced
	// this unconditionally and would have panicked.
	if issue.User == nil || issue.User.Login == nil {
		return false
	}

	if authors == nil {
		return true
	}

	for _, a := range authors {
		if a == *issue.User.Login {
			return true
		}
	}

	return false
}

// checkIssue reports whether an issue should be published. A nil authors slice
// accepts any author.
func checkIssue(issue *github.Issue, authors []string) bool {
	if !authorAllowed(issue, authors) {
		return false
	}

	if issue.PullRequestLinks == nil &&
		issue.Title != nil &&
		issue.CreatedAt != nil &&
		issue.Body != nil &&
		issue.HTMLURL != nil {
		return true
	}

	return false
}

// issueLister fetches one page of issues. It exists so the paging loop can be
// exercised without talking to GitHub.
type issueLister func(opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error)

// listAllIssues walks every page. Without this the API returns only the first
// page, silently dropping every issue beyond it.
func listAllIssues(list issueLister, opts *github.IssueListByRepoOptions) ([]*github.Issue, error) {
	var all []*github.Issue

	for {
		issues, resp, err := list(opts)
		if err != nil {
			return nil, err
		}

		all = append(all, issues...)

		// NextPage is 0 on the last page. Requiring it to advance also keeps a
		// misbehaving response from looping forever.
		if resp == nil || resp.NextPage <= opts.Page {
			return all, nil
		}

		opts.Page = resp.NextPage
	}
}

// tokenTransport adds an Authorization header to every request.
type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone rather than mutate: RoundTrip must not modify the caller's request.
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "token "+t.token)

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(clone)
}

// repoRef identifies the repository whose issues get published.
type repoRef struct {
	owner string
	name  string
}

func parseRepo(s string) (repoRef, error) {
	owner, name, found := cut(s, "/")
	if !found || owner == "" || name == "" || strings.Contains(name, "/") {
		return repoRef{}, fmt.Errorf("invalid repository %q, want owner/name", s)
	}

	return repoRef{owner: owner, name: name}, nil
}

// cut is strings.Cut, which go.mod's Go version predates.
func cut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}

	return s, "", false
}

// resolveRepo prefers the repository input, falling back to the repository the
// workflow itself runs in.
func resolveRepo(lookup func(string) string) (repoRef, error) {
	for _, key := range []string{"INPUT_REPOSITORY", "GITHUB_REPOSITORY"} {
		if v := lookup(key); v != "" {
			return parseRepo(v)
		}
	}

	return repoRef{}, errors.New("no repository configured: set the repository input or GITHUB_REPOSITORY")
}

// resolveAuthors returns the logins whose issues get published, defaulting to
// the repository owner so a repo that takes outside issues does not publish
// posts its owner never wrote. "*" disables the filter.
func resolveAuthors(lookup func(string) string, repo repoRef) []string {
	raw := lookup("INPUT_AUTHORS")
	if raw == "" {
		return []string{repo.owner}
	}
	if strings.TrimSpace(raw) == "*" {
		return nil
	}

	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	if len(out) == 0 {
		return []string{repo.owner}
	}

	return out
}

// resolveToken prefers the action input, falling back to the ambient token so
// the binary is also usable outside the action.
func resolveToken(lookup func(string) string) string {
	for _, key := range []string{"INPUT_TOKEN", "GITHUB_TOKEN"} {
		if v := lookup(key); v != "" {
			return v
		}
	}

	return ""
}

// newHTTPClient returns nil for an empty token, which makes github.NewClient
// fall back to an anonymous default client.
func newHTTPClient(token string) *http.Client {
	if token == "" {
		return nil
	}

	return &http.Client{Transport: tokenTransport{token: token}}
}

// newClient authenticates when a token is available. Anonymous requests are
// capped at 60 per hour per IP, shared with every other Actions job on the same
// runner, which is not enough to be reliable: exhausting it used to fail the
// run and take the published posts with it.
func newClient(token string) *github.Client {
	return github.NewClient(newHTTPClient(token))
}

func main() {
	repo, err := resolveRepo(os.Getenv)
	check(err)

	authors := resolveAuthors(os.Getenv, repo)
	preserved := resolvePreservedCase(os.Getenv)

	client := newClient(resolveToken(os.Getenv))
	ctx := context.Background()

	opts := &github.IssueListByRepoOptions{
		State:       "closed",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	fmt.Printf("generating posts from %s/%s issues\n", repo.owner, repo.name)

	issues, err := listAllIssues(func(o *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
		return client.Issues.ListByRepo(ctx, repo.owner, repo.name, o)
	}, opts)

	check(err)

	check(os.RemoveAll("dist"))
	check(os.MkdirAll("dist/_posts", os.ModePerm))
	items := make(map[string][]IssueItem)
	numberOfValidIssues := 0

	for _, issue := range issues {
		if checkIssue(issue, authors) == false {
			fmt.Println(*issue.ID, "is skipped since it's an invalid issue.")
			continue
		}

		filePrefix := "dist/"
		filePostfix := "_posts/"
		check(os.MkdirAll(filePrefix+filePostfix, os.ModePerm))
		filePath := filePrefix + filePostfix + fileLink(issue)

		category := ""
		if len(issue.Labels) > 0 {
			category = *issue.Labels[0].Name
		}

		check(os.WriteFile(filePath, []byte(frontMatter("post", *issue.Title, category, *issue.CreatedAt)+*issue.Body+"\n\n---\n"+mklink("discussion", *issue.HTMLURL)+"\n"), 0644))

		if len(issue.Labels) == 0 {
			category = "misc"
		}

		item := IssueItem{category, parmaLink(issue), *issue.Title}
		items[category] = append(items[category], item)
		numberOfValidIssues++
	}
	if len(items) > 0 {
		generateReadme(numberOfValidIssues, items, preserved)
	}
}
