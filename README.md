[![Build Status](https://img.shields.io/travis/function61/promswarmconnect.svg?style=for-the-badge)](https://travis-ci.org/function61/promswarmconnect)
[![Download](https://img.shields.io/docker/pulls/fn61/promswarmconnect.svg?style=for-the-badge)](https://hub.docker.com/r/fn61/promswarmconnect/)

What?
-----

Syncs services/tasks from Docker Swarm to Prometheus by pretending to be a Triton service
discovery endpoint, which is a
[built-in service discovery module](https://github.com/prometheus/prometheus/tree/master/discovery/triton)
in Prometheus.

Features:

- Have your container metrics scraped fully automatically to Prometheus.
- We don't have to make ANY changes to Prometheus (or its container) to support Docker
  Swarm (except configuration changes).
- Supports overriding metrics endpoint (default `/metrics`) and port.
- Supports clustering, so containers are discovered from all nodes. Neither Prometheus
  nor promswarmconnect needs to run on the Swarm manager node.
    * promswarmconnect needs to run on Swarm manager if you use the `docker.sock` mount option
- Supports scoping Prometheus `job` label to a) container (default), b) host (think host-level
  metrics) or c) static string (think cluster-wide metrics).
  [Read more](https://github.com/function61/promswarmconnect/blob/ecc947d4aa6b29bb4595929d2bc23b1ec7bd5e9e/cmd/promswarmconnect/main.go#L173)

![](docs/architecture.png)

NOTE: the drawing is for option 2). This is even simpler if you use option 1) with socket mount.


How to deploy
-------------

Run the image from Docker Hub (see top of README) with the configuration mentioned below.
Both options mention "VERSION" version of the image. You'll find the latest version from
the Docker Hub. We don't currently publish "latest" tag so the versions are immutable.

You need to run promswarmconnect and Prometheus on the same network.

### Option 1: run on Swarm manager node with mounted `docker.sock`

This is the easiest option, but requires you to have a placement constraint to guarantee
that promswarmconnect always runs on the manager node - its Docker socket is the only API
with knowledge of the whole cluster state.

```
$ docker service create \
	--name promswarmconnect \
	--constraint node.role==manager \
	--mount type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock \
	--env "DOCKER_URL=unix:///var/run/docker.sock" \
	--env "NETWORK_NAME=yourNetwork" \
	--network yourNetwork \
	"fn61/promswarmconnect:VERSION"
```

NOTE: `unix:..` contains three forward slashes!


### Option 2: run on any node by having Docker's socket exposed over HTTPS

This may be useful to you if you have other needs that also require you to expose Docker's
port. For example I'm running [Portainer](https://www.portainer.io/) on my own computer
and that needs to dial to Docker's socket over TLS from the outside world.

Docker's socket needs to be exposed over HTTPS with a client cert authentication. We use
[dockersockproxy](https://github.com/function61/dockersockproxy) for this. You can do the
same with just pure Docker (expose the API over HTTPS) configuration, but I found it much
easier to not mess with default Docker settings, and to do this by just deploying a container.

Below configuration `DOCKER_CLIENTCERT` (and its key) refers to the client cert that is allowed to
connect to the Docker socket over HTTPS. They can be encoded to base64 like this:

- `$ cat cert.pem | base64 -w 0`
- `$ cat cert.key | base64 -w 0`

```
$ docker service create \
	--name promswarmconnect \
	--env "DOCKER_URL=https://dockersockproxy:4431" \
	--env "DOCKER_CLIENTCERT=..." \
	--env "DOCKER_CLIENTCERT_KEY=..." \
	--env "NETWORK_NAME=yourNetwork" \
	--network yourNetwork \
	"fn61/promswarmconnect:VERSION"
```

Obviously, you need to replace URL and port with your Docker socket's details.

### Verify that it's working

Before moving on to configure Prometheus, verify that promswarmconnect is working.

Grab an Alpine container (on the same network), and verify that you can `$ curl` the API:

```
$ docker run --rm -it --network yourNetwork alpine sh
$ apk add curl
$ curl -k https://promswarmconnect/v1/discover
{
  "containers": [
    {
      "server_uuid": "/metrics",
      "vm_alias": "10.0.1.7:8081",
      "vm_brand": "",
      "vm_image_uuid": "traefik_traefik",
      "vm_uuid": "rsvltiqm6nbcj72ibi7bess0w"
    },
    {
      "server_uuid": "/metrics",                 <-- __metrics_path__
      "vm_alias": "10.0.1.15:80",                <-- __address__
      "vm_brand": "",
      "vm_image_uuid": "hellohttp_hellohttp",    <-- job (Docker service name)
      "vm_uuid": "p44b6yr05ucmhpl0teiadq3jt"     <-- instance (Docker task ID)
    }
  ]
}
```

[More info here](https://github.com/function61/promswarmconnect/blob/ecc947d4aa6b29bb4595929d2bc23b1ec7bd5e9e/cmd/promswarmconnect/main.go#L207)
on why the JSON keys are so different W.R.T. Prometheus labels they'll be relabeled at
(see also our config example).


Configuring Prometheus
----------------------

Configure your Prometheus:
[example configuration that works for us](https://github.com/function61/prometheus-conf/blob/master/prometheus.yml).

The `endpoint` needs to be your service name in Docker that you use to run promswarmconnect.

Pro-tip: you could probably use
[our Prometheus image](https://github.com/function61/prometheus-conf) (check the Docker
Hub link) as-is, if not for production but at least to check out if this concept works for
you!


Considerations for running containers
-------------------------------------

promswarmconnect only picks up containers whose *service-level ENV vars* specify
`METRICS_ENDPOINT=/metrics`. To use non-80 port, specify `METRICS_ENDPOINT=:8080/metrics`.
The metrics path is also configurable, obviously.

For a complete demo with dummy application, deploy:

- promswarmconnect (instructions were at this document)
- our prometheus image (instructions were at above pro-tip) and
- [hellohttp](https://github.com/joonas-fi/hellohttp) (it has built-in Prometheus metrics)


How to build & develop
----------------------

[How to build & develop](https://github.com/function61/turbobob/blob/master/docs/external-how-to-build-and-dev.md)
(with Turbo Bob, our build tool). It's easy and simple!


Alternatives & links
--------------------

- https://github.com/ContainerSolutions/prometheus-swarm-discovery
- https://github.com/prometheus/prometheus/issues/1766
- https://github.com/jmendiara/prometheus-swarm-discovery
