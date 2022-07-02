package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

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

	if len(issue.Labels) > 0 {
		l += *issue.Labels[0].Name + "/"
	}

	l += strconv.FormatInt(*issue.ID, 10) + ".md"

	return l
}

func generateReadme(numberOfIssues int, items map[string][]IssueItem) {
	title := mkheader(1, "Today I Learned")
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
	check(os.WriteFile("dist/README.md", []byte(content), 0644))

	fmt.Println(content)
}

func mkheader(level int, str string) string {
	hashes := strings.Repeat("#", level)
	hashes += " " + str + "\n\n"
	return hashes
}

func main() {
	client := github.NewClient(nil)

	issues, _, err := client.Issues.ListByRepo(context.Background(), "junkpiano", "til", nil)

	check(err)

	check(os.RemoveAll("dist"))
	items := make(map[string][]IssueItem)
	for _, issue := range issues {
		if len(issue.Labels) == 0 || *issue.User.Login != "junkpiano" {
			fmt.Println(*issue.ID, " is skipped since it's an invalid issue.")
			continue
		}

		category := *issue.Labels[0].Name

		err := os.MkdirAll("dist/"+category, os.ModePerm)
		check(err)
		filePath := "dist/" + fileLink(issue)

		check(os.WriteFile(filePath, []byte(mkheader(1, *issue.Title)+*issue.Body+"\n"), 0644))

		item := IssueItem{category, fileLink(issue), *issue.Title}
		items[category] = append(items[category], item)
	}
	generateReadme(len(issues), items)
}
