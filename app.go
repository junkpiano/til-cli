package main

import (
	"context"
	"fmt"
	"os"
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
	l += *issue.Labels[0].Name + "/"
	t := (*issue.CreatedAt).Format("2006/01/02")
	filename := strings.Join([]string{t, strconv.FormatInt(*issue.ID, 10)}, "/")
	l += filename + ".html"

	return l
}

func generateReadme(numberOfIssues int, items map[string][]IssueItem) {
	title := frontMatter("page", "Archives", time.Now())
	tagline := fmt.Sprintf("*%d TILs, and counting...*", numberOfIssues)
	categories := mkheader(2, "Category")
	table := ""

	for k, v := range items {
		categories += "* [" + k + "](#" + k + ")\n"
		table += mkheader(2, strcase.ToCamel(k))
		for _, item := range v {
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

func frontMatter(layout string, title string, date time.Time) string {
	return fmt.Sprintf("---\nlayout: %s\ntitle: %s\ndate: %s\n---\n\n", layout, title, date.Format("2006-01-02 15:04:05 +0000"))
}

func main() {
	client := github.NewClient(nil)

	issues, _, err := client.Issues.ListByRepo(context.Background(), "junkpiano", "til", &github.IssueListByRepoOptions{State: "closed"})

	check(err)

	check(os.RemoveAll("dist"))
	items := make(map[string][]IssueItem)
	for _, issue := range issues {
		if len(issue.Labels) == 0 || *issue.User.Login != "junkpiano" {
			fmt.Println(*issue.ID, "is skipped since it's an invalid issue.")
			continue
		}

		category := *issue.Labels[0].Name
		filePrefix := "dist/"
		filePostfix := "/_posts/"
		err := os.MkdirAll(filePrefix+category+filePostfix, os.ModePerm)
		check(err)
		filePath := filePrefix + category + filePostfix + fileLink(issue)

		check(os.WriteFile(filePath, []byte(frontMatter("post", *issue.Title, *issue.CreatedAt)+*issue.Body+"\n\n---\n"+mklink("discussion", *issue.HTMLURL)+"\n"), 0644))

		item := IssueItem{category, parmaLink(issue), *issue.Title}
		items[category] = append(items[category], item)
	}
	if len(items) > 0 {
		generateReadme(len(issues), items)
	}
}
