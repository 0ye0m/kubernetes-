# Compute Resources

** Table of Contents**
- Compute Resources
  - [Container and Pod Resource Limits](http://releases.k8s.io/HEAD/docs/#container-and-pod-resource-limits)
  - [How Pods with Resource Limits are Scheduled](http://releases.k8s.io/HEAD/docs/#how-pods-with-resource-limits-are-scheduled)
  - [How Pods with Resource Limits are Run](http://releases.k8s.io/HEAD/docs/#how-pods-with-resource-limits-are-run)
  - [Monitoring Compute Resource Usage](http://releases.k8s.io/HEAD/docs/#monitoring-compute-resource-usage)
  - [Troubleshooting](http://releases.k8s.io/HEAD/docs/#troubleshooting)
  - [Planned Improvements](http://releases.k8s.io/HEAD/docs/#planned-improvements)

When specifying a [pod](http://releases.k8s.io/HEAD/docs/./pods.md), you can optionally specify how much CPU and memory (RAM) each
container needs.  When containers have resource limits, the scheduler is able to make better
decisions about which nodes to place pods on, and contention for resources can be handled in a
consistent manner.

*CPU* and *memory* are each a *resource type*.  A resource type has a base unit.  CPU is specified
in units of cores.  Memory is specified in units of bytes.

CPU and RAM are collectively refered to as *compute resources*, or just *resources*.  Compute
resources are measureable quantities which can be requested, allocated, and consumed.  They are
distinct from [API resources](http://releases.k8s.io/HEAD/docs/./working_with_resources.md).  API resources, such as pods and
[services](http://releases.k8s.io/HEAD/docs/./services.md) are objects that can be written to and retrieved from the Kubernetes API
server.

## Container and Pod Resource Limits

Each container of a Pod can optionally specify `spec.container[].resources.limits.cpu` and/or
`spec.container[].resources.limits.memory`.  The `spec.container[].resources.requests` field is not
currently used and need not be set.

Specifying resource limits is optional.  In some clusters, an unset value may be replaced with a
default value when a pod is created or updated.  The default value depends on how the cluster is
configured.

Although limits can only be specified on individual containers, it is convenient to talk about pod
resource limits.  A *pod resource limit* for a particular resource type is the sum of the resource
limits of that type for each container in the pod, with unset values treated as zero.

The following pod has two containers.  Each has a limit of 0.5 core of cpu and 128MiB
(2<sup>20</sup> bytes) of memory.  The pod can be said to have a limit of 1 core and 256MiB of
memory.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: frontend
spec:
  containers:
  - name: db
    image: mysql
    resources:
      limits:
        memory: "128Mi"
        cpu: "500m"
  - name: wp
    image: wordpress
    resources:
      limits:
        memory: "128Mi"
        cpu: "500m"
```

## How Pods with Resource Limits are Scheduled

When a pod is created, the kubernetes scheduler selects a node for the pod to
run on.  Each node has a maximum capacity for each of the resource types: the
amount of CPU and memory it can provide for pods.  The scheduler ensures that,
for each resource type (CPU and memory), the sum of the resource limits of the
containers scheduled to the node is less than the capacity of the node.  Note
that although actual memory or CPU resource usage on nodes is very low, the
scheduler will still refuse to place pods onto nodes if the capacity check
fails.  This protects against a resource shortage on a node when resource usage
later increases, such as due to a daily peak in request rate.

Note: Although the scheduler normally spreads pods out across nodes, there are currently some cases
where pods with no limits (unset values) might all land on the same node.

## How Pods with Resource Limits are Run

When kubelet starts a container of a pod, it passes the CPU and memory limits to the container
runner (Docker or rkt).

When using Docker:
- The `spec.container[].resources.limits.cpu` is multiplied by 1024, converted to an integer, and
  used as the value of the [`--cpu-shares`](
  https://docs.docker.com/reference/run/#runtime-constraints-on-resources) flag to the `docker run`
  command.
- The `spec.container[].resources.limits.memory` is converted to an integer, and used as the value
  of the [`--memory`](https://docs.docker.com/reference/run/#runtime-constraints-on-resources) flag
  to the `docker run` command.

**TODO: document behavior for rkt**

If a container exceeds its memory limit, it may be terminated.  If it is restartable, it will be
restarted by kubelet, as will any other type of runtime failure.  If it is killed for exceeding its
memory limit, you will see the reason `OOM Killed`, as in this example:
```
$ kubectl get pods/memhog
NAME      READY     REASON       RESTARTS   AGE
memhog    0/1       OOM Killed   0          1h
```
*OOM* stands for Out Of Memory.

A container may or may not be allowed to exceed its CPU limit for extended periods of time.
However, it will not be killed for excessive CPU usage.

## Monitoring Compute Resource Usage

The resource usage of a pod is reported as part of the Pod status.

If [optional monitoring](http://releases.k8s.io/HEAD/docs/../cluster/addons/monitoring/README.md) is configured for your cluster,
then pod resource usage can be retrieved from the monitoring system.

## Troubleshooting

If the scheduler cannot find any node where a pod can fit, then the pod will remain unscheduled
until a place can be found.    An event will be produced each time the scheduler fails to find a
place for the pod, like this:
```
$ kubectl describe pods/frontend | grep -A 3 Events
Events:
  FirstSeen				LastSeen			Count	From SubobjectPath	Reason			Message
  Tue, 30 Jun 2015 09:01:41 -0700	Tue, 30 Jun 2015 09:39:27 -0700	128	{scheduler }            failedScheduling	Error scheduling: For each of these fitness predicates, pod frontend failed on at least one node: PodFitsResources.
```

If a pod or pods are pending with this message, then there are several things to try:
- Add more nodes to the cluster.
- Terminate unneeded pods to make room for pending pods.
- Check that the pod is not larger than all the nodes.  For example, if all the nodes
have a capacity of `cpu: 1`, then a pod with a limit of `cpu: 1.1` will never be scheduled.

You can check node capacities with the `kubectl get nodes -o <format>` command.
Here are some example command lines that extract just the necessary information:
- `kubectl get nodes -o yaml | grep '\sname\|cpu\|memory'`
- `kubectl get nodes -o json | jq '.items[] | {name: .metadata.name, cap: .status.capacity}'`

The [resource quota](http://releases.k8s.io/HEAD/docs/./resource_quota_admin.md) feature can be configured
to limit the total amount of resources that can be consumed.  If used in conjunction
with namespaces, it can prevent one team from hogging all the resources.

## Planned Improvements

The current system only allows resource quantities to be specified on a container.
It is planned to improve accounting for resources which are shared by all containers in a pod,
such as [EmptyDir volumes](http://releases.k8s.io/HEAD/docs/./volumes.md#emptydir).

The current system only supports container limits for CPU and Memory.
It is planned to add new resource types, including a node disk space
resource, and a framework for adding custom [resource types](http://releases.k8s.io/HEAD/docs/./design/resources.md#resource-types).

The current system does not facilitate overcommitment of resources because resources reserved
with container limits are assured.  It is planned to support multiple levels of [Quality of
Service](https://github.com/GoogleCloudPlatform/kubernetes/issues/168).

Currently, one unit of CPU means different things on different cloud providers, and on different
machine types within the same cloud providers.  For example, on AWS, the capacity of a node
is reported in [ECUs](http://aws.amazon.com/ec2/faqs/), while in GCE it is reported in logical
cores.  We plan to revise the definition of the cpu resource to allow for more consistency
across providers and platforms.



[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/compute_resources.md?pixel)]()
