package main

type DockerService struct {
	ID   string            `json:"ID"`
	Spec DockerServiceSpec `json:"Spec"`
}

type DockerServiceSpec struct {
	Name         string                        `json:"Name"`
	TaskTemplate DockerServiceSpecTaskTemplate `json:"TaskTemplate"`
}

type DockerServiceSpecTaskTemplate struct {
	ContainerSpec DockerServiceSpecTaskTemplateContainerSpec `json:"ContainerSpec"`
}

type DockerServiceSpecTaskTemplateContainerSpec struct {
	Image string   `json:"Image"`
	Env   []string `json:"Env"`
}

type DockerTask struct {
	ID                  string                        `json:"ID"`
	ServiceID           string                        `json:"ServiceID"`
	NodeID              string                        `json:"NodeID"`
	NetworksAttachments []DockerTaskNetworkAttachment `json:"NetworksAttachments"`
}

type DockerTaskNetworkAttachment struct {
	Network   DockerTaskNetworkAttachmentNetwork `json:"Network"`
	Addresses []string                           `json:"Addresses"`
}

type DockerTaskNetworkAttachmentNetwork struct {
	ID   string                                 `json:"ID"`
	Spec DockerTaskNetworkAttachmentNetworkSpec `json:"Spec"`
}

type DockerTaskNetworkAttachmentNetworkSpec struct {
	Name string `json:"Name"`
}

type DockerNode struct {
	ID          string                `json:"ID"`
	Description DockerNodeDescription `json:"Description"`
}

type DockerNodeDescription struct {
	Hostname string `json:"Hostname"`
}
