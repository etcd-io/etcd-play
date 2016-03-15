FROM golang:1.6
RUN go get -v -u -f github.com/coreos/etcd
RUN go get -v -u -f github.com/coreos/etcd/tools/functional-tester/etcd-agent
RUN go get -v -u -f github.com/coreos/etcd-play
EXPOSE 8000
