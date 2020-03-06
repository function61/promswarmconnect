package main

import (
	"crypto/tls"
	"encoding/json"
	"github.com/function61/gokit/dynversion"
	"github.com/function61/gokit/envvar"
	"github.com/function61/gokit/logex"
	"github.com/function61/gokit/ossignal"
	"github.com/function61/gokit/stopper"
	"github.com/function61/gokit/udocker"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
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

func registerTritonDiscoveryApi() error {
	networkName, err := envvar.Required("NETWORK_NAME")
	if err != nil {
		return err
	}

	dockerUrl, err := envvar.Required("DOCKER_URL")
	if err != nil {
		return err
	}

	dockerClient, dockerUrlTransformed, err := udocker.Client(
		dockerUrl,
		clientCertFromEnvOrFile,
		true)
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

func runHttpServer(logl *logex.Leveled, stop *stopper.Stopper) error {
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

	mainlogl := logex.Levels(logex.Prefix("runHttpServer", rootLogger))

	if err := runHttpServer(mainlogl, workers.Stopper()); err != nil {
		mainlogl.Error.Fatal(err)
	}
}

func clientCertFromEnvOrFile() (*tls.Certificate, error) {
	clientCert, err := getDataFromEnvBase64OrFile("DOCKER_CLIENTCERT")
	if err != nil {
		return nil, err
	}

	clientCertKey, err := getDataFromEnvBase64OrFile("DOCKER_CLIENTCERT_KEY")
	if err != nil {
		return nil, err
	}

	clientKeypair, err := tls.X509KeyPair(clientCert, clientCertKey)
	if err != nil {
		return nil, err
	}

	return &clientKeypair, nil
}

// read ENV var (identified by key) value as base64, or if value begins with "@/home/foo/data.txt",
// value is read from that file. this is safe because "@" is not part of base64 alphabet
func getDataFromEnvBase64OrFile(key string) ([]byte, error) {
	if strings.HasPrefix(os.Getenv(key), "@") {
		path := os.Getenv(key)[1:] // remove leading "@"

		return ioutil.ReadFile(path)
	}

	return envvar.RequiredFromBase64Encoded(key)
}
