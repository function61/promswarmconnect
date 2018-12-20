package main

import (
	"context"
	"github.com/docker/docker/client"
	"net"
	"regexp"
	"strconv"
)

type ConfigContext struct {
	HostIp string
}

/*
	{
		"targets": [
			"10.0.0.23:80"
		],
		"labels": {
			"job": "html2pdf"
		}
	}
*/
type PromServiceTargetsList struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

type PromServiceTargetsFile []PromServiceTargetsList

var (
	portRe  = regexp.MustCompile(":[0-9]+")
	ipRegex = regexp.MustCompile("^[0-9\\.]+")
)

func parseMetricsEndpointSpec(env string) int {
	port_ := portRe.FindString(env)
	if port_ == "" {
		port_ = ":80"
	}

	port, err := strconv.Atoi(port_[1:]) // ":80" => 80
	if err != nil {
		panic(err)
	}

	return port
}

// TODO: parsing support is sucky
// TODO: does not yet support parsing the actual path
func parseMetricsEndpointEnv(envs []string) (bool, int, string) {
	for _, env := range envs {
		match, err := regexp.MatchString("^METRICS_ENDPOINT=.+", env)
		if err != nil {
			panic(err)
		}

		if match {
			port := parseMetricsEndpointSpec(env)
			return true, port, "/metrics"
		}
	}

	return false, 0, "" // hasMetricsEndpoint, metricsPort, metricsPath
}

// for some reason the ips contain a netmask like this "10.0.0.7/24"
func extractIpFromNetmask(mangledIp string) string {
	return ipRegex.FindString(mangledIp)
}

func resolveSelfSwarmIp(cli *client.Client) (string, error) {
	ctx := context.Background()
	info, errInfo := cli.Info(ctx)
	if errInfo != nil {
		return "", errInfo
	}

	selfNodeId := info.Swarm.NodeID

	node, raw, err := cli.NodeInspectWithRaw(ctx, selfNodeId)
	if err != nil {
		return "", err
	}
	defer raw.Close()

	// 192.168.56.61:2377 => 192.168.56.61
	ip, _, err := net.SplitHostPort(node.ManagerStatus.Addr)
	if err != nil {
		return "", err
	}

	return ip, nil
}
