# Clustering Guide

Run OLB as a multi-node cluster for high availability and distributed state management.

## Architecture Overview

OLB clustering uses two protocols:

- **Raft consensus** -- Strong consistency for configuration state (routes, backends, TLS certificates). Only the leader processes config changes; followers replicate.
- **Gossip (SWIM)** -- Eventual consistency for operational state (health status, metrics, rate limit counters, session affinity tables). All nodes exchange state continuously.

```
              Raft (config state)
       ┌──────────┬──────────┐
       │          │          │
  ┌────▼────┐ ┌───▼────┐ ┌──▼─────┐
  │ Node 1  │ │ Node 2 │ │ Node 3 │
  │ (Leader)│ │(Follow)│ │(Follow)│
  │ :80/443 │ │ :80/443│ │ :80/443│
  │ :9090   │ │ :9090  │ │ :9090  │
  └────┬────┘ └───┬────┘ └──┬─────┘
       │          │          │
       └──────────┴──────────┘
          Gossip (health, metrics)
```

All nodes accept proxy traffic. A load balancer or DNS round-robin in front distributes clients across all nodes.

## What Gets Clustered

| Data | Protocol | Consistency | Notes |
|------|----------|-------------|-------|
| Config (routes, backends, TLS) | Raft | Strong (linearizable) | Changes only via leader |
| Health status | Gossip | Eventual | Propagates within seconds |
| Metrics | Local | Eventual (aggregated on query) | Each node keeps local metrics |
| Rate limit counters | Gossip (CRDT) | Eventual | Slightly over-limit is possible |
| Session affinity tables | Gossip | Eventual | Sticky sessions work cross-node |

## Setting Up a 3-Node Cluster

A minimum of 3 nodes is recommended for fault tolerance (tolerates 1 node failure).

### Node 1 (initial leader)

Create `/etc/olb/olb.yaml` on node 1 (`10.0.0.1`):

```yaml
version: 1

listeners:
  - name: http
    protocol: http
    address: ":80"
    routes:
      - name: default
        path: /
        pool: backend

pools:
  - name: backend
    algorithm: round_robin
    health_check:
      type: http
      path: /health
      interval: 10s
    backends:
      - id: app-1
        address: "10.0.1.1:8080"
      - id: app-2
        address: "10.0.1.2:8080"

admin:
  enabled: true
  address: "0.0.0.0:9090"

cluster:
  enabled: true
  node_name: node-1
  bind_address: "0.0.0.0:7946"
  raft_address: "0.0.0.0:7947"
  peers:
    - "10.0.0.1:7946"
    - "10.0.0.2:7946"
    - "10.0.0.3:7946"

logging:
  level: info
  output: stdout
```

### Node 2

Create `/etc/olb/olb.yaml` on node 2 (`10.0.0.2`), changing only `node_name`:

```yaml
version: 1

listeners:
  - name: http
    protocol: http
    address: ":80"
    routes:
      - name: default
        path: /
        pool: backend

pools:
  - name: backend
    algorithm: round_robin
    health_check:
      type: http
      path: /health
      interval: 10s
    backends:
      - id: app-1
        address: "10.0.1.1:8080"
      - id: app-2
        address: "10.0.1.2:8080"

admin:
  enabled: true
  address: "0.0.0.0:9090"

cluster:
  enabled: true
  node_name: node-2
  bind_address: "0.0.0.0:7946"
  raft_address: "0.0.0.0:7947"
  peers:
    - "10.0.0.1:7946"
    - "10.0.0.2:7946"
    - "10.0.0.3:7946"

logging:
  level: info
  output: stdout
```

### Node 3

Same as above with `node_name: node-3` on host `10.0.0.3`.

### Starting the Cluster

Start all three nodes:

```bash
# On each node
olb start --config /etc/olb/olb.yaml
```

The nodes discover each other via the `peers` list. Raft leader election happens automatically. Check cluster status:

```bash
olb cluster status
```

```
Cluster: olb-cluster
State:   healthy
Leader:  node-1 (10.0.0.1:7947)
Term:    1
Nodes:   3/3 healthy

NAME      ADDRESS         STATE      RAFT ROLE   UPTIME
node-1    10.0.0.1:7946   alive      leader      2h 15m
node-2    10.0.0.2:7946   alive      follower    2h 14m
node-3    10.0.0.3:7946   alive      follower    2h 14m
```

## Joining and Leaving Nodes

### Adding a New Node

1. Install OLB on the new host and create its config with `cluster.enabled: true`.
2. Join the cluster:

```bash
# On the new node
olb cluster join 10.0.0.1:7946

# Or via API on any existing node
curl -X POST http://10.0.0.1:9090/api/v1/cluster/join \
  -H "Content-Type: application/json" \
  -d '{"address": "10.0.0.4:7946"}'
```

The new node receives the current config via Raft log replay and begins accepting traffic.

