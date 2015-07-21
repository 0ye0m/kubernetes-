<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

![WARNING](http://kubernetes.io/img/warning.png)
![WARNING](http://kubernetes.io/img/warning.png)
![WARNING](http://kubernetes.io/img/warning.png)

<h1>PLEASE NOTE: This document applies to the HEAD of the source
tree only. If you are using a released version of Kubernetes, you almost
certainly want the docs that go with that version.</h1>

<strong>Documentation for specific releases can be found at
[releases.k8s.io](http://releases.k8s.io).</strong>

![WARNING](http://kubernetes.io/img/warning.png)
![WARNING](http://kubernetes.io/img/warning.png)
![WARNING](http://kubernetes.io/img/warning.png)

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->
# DNS Integration with Kubernetes

As of kubernetes 0.8, DNS is offered as a [cluster add-on](../../cluster/addons/README.md).
If enabled, a DNS Pod and Service will be scheduled on the cluster, and the kubelets will be
configured to tell individual containers to use the DNS Service's IP.

Every Service defined in the cluster (including the DNS server itself) will be
assigned a DNS name.  By default, a client Pod's DNS search list will
include the Pod's own namespace and the cluster's default domain.  This is best
illustrated by example:

Assume a Service named `foo` in the kubernetes namespace `bar`.  A Pod running
in namespace `bar` can look up this service by simply doing a DNS query for
`foo`.  A Pod running in namespace `quux` can look up this service by doing a
DNS query for `foo.bar`.

The cluster DNS server ([SkyDNS](https://github.com/skynetservices/skydns))
supports forward lookups (A records) and service lookups (SRV records).

## How it Works

The running DNS pod holds 3 containers - skydns, etcd (which skydns uses),
and a kubernetes-to-skydns bridge called kube2sky.  The kube2sky process
watches the kubernetes master for changes in Services, and then writes the
information to etcd, which skydns reads.  This etcd instance is not linked to
any other etcd clusters that might exist, including the kubernetes master.

## Issues

The skydns service is reachable directly from kubernetes nodes (outside
of any container) and DNS resolution works if the skydns service is targeted
explicitly. However, nodes are not configured to use the cluster DNS service or
to search the cluster's DNS domain by default.  This may be resolved at a later
time.

## For more information

See [the docs for the DNS cluster addon](../../cluster/addons/dns/README.md).


<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/docs/admin/dns.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
