package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

func tlsSelfSignedConfig() (*tls.Config, error) {
	notBefore := time.Now().Add(-1 * time.Hour) // to account for clock drift
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"function61.com"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("certificate creation: %s", err.Error())
	}

	certOut := &bytes.Buffer{}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, err
	}

	keyOut := &bytes.Buffer{}
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, err
	}

	serverCert, err := tls.X509KeyPair(certOut.Bytes(), keyOut.Bytes())
	if err != nil {
		return nil, fmt.Errorf("keypair load: %s", err.Error())
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}, nil
}