### Removing a Node

```bash
# On the node being removed (graceful)
olb cluster leave

# Or from any other node
curl -X POST http://10.0.0.1:9090/api/v1/cluster/leave \
  -H "Content-Type: application/json" \
  -d '{"node": "node-4"}'
```

The node drains active connections before departing.

## Leader Election and Failover

Raft handles leader election automatically:

1. Each follower has a randomized election timeout (150-300ms).
2. If a follower does not receive a heartbeat from the leader within its timeout, it becomes a candidate and starts an election.
3. A candidate that receives votes from a majority of nodes becomes the new leader.
4. The new leader begins sending heartbeats and accepting config changes.

**Failover time:** Typically 150-500ms from leader failure to new leader election.

**What happens during failover:**
- Proxy traffic continues on all nodes (no interruption).
- Config changes (adding backends, modifying routes) are queued until a new leader is elected.
- Health status continues to propagate via gossip (independent of Raft).

## Distributed State

### Config Changes in a Cluster

Config changes must go through the Raft leader. When you modify config on any node, the request is forwarded to the leader:

```bash
# This works on any node -- it forwards to the leader
olb backend add web-pool 10.0.1.3:8080 --weight 2

# API calls also forward to the leader
curl -X POST http://10.0.0.2:9090/api/v1/backends/web-pool \
  -d '{"address": "10.0.1.3:8080", "weight": 2}'
```

The leader commits the change to the Raft log, replicates it to followers, and applies it once a majority acknowledge.

### Health Status

Health check results propagate via gossip. Each node runs its own health checks and shares results. The cluster converges on a consistent view of backend health within a few seconds.

### Rate Limiting

Distributed rate limiting uses CRDT (Conflict-free Replicated Data Type) counters. Each node tracks local request counts and shares them via gossip. The global rate is the sum across all nodes.

Due to eventual consistency, the actual limit may be slightly exceeded during propagation. This is acceptable for most rate limiting use cases.

## Monitoring Cluster Health

### CLI

```bash
# Cluster overview
olb cluster status

# Member list with details
olb cluster members
```

### API

```bash
# Cluster status
curl http://localhost:9090/api/v1/cluster/status

# Raft state
curl http://localhost:9090/api/v1/cluster/raft

# Member list
curl http://localhost:9090/api/v1/cluster/members
```

### Metrics

Key cluster metrics exposed via Prometheus:

```
olb_cluster_nodes{state="alive"}    3
olb_cluster_raft_term               1
olb_cluster_raft_leader             1    # 1 = this node is leader
```

## Security

### Inter-Node mTLS

Enable TLS for all cluster communication:

```yaml
cluster:
  tls:
    enabled: true
    cert: /etc/olb/cluster.crt
    key: /etc/olb/cluster.key
    ca: /etc/olb/cluster-ca.crt
```

Generate certificates using your CA. All nodes must trust the same CA.

### Generating Cluster Certificates

```bash
# Generate CA
openssl genrsa -out cluster-ca.key 4096
openssl req -x509 -new -nodes -key cluster-ca.key -sha256 \
  -days 3650 -out cluster-ca.crt -subj "/CN=OLB Cluster CA"

# Generate node certificate (repeat for each node)
openssl genrsa -out node-1.key 2048
openssl req -new -key node-1.key -out node-1.csr \
  -subj "/CN=node-1" \
  -addext "subjectAltName=IP:10.0.0.1"
openssl x509 -req -in node-1.csr -CA cluster-ca.crt -CAkey cluster-ca.key \
  -CAcreateserial -out node-1.crt -days 365 -sha256
```

## Troubleshooting

### Cluster will not form

- Verify all nodes can reach each other on the gossip port (default 7946) and Raft port (default 7947).
- Check firewall rules: `nc -zv 10.0.0.2 7946` and `nc -zv 10.0.0.2 7947`.
- Ensure `peers` list is identical on all nodes and includes all node addresses.
- Check logs for `"failed to connect to peer"` messages.

### Split brain (two leaders)

This should not happen with a correct Raft implementation, but if it does:
- Check network connectivity between nodes.
- Ensure an odd number of nodes (3, 5, 7) to avoid split votes.
- Restart the minority partition; it will rejoin as followers.

### Node stuck in "candidate" state

- The node cannot reach a majority. Check network connectivity.
- Verify the `raft_address` is reachable from other nodes.
- Check if a firewall is blocking the Raft port.

### Config changes not propagating

- Verify the leader is healthy: `olb cluster status`.
- Check Raft log replication: `curl http://localhost:9090/api/v1/cluster/raft`.
- Look for `"log replication failed"` in the leader's logs.

### High gossip traffic

- Increase the gossip interval (trade freshness for bandwidth).
- Reduce the number of metadata items being gossiped.
- Check for nodes rapidly joining/leaving (flapping).
