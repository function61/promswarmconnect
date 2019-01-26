package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/function61/gokit/dynversion"
	"github.com/function61/gokit/envvar"
	"github.com/function61/gokit/ezhttp"
	"github.com/function61/gokit/logex"
	"github.com/function61/gokit/ossignal"
	"github.com/function61/gokit/stopper"
	"github.com/function61/promswarmconnect/pkg/udocker"
	"net"
	"net/http"
	"regexp"
)

type Service struct {
	Name      string
	Image     string
	ENVs      map[string]string
	Instances []ServiceInstance
}

type ServiceInstance struct {
	DockerTaskId string
	NodeID       string
	NodeHostname string
	IPv4         string
}

func listDockerServiceInstances(dockerUrl string, networkName string, dockerClient *http.Client) ([]Service, error) {
	// all the requests have to finish within this timeout
	ctx, cancel := context.WithTimeout(context.TODO(), ezhttp.DefaultTimeout10s)
	defer cancel()

	dockerTasks := []udocker.Task{}
	if _, err := ezhttp.Send(
		ctx,
		http.MethodGet,
		dockerUrl+udocker.TasksEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJson(&dockerTasks, true),
	); err != nil {
		return nil, err
	}

	dockerServices := []udocker.Service{}
	if _, err := ezhttp.Send(
		ctx,
		http.MethodGet,
		dockerUrl+udocker.ServicesEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJson(&dockerServices, true),
	); err != nil {
		return nil, err
	}

	dockerNodes := []udocker.Node{}
	if _, err := ezhttp.Send(
		ctx,
		http.MethodGet,
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

func registerTritonDiscoveryApi() error {
	networkName, err := envvar.Get("NETWORK_NAME")
	if err != nil {
		return err
	}

	dockerUrl, err := envvar.Get("DOCKER_URL")
	if err != nil {
		return err
	}

	dockerClient, dockerUrlTransformed, err := udocker.Client(dockerUrl)
	if err != nil {
		return err
	}

	// for unix sockets we need to fake "http://localhost"
	dockerUrl = dockerUrlTransformed

	// adapts Docker Swarm services to Prometheus by pretending to be Triton discovery service.
	// requires also some hacking via Prometheus config, because we're passing data in fields
	// in different format than Prometheus expects
	http.HandleFunc("/v1/discover", func(w http.ResponseWriter, r *http.Request) {
		services, err := listDockerServiceInstances(dockerUrl, networkName, dockerClient)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tritonResponse := serviceInstancesToTritonContainers(services)

		jsonEncoder := json.NewEncoder(w)
		jsonEncoder.SetIndent("", "  ")
		if err := jsonEncoder.Encode(&tritonResponse); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return nil
}

func mainInternal(logl *logex.Leveled, stop *stopper.Stopper) error {
	if err := registerTritonDiscoveryApi(); err != nil {
		return err
	}

	// dummy cert valid until 2028-12-18T07:57:25Z
	serverCert, err := tls.X509KeyPair([]byte(dummyCert), []byte(dummyCertKey))
	if err != nil {
		return err
	}

	// we need TLS even though calling Prometheus specifies InsecureSkipVerify, because
	// the code in Prometheus is hardcoded to use https. well, I guess encryption without
	// authentication is still better than no encryption at all.
	srv := http.Server{
		Addr: ":443",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
		},
	}

	go func() {
		defer stop.Done()
		defer logl.Info.Printf("stopped")

		<-stop.Signal

		if err := srv.Shutdown(nil); err != nil {
			logl.Error.Fatalf("Shutdown() failed: %v", err)
		}
	}()

	logl.Info.Printf("Started v%s", dynversion.Version)

	if err := srv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func main() {
	rootLogger := logex.StandardLogger()

	workers := stopper.NewManager()

	go func(logl *logex.Leveled) {
		logl.Info.Printf("Got %s; stopping", <-ossignal.InterruptOrTerminate())

		workers.StopAllWorkersAndWait()
	}(logex.Levels(logex.Prefix("entrypoint", rootLogger)))

	mainlogl := logex.Levels(logex.Prefix("mainInternal", rootLogger))

	if err := mainInternal(mainlogl, workers.Stopper()); err != nil {
		mainlogl.Error.Fatal(err)
	}
}

func networkAttachmentForNetworkName(task udocker.Task, networkName string) *udocker.TaskNetworkAttachment {
	for _, attachment := range task.NetworksAttachments {
		if attachment.Network.Spec.Name == networkName {
			return &attachment
		}
	}

	return nil
}

// ":443/metrics" => ("443", "/metrics")
// "/metrics" => ("", "/metrics")
var splitPortAndPathRe = regexp.MustCompile("^(:([0-9]+))?(.+)")

func splitPortAndPath(hostPort string) (string, string) {
	matches := splitPortAndPathRe.FindStringSubmatch(hostPort)

	return matches[2], matches[3]
}

func nodeById(id string, nodes []udocker.Node) *udocker.Node {
	for _, node := range nodes {
		if node.ID == id {
			return &node
		}
	}

	return nil
}
