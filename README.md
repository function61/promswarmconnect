What?
-----

Integrates Docker (Swarm) with Prometheus via the file config option
(Prometheus currently [doesn't have support for Swarm mode](https://github.com/prometheus/prometheus/issues/1766)).

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
			"10.0.0.23"
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
