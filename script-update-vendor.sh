#!/usr/bin/env bash
set -e

echo "Installing glide..."
GLIDE_ROOT="$GOPATH/src/github.com/Masterminds/glide"
rm -rf $GLIDE_ROOT
go get -u github.com/Masterminds/glide
pushd "${GLIDE_ROOT}"
	git reset --hard HEAD
	go install
popd

rm -rf vendor
glide -v
glide update --strip-vendor --strip-vcs --update-vendored
glide vc --only-code --no-tests

