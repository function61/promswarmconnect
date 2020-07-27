package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/function61/gokit/envvar"
	"github.com/function61/gokit/ezhttp"
	"github.com/function61/gokit/udocker"
)

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
		ezhttp.RespondsJson(&dockerTasks, true),
	); err != nil {
		return nil, err
	}

	dockerServices := []udocker.Service{}
	if _, err := ezhttp.Get(
		ctx,
		dockerUrl+udocker.ServicesEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJson(&dockerServices, true),
	); err != nil {
		return nil, err
	}

	dockerNodes := []udocker.Node{}
	if _, err := ezhttp.Get(
		ctx,
		dockerUrl+udocker.NodesEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJson(&dockerNodes, true),
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

			var firstIp net.IP = nil
			attachment := networkAttachmentForNetworkName(task, networkName)
			if attachment != nil {
				// for some reason Docker insists on stuffing the CIDR after the IP
				var err error
				firstIp, _, err = net.ParseCIDR(attachment.Addresses[0])
				if err != nil {
					return nil, err
				}
			}

			if firstIp == nil {
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

			instances = append(instances, ServiceInstance{
				DockerTaskId: task.ID,
				NodeID:       node.ID,
				NodeHostname: node.Description.Hostname,
				IPv4:         firstIp.String(),
			})
		}

		envs := map[string]string{}

		for _, envSerialized := range dockerService.Spec.TaskTemplate.ContainerSpec.Env {
			envKey, envVal := envvar.Parse(envSerialized)
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
