package udocker

const (
	ListContainersEndpoint = "/v1.24/containers/json"
	TasksEndpoint          = "/v1.24/tasks?filters=%7B%22desired-state%22%3A%5B%22running%22%5D%7D"
	ServicesEndpoint       = "/v1.24/services"
	NodesEndpoint          = "/v1.24/nodes"
)

func ContainerInspectEndpoint(containerId string) string {
	return "/v1.24/containers/" + containerId + "/json"
}
