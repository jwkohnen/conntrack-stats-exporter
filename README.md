# Conntrack Statistics Exporter

The well known prometheus node exporter exports conntrack metrics off the /proc
pseudo file system. The conntrack module developers consider that surface as
deprecated and provide a CLI tool `conntrack` that shows some interesting
metrics.

Motivation for this exporter was to survey `insert_failed` statistics due to a
race condition in the Linux ipfilter conntrack kernel code. This is a subtle
bug that in some circumstances escalates in high workload scenarios in
Kubernetes clusters and causes drop of initial packets of NATted connections
(both UDP, TCP.) The `insert_failed` statistic correlates with dropped
connections due to this bug.

## Further information about the conntrack race bug and its effect on Kubernetes

* https://blog.quentin-machu.fr/2018/06/24/5-15s-dns-lookups-on-kubernetes/
* https://tech.xing.com/a-reason-for-unexplained-connection-timeouts-on-kubernetes-docker-abd041cf7e02
* https://www.weave.works/blog/racy-conntrack-and-dns-lookup-timeouts
