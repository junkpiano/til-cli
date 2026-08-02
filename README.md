# TiL-ClI

This action generates jekyll posts from github issues.

Closed issues become posts under `_posts/`, the first label of each issue
becomes its category, and an `archives.md` index is generated alongside them.
Here is my weblog. -> <https://junkpiano.github.io/til>

## Usage

```yaml
- uses: junkpiano/til-cli@v1
  with:
    token: ${{ secrets.GITHUB_TOKEN }}
```

By default it reads issues from the repository the workflow runs in, and
publishes only those opened by the repository owner.

## Inputs

| input | default | description |
| --- | --- | --- |
| `token` | none | Token used to read issues. Without it the API is called anonymously, which is limited to 60 requests per hour per runner IP, shared with every other Actions job on that host. |
| `repository` | the workflow's repository | Repository to read issues from, as `owner/name`. |
| `authors` | the repository owner | Comma separated logins whose issues get published. Set to `*` to publish issues from anyone. |
| `preserve-case` | `iOS` | Comma separated labels used verbatim as category headings instead of being camel cased. |

Reading issues from another repository, or from a private one, needs a token
with access to it — the default `GITHUB_TOKEN` is scoped to the repository
running the workflow.

## Example

```yaml
name: sync

on:
  issues:
    types: [closed, edited, labeled, reopened, unlabeled]

concurrency:
  group: sync

jobs:
  sync:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      issues: read
    steps:
      - uses: actions/checkout@v3
      - uses: junkpiano/til-cli@v1
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
      - uses: EndBug/add-and-commit@v9
        with:
          default_author: github_actions
          message: "auto update by github action"
```
