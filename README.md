What?
-----

TL;DR: use `-e METRICS_ENDPOINT=/metrics` (or `METRICS_ENDPOINT=:8080/metrics`
if using port != 80) for your Docker Swarm service, and
[Prometheus](https://prometheus.io/) will automatically discover your service
and scrape metrics from it.

Integrates Docker (Swarm) with Prometheus via the file config option
(Prometheus currently [doesn't have support for Swarm mode](https://github.com/prometheus/prometheus/issues/1766)).

This image contains the whole bundle, Prometheus + docker-prometheus-bridge +
configuration for truly automated service discovery.

Because we're already integrating with Docker, we also export
[containers' metrics](containermetrics.go) and while we're at it,
[host's metrics](hoststats.go) (see `/hostproc` mount).

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


Howto run
---------

Basically, you need to run this as a Swarm service (for Prometheus to have access to services running in Swarm network):

```
$ docker service create --network YOUR_NETWORK --name prometheus -p 9090:9090 \
	-e METRICS_ENDPOINT=:8081/metrics \
	--mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
	--mount type=bind,src=/proc,dst=/hostproc \
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


Building
--------

```
$ docker run --rm -it -v "$(pwd):/app" golang:1.8.0 bash
$ cd /app
$ go get -d ./...
$ make

# Ctrl + d
```

You now have `./app` statically compiled.

Running `$ docker build` would bake that into a Docker image.


Alternatives & other reading
----------------------------

- https://github.com/bvis/docker-prometheus-swarm - doesn't export metrics from containers per se,
  but container metadata with cadvisor AND node metadata with node-exporter)
- https://github.com/docker/docker/issues/27307 roadmap for Docker-internal Prometheus integration
- https://github.com/prometheus/prometheus/issues/1766 Docker Swarm integration for Prometheus
