package main

func serviceInstancesToTritonContainers(services []Service) TritonDiscoveryResponse {
	containers := []TritonDiscoveryResponseContainer{}

	for _, service := range services {
		// don't add all services, but only those whitelisted by this explicit setting
		metricsEndpoint, metricsEndpointExists := service.ENVs["METRICS_ENDPOINT"]
		if !metricsEndpointExists {
			continue
		}

		metricsEndpointPort, metricsEndpointPath := splitPortAndPath(metricsEndpoint)
		if metricsEndpointPort == "" {
			metricsEndpointPort = "80"
		}

		/*	Prometheus timeseries have two required labels for each timeseries:

			- job. this corresponds with a service. example: "html2pdf"
			- instance. there usually are multiple instances of a service (job) running

			Usually instance is container specific, but there are exceptions. Here are the
			use cases for instance label in order of popularity:

			1) container scoped (default) => each container gets own timeseries
				- use container ID (or IP) as "instance" label
			2) host scoped => each host gets own timeseries
				- example: host-level metrics, but exporter runs in container
				- use hostname (or host id) as "instance" label
			3) static string => only one timeseries in whole cluster
				- example: cluster-wide metrics (node count for example)
				- use static string (e.g. "n/a") as "instance" label
		*/

		// this is used to implement cases 2) and 3). use "_HOSTNAME_" to replace with hostname
		// or any other string to have a static string
		overrideInstanceLabel := service.ENVs["METRICS_OVERRIDE_INSTANCE"] // ok if not set

		for _, instance := range service.Instances {
			hostAndPort := instance.IPv4 + ":" + metricsEndpointPort

			instanceLabel := instance.DockerTaskId
			if overrideInstanceLabel != "" {
				instanceLabel = overrideInstanceLabel

				if instanceLabel == "_HOSTNAME_" {
					instanceLabel = instance.NodeHostname
				}
			}

			// these pieces of info are assigned to pretty much random keys, knowing that
			// we must redirect them into other/correct fields anyway by hacking with
			// Prometheus relabeling ("VMAlias actually means __address_" etc) configuration.
			// without relabeling the Triton plugin code in Prometheus requires DNS suffixes etc.
			containers = append(containers, TritonDiscoveryResponseContainer{
				VMImageUUID: service.Name,        // relabeled: job
				VMUUID:      instanceLabel,       // relabeled: instance
				VMAlias:     hostAndPort,         // relabeled: __address__
				ServerUUID:  metricsEndpointPath, // relabeled: __metrics_path__
			})
		}
	}

	return TritonDiscoveryResponse{
		Containers: containers,
	}
}
