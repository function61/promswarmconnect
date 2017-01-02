What?
-----

tl;dr: use `-e METRICS_ENDPOINT=/metrics` (or `METRICS_ENDPOINT=:8080/metrics` if using port != 80) for your Docker Swarm service,
and [Prometheus](https://prometheus.io/) will automatically discover your service and scrape metrics from it.

Integrates Docker (Swarm) with Prometheus via the file config option
(Prometheus currently [doesn't have support for Swarm mode](https://github.com/prometheus/prometheus/issues/1766)).

This image contains the whole bundle, Prometheus + docker-prometheus-bridge + configuration for truly automated service discovery.

```
+--------------------------+           +----------------------+
|                          | writes    |                      |
| docker-prometheus-bridge +-----------> Configuration file   |
|                          |           | (Scrapeable targets) |
+-------^------------------+           |                      |
        |                              +-----------+----------+
        | queries                                  |
        |                                          |
        |                                          | reads
+-------+------+                                   |
|              |                              +----v-------+
| Docker API   |                              |            |
| (Swarm mode) |                              | Prometheus |
|              |                              |            |
+--------------+                              +------------+
```

Only advertises services that have environment variable defined: `METRICS_ENDPOINT=/metrics` (whitelisting).

Uses service name as job name in Prometheus.

NOTE: current limitation is that you have to deploy Prometheus on the same node as a Swarm manager!
(use constraint with `$ docker service create` if/when you have a multi-node cluster)

DISCLAIMER: This is the first Go program I wrote, so it probably violates many Go idioms I am yet to learn.


Howto run
---------

Basically, you need to run this as a Swarm service (for Prometheus to have access to services running in Swarm network):

```
$ docker service create --network YOUR_NETWORK --name prometheus -p 9090:9090 \
	--mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
	--mount type=volume,src=prometheus-dev,dst=/prometheus \
	fn61/prometheus-docker-swarm:latest
```


Example config file to be fed to Prometheus
-------------------------------------------

This is the file format that docker-prometheus-bridge generated for Prometheus:

```
[
	{
		"targets": [
			"10.0.0.23:80"
		],
		"labels": {
			"job": "html2pdf"
		}
	}
]
```


Alternatives
------------

- https://github.com/bvis/docker-prometheus-swarm - doesn't export metrics from containers per se,
  but container metadata with cadvisor AND node metadata with node-exporter)
