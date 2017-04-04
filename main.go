package main

import (
	"github.com/docker/docker/client"
	"log"
	"sync"
)

// requires that this will be ran on a manager node
func main() {
	// connection errors are not actually handled here, but instead when we call our first method.
	cli, err := client.NewClient("unix:///var/run/docker.sock", "", nil, nil)
	if err != nil {
		panic(err)
	}

	hostIp, err := resolveSelfSwarmIp(cli)
	if err != nil {
		panic(err)
	}

	log.Printf("main: resolved host IP to %s", hostIp)

	conf := &ConfigContext{hostIp}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go collectHostStatsTask(wg)

	wg.Add(1)
	go syncPromTargetsTask(cli, conf, wg)

	wg.Add(1)
	go containerMetricsTask(cli, wg)

	wg.Wait()
}
