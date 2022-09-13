package main

import (
	"context"
	"fmt"
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
		categories += "* [" + strcase.ToCamel(k) + "](#" + k + ")\n"
		table += mkheader(2, strcase.ToCamel(k))
		for _, item := range items[k] {
			line := fmt.Sprintf("* [%s](%s) \n", item.title, item.path)
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

func frontMatter(layout string, title string, category string, date time.Time) string {
	return fmt.Sprintf("---\nlayout: %s\ntitle: %s\ndate: %s\ncategory: %s\n---\n\n", layout, title, date.Format("2006-01-02 15:04:05 +0000"), category)
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

func main() {
	client := github.NewClient(nil)

	issues, _, err := client.Issues.ListByRepo(context.Background(), "junkpiano", "til", &github.IssueListByRepoOptions{State: "closed"})

	check(err)

	check(os.RemoveAll("dist"))
	check(os.MkdirAll("dist/_posts", os.ModePerm))
	items := make(map[string][]IssueItem)
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
	}
	if len(items) > 0 {
		generateReadme(len(issues), items)
	}
}
