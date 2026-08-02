#!/bin/sh -l
set -e

/app/til-cli

# Only touch the published posts once generation has actually produced some.
# Without this guard a til-cli failure (an API error, say) left dist/ empty,
# and the rm below then wiped _posts and copied nothing back — the action
# reported success while deleting the whole site.
if [ ! -d dist/_posts ] || [ -z "$(ls -A dist/_posts)" ]; then
	echo "til-cli produced no posts; leaving $GITHUB_WORKSPACE untouched" >&2
	exit 1
fi

rm -rf "$GITHUB_WORKSPACE/_posts"
cp -r dist/* "$GITHUB_WORKSPACE/"
rm -rf dist # clean up
