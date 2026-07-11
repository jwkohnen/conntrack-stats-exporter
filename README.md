# Prometheus Node Exporter

NOTE: THIS PROJECT IS DEPRECATED AND IN MAINTENANCE MODE!

Since quite a while now the Linux kernel does export conntrack stats via the procfs and
also Prometheus Mode Exporter exports them if available.

This Exporter will emit an log line at start up if the procfs makes the stats
available.

I will keep updating the container images to fix security bugs (or rather: to
make security scanners happy,) but you should probably migrate to the
prometheus node exporter.  If you miss a conntrack related metric in the node
exporter, chances are that the prometheus folks do export it already, but chose
a funny name, e.g. conntrack_max -> conntrack_entries_limit.

# Conntrack Statistics Exporter

Motivation for this exporter was to survey `insert_failed` statistics due to a
race condition in the Linux ipfilter conntrack kernel code. This is a subtle
bug that in some circumstances escalates in high workload scenarios in
Kubernetes clusters and causes drop of initial packets of NATted connections
(both UDP, TCP.) The `insert_failed` statistic correlates with dropped
connections due to this bug.

# Helm Chart

See [Prometheus Community Charts](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus-conntrack-stats-exporter).
Kudos to @monotek!

## Further information about the conntrack race bug and its effect on Kubernetes

* https://blog.quentin-machu.fr/2018/06/24/5-15s-dns-lookups-on-kubernetes/
* https://tech.xing.com/a-reason-for-unexplained-connection-timeouts-on-kubernetes-docker-abd041cf7e02
* https://www.weave.works/blog/racy-conntrack-and-dns-lookup-timeouts
