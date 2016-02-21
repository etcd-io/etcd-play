#### Install

```
go get -v -u github.com/coreos/etcd
go get -v -u github.com/coreos/etcd/tools/functional-tester/etcd-agent
go get -v -u github.com/coreos/etcd-play
```

There are multiple ways to play etcd.


<br>
#### Play `etcd` in local terminal

```
etcd-play terminal
```

![terminal](screenshots/terminal.png)


<br>
#### Play etcd with local web browser


```
etcd-play web
```

Web demo will be served at <a href="http://localhost:8000" href="_blank">http://localhost:8000</a>

![local](screenshots/local.png)


<br>
#### Public etcd

```
etcd-agent  # deploy in machine1
etcd-agent  # deploy in machine2
etcd-agent  # deploy in machine3
etcd-agent  # deploy in machine4
etcd-agent  # deploy in machine5

AGENT_RPC_ENDPOINTS='10.10.10.1:9027,10.10.10.2:9027,10.10.10.3:9027,10.10.10.4:9027,10.10.10.5:9027'
etcd-play web --keep-alive --production --remote --agent-endpoints="$(echo $AGENT_RPC_ENDPOINTS)" 
```

Go to <a href="https://play.etcd.io" href="_blank">https://play.etcd.io</a>

![web](screenshots/web.png)
