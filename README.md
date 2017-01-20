This is an experimental project. Project/code is subject to change anytime.

# etcd-play

[![Build Status](https://img.shields.io/travis/coreos/etcd-play.svg?style=flat-square)][cistat] [![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)][etcd-play-godoc]   

etcd-play is a playground for exploring the [`etcd`][etcd-home] distributed key-value database. Try it out live at [play.etcd.io][play-etcd].

## Play with `etcd` in a web browser

`etcd` uses the [Raft consensus algorithm][raft-home] to replicate data on distributed machines in order to gracefully handle network partitions, node failures, and even leader failures. The etcd team extensively tests failure scenarios in the [etcd functional test suite][etcd-functests]. Real-time results from this testing are available at the [etcd test dashboard][etcd-dash].

In Raft, followers are passive, only responding to incoming RPCs. Clients can make requests to any node, follower or leader. Followers, in turn, forward requests to their leader. Last, the leader appends those requests (commands) to its log and sends `AppendEntries` RPCs to all of its followers.

### Follower failures

What if followers fail?

The leader retries RPCs until they succeed. As soon as a follower recovers, it will catch up with the leader.

<img src="https://storage.googleapis.com/etcd/assets-etcd-play-old/follower-failures-20160307.gif" alt="follower-failures"/>

In the animation above, notice the stress on the remaining nodes while two of followers (*etcd1* and *etcd2*) are down. Nevertheless, notice that all data is replicated across the cluster, except those two failed ones. Immediately after the nodes recover, the followers sync their data from the leader, looping on this process until all hashes match.

### Leader failure

What if a leader fails?

A leader sends periodic heartbeat messages to its followers to maintain its authority. If a follower has not received heartbeats from a valid leader within the election timeout, it assumes that there is no current leader in the cluster, and becomes a candidate to start a new election. Each node includes its last term and last log index in its `RequestVote` RPC, so that Raft can choose the candidate that is most likely to contain all committed entries. When the old leader recovers, it will retry to commit log entries of its own. The Raft *term* is used to detect these stale leaders: Followers deny RPCs if the sender's term is older, and then the sender (often the old leader) reverts back to follower state and updates its term to the latest cluster term.

<img src="https://storage.googleapis.com/etcd/assets-etcd-play-old/leader-failure-20160303.gif" alt="leader-failure"/>

The animation above shows the Leader going down, and shortly a new leader is elected.

### All nodes failure

`etcd` is highly available as long as a quorum of cluster members are operational and can communicate with each other and with clients. 5-node clusters can tolerate failures of any two members. Data loss is still possible in catastrophic events, like all nodes failing. `etcd` persists enough information on stable storage so that members can recover safely from the disk and rejoin the cluster. In particular, `etcd` stores new log entries onto disk before committing them to the log to prevent committed entries from being lost on an unexpected restart.

<img src="https://storage.googleapis.com/etcd/assets-etcd-play-old/all-nodes-failures-20160307.gif" alt="all-node-failures"/>

The animation above shows all nodes being terminated with the `Kill` button. `etcd` recovers the data from stable storage. You can see the number of keys and hash values match, before and after. The cluster can handle client requests immediately after recovery with a new leader.

[cistat]: https://travis-ci.org/coreos/etcd-play
[etcd-dash]: http://dash.etcd.io
[etcd-functests]: https://github.com/coreos/etcd/tree/master/tools/functional-tester
[etcd-home]: https://coreos.com/etcd/
[etcd-play-godoc]: https://godoc.org/github.com/coreos/etcd-play
[play-etcd]: http://play.etcd.io
[raft-home]: https://raft.github.io
