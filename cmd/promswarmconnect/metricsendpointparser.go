package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type MetricsEndpoint struct {
	// https://prometheus.io/docs/concepts/jobs_instances/
	Job         string
	Instance    string
	Address     string // __address__
	MetricsPath string // __metrics_path__
	Scheme      string // __scheme__

	Service *Service
}

// parses Prometheus endpoints from Service info provided by a discovery backend

func serviceToMetricsEndpoints(services []Service) []MetricsEndpoint {
	metricsEndpoints := []MetricsEndpoint{}

	for _, service := range services {
		// looks up METRICS_ENDPOINT, METRICS_OVERRIDE_INSTANCE
		foundEndpoints := processSuffix(service, "")
		metricsEndpoints = append(metricsEndpoints, foundEndpoints...)

		i := 2
		for {
			// looks up METRICS_ENDPOINT2, METRICS_OVERRIDE_INSTANCE2 etc.
			foundEndpoints = processSuffix(service, fmt.Sprintf("%d", i))
			if len(foundEndpoints) == 0 {
				break
			}

			metricsEndpoints = append(metricsEndpoints, foundEndpoints...)
			i++
		}
	}

	return metricsEndpoints
}

func processSuffix(service Service, suff string) []MetricsEndpoint {
	// don't add all services, but only those whitelisted by this explicit setting
	endpointSpecifierRaw, endpointSpecifierExists := service.ENVs["METRICS_ENDPOINT"+suff]
	if !endpointSpecifierExists {
		return nil
	}

	spec, err := parseEndpointSpecifier(endpointSpecifierRaw)
	if err != nil {
		panic(err) // FIXME: handle error gracefully
	}
	metricsEndpointPort := "80"
	if spec.port != "" {
		metricsEndpointPort = spec.port
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
	// TODO: deprecate this over the now-smarter endpoint specifier
	overrideInstanceLabel := service.ENVs["METRICS_OVERRIDE_INSTANCE"+suff] // ok if not set
	if spec.instanceOverride != "" {
		overrideInstanceLabel = spec.instanceOverride
	}

	jobLabel := service.Name
	if spec.jobOverride != "" {
		jobLabel = spec.jobOverride
	}

	metricsEndpoints := []MetricsEndpoint{}

	scheme := func() string {
		if metricsEndpointPort == "443" { // FIXME: support non-default ports also..
			return "https"
		} else {
			return "http"
		}
	}()

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
			Job:         jobLabel,
			Instance:    instanceLabel,
			Address:     hostAndPort,
			MetricsPath: spec.path,
			Scheme:      scheme,

			Service: &service,
		})
	}

	return metricsEndpoints
}

type endpointSpecifier struct {
	port             string
	path             string
	instanceOverride string
	jobOverride      string
}

// ":443/metrics" => ("443", "/metrics")
// "/metrics" => ("", "/metrics")
var splitPortAndPathRe = regexp.MustCompile("^(:([0-9]+))?(.+)")

// parses values like:
//     "/metrics"
//     ":80/metrics,job=hellohttp,instance=fas5324df"
func parseEndpointSpecifier(hostPort string) (*endpointSpecifier, error) {
	portions := strings.Split(hostPort, ",")

	hostPortParse := splitPortAndPathRe.FindStringSubmatch(portions[0])
	if hostPortParse == nil {
		return nil, errors.New("unable to parse host:port")
	}

	spec := endpointSpecifier{
		port: hostPortParse[2],
		path: hostPortParse[3],
	}

	for _, portion := range portions[1:] {
		equalsPos := strings.Index(portion, "=")
		if equalsPos == -1 {
			return nil, errors.New("portion equals sign not found")
		}

		key := portion[0:equalsPos]
		value := portion[equalsPos+1:]

		switch key {
		case "job":
			spec.jobOverride = value
		case "instance":
			spec.instanceOverride = value
		default:
			return nil, fmt.Errorf("unknown key: %s", key)
		}

		if value == "" {
			return nil, fmt.Errorf("empty value for key: %s", key)
		}
	}

	return &spec, nil
}
