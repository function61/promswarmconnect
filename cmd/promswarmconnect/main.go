package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/function61/gokit/app/dynversion"
	"github.com/function61/gokit/app/udocker"
	"github.com/function61/gokit/log/logex"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/function61/gokit/os/osutil"
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

func registerTritonDiscoveryApi(mux *http.ServeMux) error {
	networkName, err := osutil.GetenvRequired("NETWORK_NAME")
	if err != nil {
		return err
	}

	dockerUrl, err := osutil.GetenvRequired("DOCKER_URL")
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
	mux.HandleFunc("/v1/discover", func(w http.ResponseWriter, r *http.Request) {
		services, err := listDockerServiceAndContainerInstances(
			r.Context(),
			dockerUrl,
			networkName,
			dockerClient)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		jsonResponse(w, serviceInstancesToTritonContainers(services))
	})

	return nil
}

func main() {
	rootLogger := logex.StandardLogger()

	osutil.ExitIfError(mainInternal(
		osutil.CancelOnInterruptOrTerminate(rootLogger),
		rootLogger))
}

func mainInternal(ctx context.Context, logger *log.Logger) error {
	logl := logex.Levels(logger)

	mux := http.NewServeMux()

	if err := registerTritonDiscoveryApi(mux); err != nil {
		return err
	}

	serverCert, err := tls.X509KeyPair([]byte(dummyCert), []byte(dummyCertKey))
	if err != nil {
		return err
	}

	// we need TLS even though calling Prometheus specifies InsecureSkipVerify, because
	// the code in Prometheus is hardcoded to use https. well, I guess encryption without
	// authentication is still better than no encryption at all.
	srv := &http.Server{
		Handler: mux,
		Addr:    ":443",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
		},
	}

	logl.Info.Printf("Started v%s", dynversion.Version)

	return httputils.CancelableServer(ctx, srv, func() error { return srv.ListenAndServeTLS("", "") })
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

	return osutil.GetenvRequiredFromBase64(key)
}

func jsonResponse(w http.ResponseWriter, output interface{}) {
	w.Header().Set("Content-Type", "application/json")

	jsonEncoder := json.NewEncoder(w)
	jsonEncoder.SetIndent("", "  ")
	if err := jsonEncoder.Encode(output); err != nil {
		// not safe to respond with HTTP error, because headers are most likely sent and
		// connection is probably broken
		log.Printf("jsonResponse: %v", err)
	}
}
