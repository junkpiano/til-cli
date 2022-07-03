#!/bin/sh -l

echo "Hello $1"
time=$(date)
echo "::set-output name=time::$time"

/app/til-cli
cp -r dist/* $GITHUB_WORKSPACE/
rm -rf dist # clean up
