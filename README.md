## etcd-play [![Build Status](https://img.shields.io/travis/coreos/etcd-play.svg?style=flat-square)](https://travis-ci.org/coreos/etcd-play) [![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/coreos/etcd-play)

<a href="http://play.etcd.io" target="_blank">play.etcd.io</a>

Playground for distributed database [`etcd`](https://github.com/coreos/etcd).

- [Get started](#get-started)
- [play `etcd` in terminal](#play-etcd-in-terminal)
- [play `etcd` in web browser](#play-etcd-in-web-browser)
- [explain `Raft` with `etcd`](#explain-raft-with-etcd)
	- [Follower failures](#follower-failures)
	- [Leader failure](#leader-failure)
	- [All node failures](#all-node-failures)

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

<img src="https://storage.googleapis.com/play-etcd/terminal.png" alt="terminal" width="570"/>

[↑ top](#etcd-play--)
<br><br><br><br><hr>


#### play `etcd` in web browser

<img src="https://storage.googleapis.com/play-etcd/web_20160225111501.gif" alt="web"/>

[↑ top](#etcd-play--)
<br><br><br><br><hr>


#### explain `Raft` with `etcd`

Using the [Raft consensus algorithms](https://raft.github.io), `etcd`
replicates your data in distributed machines in order to handle network
partitions and machine failures, even leader failures. And etcd team
has been extensively testing these failures in
[functional-test suite](https://github.com/coreos/etcd/tree/master/tools/functional-tester).
Real-time results can be found at [dash.etcd.io](http://dash.etcd.io).

In Raft, followers are completely passive, only responding to incoming
RPCs. Clients can request to any nodes(followers or leader). Then followers
forward the requests to its leader. And the leader appends those
requests(commands) to its log and sends `AppendEntries` RPCs to its followers.

<br>
##### Follower failures

What if followers fail? Leader retries RPCs until they succeed. As soon as
follower recovers, it will catch up with the leader.

<img src="https://storage.googleapis.com/play-etcd/follower_failures_20160225133000.gif" alt="follower_failures"/>

You can see me stressing the follower node(etcd3) while two
followers(etcd1, etcd2) are down. And it shows all data are being replicated
across the cluster, except the two failed ones. Right after recover, the
followers syncs their data from its leader(all hashes are matching).

<br>
##### Leader failure

What if leader fails? A leader sends periodic heartbeat messages to its followers
to maintain its authority. If a follower has not received heartbeats from a
valid leader within election timeout, it assumes that there is no current
leader in the cluster, and becomes a candidate to start a new election. Each
node includes last term and last log index in `RequestVote` RPC, so that
Raft can choose the candidate that is most likely to contain all committed
entries. When the old leader recovers, it will retry to commit log entries of
its own. Raft term is used to detect these stale leaders: followers denies RPCs
if the sender's term is older, and then sender(or the old leader) reverts back
to follower state and updates its term.

<img src="https://storage.googleapis.com/play-etcd/leader_failure_20160225133000.gif" alt="leader_failure"/>

Leader goes down, but shortly new leader gets elected.

<br>
##### All node failures

`etcd` is highly available, as long as quorum of cluster are operational and can
communicate with each member and clients. 5-node cluster can tolerate failures of
any two servers. Data loss is still possible in catastrophic events, like all node
failures. `etcd` persists enough information on stable storage so that members can
recover safely from the disk and rejoin the cluster. Especially `etcd` stores new
log entries onto disk before commitment to prevent committed entries from being lost
or uncommitted on server restart.

<img src="https://storage.googleapis.com/play-etcd/all_node_failures_20160225133000.gif" alt="all_node_failures"/>

All nodes terminated. And `etcd` is recovering the data from stable storage.
You can see number of keys and hashes values are matching before and after.
It can handle client requests right after recovery with a new leader.

[↑ top](#etcd-play--)
<br><br><br><br><hr>
