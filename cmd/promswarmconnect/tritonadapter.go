package main

// cannot reference from Prometheus sources because this was inlined
type TritonDiscoveryResponseContainer struct {
	ServerUUID  string `json:"server_uuid"`
	VMAlias     string `json:"vm_alias"`
	VMBrand     string `json:"vm_brand"`
	VMImageUUID string `json:"vm_image_uuid"`
	VMUUID      string `json:"vm_uuid"`
}

type TritonDiscoveryResponse struct {
	Containers []TritonDiscoveryResponseContainer `json:"containers"`
}

type MetricsEndpoint struct {
	// https://prometheus.io/docs/concepts/jobs_instances/
	Job         string
	Instance    string
	Address     string // __address__
	MetricsPath string // __metrics_path__

	Service *Service
}

func metricsEndpointToTritonResponse(endpoints []MetricsEndpoint) TritonDiscoveryResponse {
	containers := []TritonDiscoveryResponseContainer{}

	for _, endpoint := range endpoints {
		// these pieces of info are assigned to pretty much random keys, knowing that
		// we must redirect them into other/correct fields anyway by hacking with
		// Prometheus relabeling ("VMAlias actually means __address_" etc) configuration.
		// without relabeling the Triton plugin code in Prometheus requires DNS suffixes etc.
		containers = append(containers, TritonDiscoveryResponseContainer{
			VMImageUUID: endpoint.Job,
			VMUUID:      endpoint.Instance,
			VMAlias:     endpoint.Address,
			ServerUUID:  endpoint.MetricsPath,
		})
	}

	return TritonDiscoveryResponse{
		Containers: containers,
	}
}

func serviceInstancesToTritonContainers(services []Service) TritonDiscoveryResponse {
	return metricsEndpointToTritonResponse(serviceToMetricsEndpoints(services))
}
