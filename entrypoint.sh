#!/bin/sh -l

/app/til-cli
cp -r dist/* $GITHUB_WORKSPACE/
rm -rf dist # clean up
