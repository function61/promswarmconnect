package udocker

// μDocker - super-lightweight Docker client. pulling in the official client uses
// huge amounts of code and memory

import (
	"context"
	"crypto/tls"
	"github.com/function61/gokit/envvar"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func Client(dockerUrl string) (*http.Client, string, error) {
	u, err := url.Parse(dockerUrl)
	if err != nil {
		return nil, "", err
	}

	if u.Scheme == "unix" { // unix socket needs own dialer
		return &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
					dialer := net.Dialer{} // don't know why we need a struct to use DialContext()
					return dialer.DialContext(ctx, "unix", u.Path)
				},
			},
		}, "http://localhost", nil
	}

	clientCertificate, err := loadClientCertificate()
	if err != nil {
		return nil, "", err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{*clientCertificate},
		InsecureSkipVerify: true,
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}, dockerUrl, nil
}

func loadClientCertificate() (*tls.Certificate, error) {
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

	return envvar.GetFromBase64Encoded(key)
}
