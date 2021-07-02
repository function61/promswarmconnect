package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/function61/gokit/app/udocker"
	"github.com/function61/gokit/net/http/ezhttp"
	"github.com/function61/gokit/os/osutil"
)

func listDockerServiceAndContainerInstances(
	ctx context.Context,
	dockerUrl string,
	networkName string,
	dockerClient *http.Client,
) ([]Service, error) {
	services, err := listDockerServiceInstances(ctx, dockerUrl, networkName, dockerClient)
	if err != nil {
		return nil, err
	}

	containersAsServices, err := listDockerContainerInstances(ctx, dockerUrl, networkName, dockerClient)
	if err != nil {
		return nil, err
	}

	return append(services, containersAsServices...), nil
}

func listDockerServiceInstances(
	ctx context.Context,
	dockerUrl string,
	networkName string,
	dockerClient *http.Client,
) ([]Service, error) {
	// all the requests have to finish within this timeout
	ctx, cancel := context.WithTimeout(ctx, ezhttp.DefaultTimeout10s)
	defer cancel()

	dockerTasks := []udocker.Task{}
	if _, err := ezhttp.Get(
		ctx,
		dockerUrl+udocker.TasksEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJsonAllowUnknownFields(&dockerTasks),
	); err != nil {
		return nil, err
	}

	dockerServices := []udocker.Service{}
	if _, err := ezhttp.Get(
		ctx,
		dockerUrl+udocker.ServicesEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJsonAllowUnknownFields(&dockerServices),
	); err != nil {
		return nil, err
	}

	dockerNodes := []udocker.Node{}
	if _, err := ezhttp.Get(
		ctx,
		dockerUrl+udocker.NodesEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJsonAllowUnknownFields(&dockerNodes),
	); err != nil {
		return nil, err
	}

	services := []Service{}

	for _, dockerService := range dockerServices {
		instances := []ServiceInstance{}

		for _, task := range dockerTasks {
			if task.ServiceID != dockerService.ID {
				continue
			}

			// task is not allocated to run on an explicit node yet, skip it since
			// our context is discovering running containers.
			if task.NodeID == "" {
				continue
			}

			node := nodeById(task.NodeID, dockerNodes)
			if node == nil {
				return nil, fmt.Errorf("node %s not found for task %s", task.NodeID, task.ID)
			}

			ip, err := func() (string, error) {
				if attachment := networkAttachmentForNetworkName(task, networkName); attachment != nil && len(attachment.Addresses) > 0 {
					// for some reason Docker insists on stuffing the CIDR after the IP
					firstIp, _, err := net.ParseCIDR(attachment.Addresses[0])
					if err != nil {
						return "", err
					}

					return firstIp.String(), nil
				}

				// fallback for host networking
				if hostAttachment := networkAttachmentForNetworkName(task, "host"); hostAttachment != nil && node.Status.Addr != "" {
					return node.Status.Addr, nil
				}

				return "", nil
			}()
			if err != nil {
				return nil, err
			}

			if ip == "" { // failed to find address for the task
				continue
			}

			instances = append(instances, ServiceInstance{
				DockerTaskId: task.ID,
				NodeID:       node.ID,
				NodeHostname: node.Description.Hostname,
				IPv4:         ip,
			})
		}

		envs := map[string]string{}

		for _, envSerialized := range dockerService.Spec.TaskTemplate.ContainerSpec.Env {
			envKey, envVal := osutil.ParseEnv(envSerialized)
			if envKey != "" {
				envs[envKey] = envVal
			}
		}

		services = append(services, Service{
			Name:      dockerService.Spec.Name,
			Image:     dockerService.Spec.TaskTemplate.ContainerSpec.Image,
			ENVs:      envs,
			Instances: instances,
		})
	}

	return services, nil
}

func listDockerContainerInstances(
	ctx context.Context,
	dockerUrl string,
	networkName string,
	dockerClient *http.Client,
) ([]Service, error) {
	services := []Service{}

	containers := []udocker.ContainerListItem{}
	if _, err := ezhttp.Get(
		ctx,
		dockerUrl+udocker.ListContainersEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJsonAllowUnknownFields(&containers),
	); err != nil {
		return nil, err
	}

	for _, container := range containers {
		if len(container.Names) == 0 {
			continue
		}

		// these are already handled by more specific handler
		if _, isSwarmService := container.Labels[udocker.SwarmServiceNameLabelKey]; isSwarmService {
			continue
		}

		ipAddress := ""
		if settings, found := container.NetworkSettings.Networks[networkName]; found {
			ipAddress = settings.IPAddress // prefer IP from the asked networkName
		}

		if settings, found := container.NetworkSettings.Networks["bridge"]; ipAddress == "" && found {
			ipAddress = settings.IPAddress // fall back to bridge IP if not found
		}

		if ipAddress == "" {
			continue
		}

		serviceName := container.Names[0]
		if composeServiceName, has := container.Labels["com.docker.compose.service"]; has {
			serviceName = composeServiceName
		}

		// stupid Docker doesn't return ENV vars with ListContainers call, so let's lie that
		// labels are ENV vars and inch closer to our goal of being able to specify metrics
		// endpoint as a label (so, now labels only work for docker-compose or manually
		// launched containers)
		labelsAsEnvs := map[string]string{}
		for key, value := range container.Labels {
			labelsAsEnvs[key] = value
		}

		services = append(services, Service{
			Name:  serviceName,
			Image: container.Image,
			ENVs:  labelsAsEnvs,
			Instances: []ServiceInstance{
				{
					DockerTaskId: container.Id[0:12], // Docker ps uses 12 hexits
					NodeID:       "dummy",
					NodeHostname: "dummy",
					IPv4:         ipAddress,
				},
			},
		})
	}

	return services, nil
}

func networkAttachmentForNetworkName(task udocker.Task, networkName string) *udocker.TaskNetworkAttachment {
	for _, attachment := range task.NetworksAttachments {
		if attachment.Network.Spec.Name == networkName {
			return &attachment
		}
	}

	return nil
}

func nodeById(id string, nodes []udocker.Node) *udocker.Node {
	for _, node := range nodes {
		if node.ID == id {
			return &node
		}
	}

	return nil
}
