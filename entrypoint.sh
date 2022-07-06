#!/bin/sh -l

/app/til-cli
rm -rf $GITHUB_WORKSPACE/_posts 
cp -r dist/* $GITHUB_WORKSPACE/
rm -rf dist # clean up
