FROM golang:1.6-onbuild
RUN go get -v -u -f github.com/coreos/etcd
EXPOSE 8000
