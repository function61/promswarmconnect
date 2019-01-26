package main

import (
	"fmt"
	"github.com/function61/gokit/assert"
	"testing"
)

func TestServiceToMetricsEndpointsNo(t *testing.T) {
	metricsEndpointMissing := map[string]string{
		"foo": "bar", // no METRICS_ENDPOINT defined => will not parse as endpoint
	}

	endpoints := serviceToMetricsEndpoints([]Service{serviceDef(metricsEndpointMissing, inst1, inst2)})
	assert.Assert(t, len(endpoints) == 0)
}

func TestServiceToMetricsEndpoints(t *testing.T) {
	envs := map[string]string{
		"METRICS_ENDPOINT": ":80/metrics",
	}

	endpoints := serviceToMetricsEndpoints([]Service{serviceDef(envs, inst1, inst2)})
	assert.Assert(t, len(endpoints) == 2)

	assertEndpoint(t, endpoints[0], "job<hellohttp> instance<task1> address<10.0.0.2:80> path</metrics>")
	assertEndpoint(t, endpoints[1], "job<hellohttp> instance<task2> address<10.0.0.3:80> path</metrics>")
}

func TestServiceToMetricsEndpointsDifferentPortAndPath(t *testing.T) {
	envs := map[string]string{
		"METRICS_ENDPOINT": ":443/foometrics",
	}

	endpoints := serviceToMetricsEndpoints([]Service{serviceDef(envs, inst1, inst2)})
	assert.Assert(t, len(endpoints) == 2)

	assertEndpoint(t, endpoints[0], "job<hellohttp> instance<task1> address<10.0.0.2:443> path</foometrics>")
	assertEndpoint(t, endpoints[1], "job<hellohttp> instance<task2> address<10.0.0.3:443> path</foometrics>")
}

func TestServiceToMetricsEndpointsOverrideInstance(t *testing.T) {
	envs := map[string]string{
		"METRICS_ENDPOINT":          "/metrics", // also testing missing port => default to 80
		"METRICS_OVERRIDE_INSTANCE": "n/a",
	}

	endpoints := serviceToMetricsEndpoints([]Service{serviceDef(envs, inst1, inst2)})
	assert.Assert(t, len(endpoints) == 2)

	assertEndpoint(t, endpoints[0], "job<hellohttp> instance<n/a> address<10.0.0.2:80> path</metrics>")
	assertEndpoint(t, endpoints[1], "job<hellohttp> instance<n/a> address<10.0.0.3:80> path</metrics>")
}

func TestServiceToMetricsEndpointsOverrideInstanceWithHostname(t *testing.T) {
	envs := map[string]string{
		"METRICS_ENDPOINT":          "/metrics",
		"METRICS_OVERRIDE_INSTANCE": "_HOSTNAME_", // can be used for host-level metrics
	}

	endpoints := serviceToMetricsEndpoints([]Service{serviceDef(envs, inst1)})
	assert.Assert(t, len(endpoints) == 1)

	assertEndpoint(t, endpoints[0], "job<hellohttp> instance<node1.example.com> address<10.0.0.2:80> path</metrics>")
}

func assertEndpoint(t *testing.T, endpoint MetricsEndpoint, expectedRepr string) {
	t.Helper()

	actualRepr := fmt.Sprintf(
		"job<%s> instance<%s> address<%s> path<%s>",
		endpoint.Job,
		endpoint.Instance,
		endpoint.Address,
		endpoint.MetricsPath)

	assert.EqualString(t, actualRepr, expectedRepr)
}

// test data
var inst1 = ServiceInstance{
	DockerTaskId: "task1",
	NodeID:       "node1",
	NodeHostname: "node1.example.com",
	IPv4:         "10.0.0.2",
}

var inst2 = ServiceInstance{
	DockerTaskId: "task2",
	NodeID:       "node1",
	NodeHostname: "node1.example.com",
	IPv4:         "10.0.0.3",
}

func serviceDef(envs map[string]string, instances ...ServiceInstance) Service {
	return Service{
		Name:      "hellohttp",
		Image:     "joonas/hellohttp:latest",
		ENVs:      envs,
		Instances: instances,
	}
}
