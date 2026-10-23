---
title: "ping"
description: "Ping an IPv4 address or DNS name from inside a pod."
category: Kubernetes
weight: 14
---

```
kubectl ping POD DESTINATION [-n NAMESPACE] [-c CONTAINER] [--count N] [--size BYTES] [--timeout SECONDS]
```

Runs `ping` inside a running pod, so the traffic originates from the pod's network namespace. That makes it useful for checking egress, network policies, routes and DNS-free connectivity from exactly where a workload lives.

The source is always a pod. The destination is an IPv4 address or a DNS name. Names are resolved by the pod itself, so cluster-internal names work, for example `my-svc.my-ns.svc.cluster.local`, a headless-service pod name, or a plain `my-svc` within the pod's namespace. IPv6 addresses and malformed names are rejected before anything is sent to the cluster.

Note that a ClusterIP service address only answers ping if something is responding to ICMP on it; in many clusters it won't, even when the service is healthy. A failed ping to a service name is not proof the service is down.

## Requirements

The container must have a `ping` binary (iputils or busybox). Minimal and distroless images often don't, in which case the plugin reports that ping may not be installed. Pick another container with `-c`, or ping from a different pod.

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace of the pod (default: current context namespace) |
| `-c, --container` | Container to run ping in (default: the pod's default container) |
| `--count` | Number of packets to send (default 4, max 1000) |
| `--size` | Payload size in bytes (0 uses the ping default, max 65507) |
| `--timeout` | Seconds to wait for each reply (default 2) |

## Examples

```sh
kubectl ping web-7d4b9 8.8.8.8
kubectl ping web-7d4b9 example.com
kubectl ping web-7d4b9 my-svc.my-ns.svc.cluster.local
kubectl ping web-7d4b9 10.0.0.5 -n prod -c app --count 10
kubectl ping web-7d4b9 10.0.0.5 --size 1400 --timeout 1
```

## Sample output

```
$ kubectl ping web-7d4b9 8.8.8.8 --count 3
Pinging 8.8.8.8 from pod default/web-7d4b9
PING 8.8.8.8 (8.8.8.8): 56 data bytes
64 bytes from 8.8.8.8: seq=0 ttl=113 time=4.512 ms
64 bytes from 8.8.8.8: seq=1 ttl=113 time=4.201 ms
64 bytes from 8.8.8.8: seq=2 ttl=113 time=4.388 ms

--- 8.8.8.8 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
```

The "Pinging ..." line goes to stderr, so `kubectl ping ... 2>/dev/null` leaves only ping's output.

Ping's output is streamed back as it runs, and the plugin exits with ping's own exit code, so it works in scripts.
