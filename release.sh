#!/usr/bin/env bash
set -e

VERSION=$1
if [ -z "${VERSION}" ]; then
	echo "Usage: ${0} VERSION" >> /dev/stderr
	exit 255
fi

# A non-installed actool can be used, for example:
# ACTOOL=../../appc/spec/bin/actool
ACTOOL=${ACTOOL:-actool}
if ! command -v $ACTOOL >/dev/null; then
    echo "cannot find actool ($ACTOOL)"
    exit 1
fi

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

###################################
RELEASE_DIR=$GOPATH/src/github.com/coreos/etcd-play/release
DOCKER_DIR=$GOPATH/src/github.com/coreos/etcd-play/release/image-docker
mkdir -p $DOCKER_DIR

###################################
echo Building etcd binary...
go build github.com/coreos/etcd-play
mv etcd-play $RELEASE_DIR/etcd-play

###################################
echo Building aci image...
acbuild --debug begin

TMPHOSTS="$(mktemp)"

acbuildEnd() {
    rm "$TMPHOSTS"
    export EXIT=$?
    acbuild --debug end && exit $EXIT 
}
trap acbuildEnd EXIT

cat <<DF > $TMPHOSTS
127.0.0.1   localhost localhost.localdomain localhost4 localhost4.localdomain4
DF

acbuild --debug set-name coreos.com/etcd-play
acbuild --debug copy --to-dir $RELEASE_DIR/etcd-play /
acbuild --debug label add version "$VERSION"
acbuild --debug set-exec -- /etcd-play
acbuild --debug port add web tcp 8000
acbuild --debug copy "$TMPHOSTS" /etc/hosts
acbuild --debug write --overwrite $RELEASE_DIR/etcd-play-${VERSION}-linux-amd64.aci

###################################
echo Building docker image...
cp $RELEASE_DIR/etcd-play $DOCKER_DIR/etcd-play

cat <<DF > ${DOCKER_DIR}/Dockerfile
FROM scratch
ADD etcd-play /
EXPOSE 8000
ENTRYPOINT ["/etcd-play"]
DF

docker build -t quay.io/coreos/etcd-play:${VERSION} ${DOCKER_DIR}
