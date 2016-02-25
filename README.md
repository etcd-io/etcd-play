## etcd-play [![Build Status](https://img.shields.io/travis/coreos/etcd-play.svg?style=flat-square)](https://travis-ci.org/coreos/etcd-play) [![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/coreos/etcd-play)

<a href="http://play.etcd.io" href="_blank">play.etcd.io</a>

Playground for distributed database [`etcd`](https://github.com/coreos/etcd).

- [Get started](#get-started)
- [play `etcd` in terminal](#play-etcd-in-terminal)
- [play `etcd` in web browser](#play-etcd-in-web-browser)
- [10 ways to kill `etcd`](#10-ways-to-kill-etcd)
	- [kill leader](#kill-leader)

<br>
#### Get started

```
# install
go get -v -u github.com/coreos/etcd
go get -v -u github.com/coreos/etcd/tools/functional-tester/etcd-agent
go get -v -u github.com/coreos/etcd-play


etcd-play terminal    # run in terminal
etcd-play web         # run in your web browser (localhost)


# run with remote machines
etcd-agent  # deploy in machine1
etcd-agent  # deploy in machine2
etcd-agent  # deploy in machine3
etcd-agent  # deploy in machine4
etcd-agent  # deploy in machine5

AGENT_RPC_ENDPOINTS='10.0.0.1:9027,10.0.0.2:9027,10.0.0.3:9027,10.0.0.4:9027,10.0.0.5:9027'
etcd-play web --keep-alive --linux-auto-port=false --production --remote --agent-endpoints="$(echo $AGENT_RPC_ENDPOINTS)" 
```

[↑ top](#etcd-play--)
<br><br><br><br><hr>


#### play `etcd` in terminal

<img src="https://storage.googleapis.com/play-etcd/terminal_20160225020001.gif" alt="terminal_20160225020001" style="width: 500px; height: 300px;"/>


[↑ top](#etcd-play--)
<br><br><br><br><hr>


#### play `etcd` in web browser

<img src="https://storage.googleapis.com/play-etcd/web_20160225020001.gif" alt="web_20160225020001"/>


[↑ top](#etcd-play--)
<br><br><br><br><hr>


#### 10 ways to kill `etcd`

<br>
##### kill leader

[↑ top](#etcd-play--)
<br>

##### ...

[↑ top](#etcd-play--)
<br>

##### ...

[↑ top](#etcd-play--)
<br>

##### ...

[↑ top](#etcd-play--)
<br>

##### ...

[↑ top](#etcd-play--)
<br>

##### ...

[↑ top](#etcd-play--)
<br><br><br><br><hr>
