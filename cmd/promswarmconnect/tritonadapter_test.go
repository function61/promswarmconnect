package main

import (
	"encoding/json"
	"github.com/function61/gokit/assert"
	"testing"
)

func TestServiceInstancesToTritonContainers(t *testing.T) {
	dummySvc1WithoutProperEnvVar := Service{
		Name:  "hellohttp",
		Image: "joonas/hellohttp:latest",
		ENVs: map[string]string{
			"foo": "bar",
		},
		Instances: []ServiceInstance{
			{
				DockerTaskId: "task1",
				NodeID:       "node1",
				NodeHostname: "node1.example.com",
				IPv4:         "10.0.0.2",
			},
		},
	}

	dummySvc2 := Service{
		Name:  "hellohttp",
		Image: "joonas/hellohttp:latest",
		ENVs: map[string]string{
			"METRICS_ENDPOINT": ":80/metrics",
		},
		Instances: []ServiceInstance{
			{
				DockerTaskId: "task1",
				NodeID:       "node1",
				NodeHostname: "node1.example.com",
				IPv4:         "10.0.0.2",
			},
			{
				DockerTaskId: "task2",
				NodeID:       "node1",
				NodeHostname: "node1.example.com",
				IPv4:         "10.0.0.3",
			},
		},
	}

	noProperEnvVarResult := serviceInstancesToTritonContainers([]Service{dummySvc1WithoutProperEnvVar})
	assert.Assert(t, len(noProperEnvVarResult.Containers) == 0)

	result := serviceInstancesToTritonContainers([]Service{dummySvc2})
	assert.Assert(t, len(result.Containers) == 2)

	asJson, err := json.MarshalIndent(&result, "", "  ")
	assert.Assert(t, err == nil)

	assert.EqualString(t, string(asJson), `{
  "containers": [
    {
      "server_uuid": "/metrics",
      "vm_alias": "10.0.0.2:80",
      "vm_brand": "",
      "vm_image_uuid": "hellohttp",
      "vm_uuid": "task1"
    },
    {
      "server_uuid": "/metrics",
      "vm_alias": "10.0.0.3:80",
      "vm_brand": "",
      "vm_image_uuid": "hellohttp",
      "vm_uuid": "task2"
    }
  ]
}`)
}
