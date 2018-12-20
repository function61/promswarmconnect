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
