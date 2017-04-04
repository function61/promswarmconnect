package main

import (
	"github.com/c9s/goprocinfo/linux"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"sync"
	"time"
)

type Metrics struct {
	cpu_user       prometheus.Gauge
	cpu_sys        prometheus.Gauge
	mem_free       prometheus.Gauge
	mem_available  prometheus.Gauge
	vm_page_in     prometheus.Gauge
	vm_page_out    prometheus.Gauge
	disk_free      prometheus.Gauge
	disk_used      prometheus.Gauge
	net_tx         prometheus.Gauge
	net_rx         prometheus.Gauge
	disk_io_reads  *prometheus.GaugeVec
	disk_io_writes *prometheus.GaugeVec
	disk_io_wait   *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	m := &Metrics{}

	m.cpu_user = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_user",
		Help: "cpu_user",
	})
	prometheus.MustRegister(m.cpu_user)

	m.cpu_sys = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_sys",
		Help: "cpu_sys",
	})
	prometheus.MustRegister(m.cpu_sys)

	m.mem_free = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mem_free",
		Help: "mem_free",
	})
	prometheus.MustRegister(m.mem_free)

	m.mem_available = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mem_available",
		Help: "mem_available",
	})
	prometheus.MustRegister(m.mem_available)

	m.vm_page_in = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vm_page_in",
		Help: "vm_page_in",
	})
	prometheus.MustRegister(m.vm_page_in)

	m.vm_page_out = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vm_page_out",
		Help: "vm_page_out",
	})
	prometheus.MustRegister(m.vm_page_out)

	m.disk_free = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_free",
		Help: "disk_free",
	})
	prometheus.MustRegister(m.disk_free)

	m.disk_used = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_used",
		Help: "disk_used",
	})
	prometheus.MustRegister(m.disk_used)

	m.net_tx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "net_tx",
		Help: "net_tx",
	})
	prometheus.MustRegister(m.net_tx)

	m.net_rx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "net_rx",
		Help: "net_rx",
	})
	prometheus.MustRegister(m.net_rx)

	m.disk_io_reads = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "disk_io_reads",
		Help: "disk_io_reads",
	}, []string{"dev"})
	prometheus.MustRegister(m.disk_io_reads)

	m.disk_io_writes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "disk_io_writes",
		Help: "disk_io_writes",
	}, []string{"dev"})
	prometheus.MustRegister(m.disk_io_writes)

	m.disk_io_wait = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "disk_io_wait",
		Help: "disk_io_wait",
	}, []string{"dev"})
	prometheus.MustRegister(m.disk_io_wait)

	return m
}

func pollCpu(wg *sync.WaitGroup, m *Metrics) {
	defer wg.Done()

	stat, err := linux.ReadStat("/hostproc/stat")
	if err != nil {
		log.Fatalf("err:", err)
	}

	m.cpu_user.Set(float64(stat.CPUStatAll.User))
	m.cpu_sys.Set(float64(stat.CPUStatAll.System))
}

func pollRam(wg *sync.WaitGroup, m *Metrics) {
	defer wg.Done()

	stat, err := linux.ReadMemInfo("/hostproc/meminfo")
	if err != nil {
		log.Fatalf("err:", err)
	}

	m.mem_free.Set(float64(stat.MemFree))
	m.mem_available.Set(float64(stat.MemAvailable))
}

func pollNet(wg *sync.WaitGroup, m *Metrics) {
	defer wg.Done()

	rows, err := linux.ReadNetworkStat("/hostproc/net/dev")

	if err != nil {
		log.Fatalf("err:", err)
	}

	aggregatedRx := 0.0
	aggregatedTx := 0.0

	for _, row := range rows {
		// TODO: interfaces not always named like this,
		// but we don't want to see hundreds of "veth"
		if row.Iface != "eth0" && row.Iface != "eth1" {
			continue
		}

		aggregatedRx += float64(row.RxBytes)
		aggregatedTx += float64(row.TxBytes)
	}

	m.net_rx.Set(aggregatedRx)
	m.net_tx.Set(aggregatedTx)
}

func pollDisk(wg *sync.WaitGroup, m *Metrics) {
	defer wg.Done()

	stat, err := linux.ReadDisk("/")

	if err != nil {
		log.Fatalf("err:", err)
	}

	m.disk_free.Set(float64(stat.Free))
	m.disk_used.Set(float64(stat.Used))
}

func pollIOPS(wg *sync.WaitGroup, m *Metrics) {
	defer wg.Done()

	rows, err := linux.ReadDiskStats("/hostproc/diskstats")

	if err != nil {
		log.Fatalf("err:", err)
	}

	for _, stat := range rows {
		// skip pseudo-disks like "ram", "dm" and "loop"
		// also, skip partitions like "sda1", "sda2" etc.
		if stat.Name != "sda" && stat.Name != "sdb" && stat.Name != "vda" {
			continue
		}

		m.disk_io_reads.With(prometheus.Labels{
			"dev": stat.Name,
		}).Set(float64(stat.ReadIOs))

		m.disk_io_writes.With(prometheus.Labels{
			"dev": stat.Name,
		}).Set(float64(stat.WriteIOs))

		m.disk_io_wait.With(prometheus.Labels{
			"dev": stat.Name,
		}).Set(float64(stat.TimeInQueue))
	}
}

func vmstat(wg *sync.WaitGroup, m *Metrics) {
	defer wg.Done()

	stat, err := linux.ReadVMStat("/hostproc/vmstat")

	if err != nil {
		log.Fatalf("err:", err)
	}

	m.vm_page_in.Set(float64(stat.PagePagein))
	m.vm_page_out.Set(float64(stat.PagePageout))
}

func collectHostStatsTask(wg *sync.WaitGroup) {
	defer wg.Done()

	pollWg := &sync.WaitGroup{}
	metrics := NewMetrics()

	// TODO: fix clock drift
	for {
		pollWg.Add(1)
		pollCpu(pollWg, metrics)

		pollWg.Add(1)
		pollRam(pollWg, metrics)

		pollWg.Add(1)
		pollNet(pollWg, metrics)

		pollWg.Add(1)
		pollDisk(pollWg, metrics)

		pollWg.Add(1)
		vmstat(pollWg, metrics)

		pollWg.Add(1)
		pollIOPS(pollWg, metrics)

		// wait until all poll operations are done
		pollWg.Wait()

		time.Sleep(5 * time.Second)
	}
}
