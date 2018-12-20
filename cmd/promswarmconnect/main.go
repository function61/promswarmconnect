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
	"net"
	"net/http"
	"regexp"
)

const (
	tasksEndpoint    = "/v1.24/tasks?filters=%7B%22desired-state%22%3A%5B%22running%22%5D%7D"
	servicesEndpoint = "/v1.24/services"
	nodesEndpoint    = "/v1.24/nodes"
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

func createDockerClient() (*http.Client, error) {
	clientCertificate, err := loadClientCertificateFromEnv()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*clientCertificate},
		InsecureSkipVerify: true,
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, nil
}

func listDockerServiceInstances(dockerUrl string, networkName string, dockerClient *http.Client) ([]Service, error) {
	// both requests have to finish within this timeout
	ctx, cancel := context.WithTimeout(context.TODO(), ezhttp.DefaultTimeout10s)
	defer cancel()

	dockerTasks := []DockerTask{}
	if _, err := ezhttp.Send(
		ctx,
		http.MethodGet,
		dockerUrl+tasksEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJson(&dockerTasks, true),
	); err != nil {
		return nil, err
	}

	dockerServices := []DockerService{}
	if _, err := ezhttp.Send(
		ctx,
		http.MethodGet,
		dockerUrl+servicesEndpoint,
		ezhttp.Client(dockerClient),
		ezhttp.RespondsJson(&dockerServices, true),
	); err != nil {
		return nil, err
	}

	dockerNodes := []DockerNode{}
	if _, err := ezhttp.Send(
		ctx,
		http.MethodGet,
		dockerUrl+nodesEndpoint,
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
			envKey, envVal := parseEnvString(envSerialized)
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

func registerTritonDiscoveryApi() error {
	networkName, err := envvar.Get("NETWORK_NAME")
	if err != nil {
		return err
	}

	dockerUrl, err := envvar.Get("DOCKER_URL")
	if err != nil {
		return err
	}

	dockerClient, err := createDockerClient()
	if err != nil {
		return err
	}

	// adapts Docker Swarm services to Prometheus by pretending to be Triton discovery service.
	// requires also some hacking via Prometheus config, because we're passing data in fields
	// in different format than Prometheus expects
	http.HandleFunc("/v1/discover", func(w http.ResponseWriter, r *http.Request) {
		containers := []TritonDiscoveryResponseContainer{}

		services, err := listDockerServiceInstances(dockerUrl, networkName, dockerClient)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

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

		if err := json.NewEncoder(w).Encode(&TritonDiscoveryResponse{
			Containers: containers,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return nil
}

func mainInternal(logl *logex.Leveled, stop *stopper.Stopper) error {
	if err := registerTritonDiscoveryApi(); err != nil {
		return err
	}

	tlsConfig, err := tlsSelfSignedConfig()
	if err != nil {
		return err
	}

	// we need TLS even though calling Prometheus specifies InsecureSkipVerify, because
	// the code in Prometheus is hardcoded to use https. well, I guess encryption without
	// authentication is still better than no encryption at all.
	srv := http.Server{
		Addr:      ":443",
		TLSConfig: tlsConfig,
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

	err = srv.ListenAndServeTLS("", "")
	if err != http.ErrServerClosed { // not graceful close?
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

func networkAttachmentForNetworkName(task DockerTask, networkName string) *DockerTaskNetworkAttachment {
	for _, attachment := range task.NetworksAttachments {
		if attachment.Network.Spec.Name == networkName {
			return &attachment
		}
	}

	return nil
}

func loadClientCertificateFromEnv() (*tls.Certificate, error) {
	clientCert, err := envvar.GetFromBase64Encoded("CLIENTCERT")
	if err != nil {
		return nil, err
	}

	clientCertKey, err := envvar.GetFromBase64Encoded("CLIENTCERT_KEY")
	if err != nil {
		return nil, err
	}

	clientKeypair, err := tls.X509KeyPair(clientCert, clientCertKey)
	if err != nil {
		return nil, err
	}

	return &clientKeypair, nil
}

var envParseRe = regexp.MustCompile("^([^=]+)=(.*)")

func parseEnvString(serialized string) (string, string) {
	matches := envParseRe.FindStringSubmatch(serialized)
	if matches == nil {
		return "", ""
	}

	return matches[1], matches[2]
}

// ":443/metrics" => ("443", "/metrics")
// "/metrics" => ("", "/metrics")
var splitPortAndPathRe = regexp.MustCompile("^(:([0-9]+))?(.+)")

func splitPortAndPath(hostPort string) (string, string) {
	matches := splitPortAndPathRe.FindStringSubmatch(hostPort)

	return matches[2], matches[3]
}

func nodeById(id string, nodes []DockerNode) *DockerNode {
	for _, node := range nodes {
		if node.ID == id {
			return &node
		}
	}

	return nil
}
