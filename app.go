package main

import (
	"context"
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

func findReserved(key string) string {
	reserved := make(map[string]string)
	reserved["iOS"] = "iOS"

	if val, ok := reserved[key]; ok {
		return val
	}

	return strcase.ToCamel(key)
}

func generateReadme(numberOfIssues int, items map[string][]IssueItem) {
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
		key := findReserved(k)
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

func checkIssue(issue *github.Issue) bool {
	if *issue.User.Login == "junkpiano" &&
		issue.PullRequestLinks == nil &&
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
	client := newClient(resolveToken(os.Getenv))
	ctx := context.Background()

	opts := &github.IssueListByRepoOptions{
		State:       "closed",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	issues, err := listAllIssues(func(o *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
		return client.Issues.ListByRepo(ctx, "junkpiano", "til", o)
	}, opts)

	check(err)

	check(os.RemoveAll("dist"))
	check(os.MkdirAll("dist/_posts", os.ModePerm))
	items := make(map[string][]IssueItem)
	numberOfValidIssues := 0

	for _, issue := range issues {
		if checkIssue(issue) == false {
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
		generateReadme(numberOfValidIssues, items)
	}
}
