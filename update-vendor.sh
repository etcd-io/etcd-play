#!/usr/bin/env bash
set -e

GLIDE_ROOT="$GOPATH/src/github.com/Masterminds/glide"
rm -rf $GLIDE_ROOT
go get -v -u github.com/Masterminds/glide
go get -v -u github.com/sgotti/glide-vc
pushd "${GLIDE_ROOT}"
	git reset --hard HEAD
	go install
popd

rm -rf vendor

glide --verbose update --delete --strip-vendor --strip-vcs --update-vendored --skip-test
glide vc --only-code --no-tests
