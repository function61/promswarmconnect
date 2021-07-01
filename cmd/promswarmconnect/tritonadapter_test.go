package main

import (
	"testing"

	"github.com/function61/gokit/testing/assert"
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

	assert.EqualJson(t, serviceInstancesToTritonContainers([]Service{dummySvc2}), `{
  "containers": [
    {
      "server_uuid": "/metrics",
      "vm_alias": "10.0.0.2:80",
      "vm_brand": "http",
      "vm_image_uuid": "hellohttp",
      "vm_uuid": "task1"
    },
    {
      "server_uuid": "/metrics",
      "vm_alias": "10.0.0.3:80",
      "vm_brand": "http",
      "vm_image_uuid": "hellohttp",
      "vm_uuid": "task2"
    }
  ]
}`)
}

func TestHttps(t *testing.T) {
	result := serviceInstancesToTritonContainers([]Service{
		{
			Name:  "hellohttp",
			Image: "joonas/hellohttp:latest",
			ENVs: map[string]string{
				"METRICS_ENDPOINT": ":443/metrics",
			},
			Instances: []ServiceInstance{
				{
					DockerTaskId: "task1",
					NodeID:       "node1",
					NodeHostname: "node1.example.com",
					IPv4:         "10.0.0.2",
				},
			},
		},
	})

	assert.EqualJson(t, result, `{
  "containers": [
    {
      "server_uuid": "/metrics",
      "vm_alias": "10.0.0.2:443",
      "vm_brand": "https",
      "vm_image_uuid": "hellohttp",
      "vm_uuid": "task1"
    }
  ]
}`)
}
