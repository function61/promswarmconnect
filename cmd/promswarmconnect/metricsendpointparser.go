package main

type MetricsEndpoint struct {
	// https://prometheus.io/docs/concepts/jobs_instances/
	Job         string
	Instance    string
	Address     string // __address__
	MetricsPath string // __metrics_path__

	Service *Service
}

// parses Prometheus endpoints from Service info provided by a discovery backend

func serviceToMetricsEndpoints(services []Service) []MetricsEndpoint {
	metricsEndpoints := []MetricsEndpoint{}

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

			metricsEndpoints = append(metricsEndpoints, MetricsEndpoint{
				Job:         service.Name,
				Instance:    instanceLabel,
				Address:     hostAndPort,
				MetricsPath: metricsEndpointPath,

				Service: &service,
			})
		}
	}

	return metricsEndpoints
}
