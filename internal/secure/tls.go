// Package secure builds gRPC transport credentials for the internal service
// links (tm↔db, tm↔bin). Those links are modern and owned on both ends, so they
// run mTLS (migration-plan.md §5: "mTLS nos links internos") — unlike the
// client edge, which is stuck on the legacy CPSock.
//
// Certificate material is referenced only by file path (from flags/env); nothing
// is embedded. When no paths are configured the credentials fall back to
// insecure transport for local bring-up — production must set all three paths.
package secure

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Config points at the PEM files for one side of an mTLS link.
type Config struct {
	CertFile   string // this peer's certificate
	KeyFile    string // this peer's private key
	CAFile     string // CA that signs the other peer's certificate
	ServerName string // expected server name (client side only)
}

// Enabled reports whether any TLS material was configured. When false the
// credentials helpers return insecure transport.
func (c Config) Enabled() bool {
	return c.CertFile != "" || c.KeyFile != "" || c.CAFile != ""
}

// ServerCreds builds credentials for a gRPC server. With TLS enabled it requires
// and verifies a client certificate (mutual TLS).
func ServerCreds(c Config) (credentials.TransportCredentials, error) {
	if !c.Enabled() {
		return insecure.NewCredentials(), nil
	}
	cert, pool, err := load(c)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

// ClientCreds builds credentials for a gRPC client, presenting this peer's
// certificate and verifying the server against the configured CA.
func ClientCreds(c Config) (credentials.TransportCredentials, error) {
	if !c.Enabled() {
		return insecure.NewCredentials(), nil
	}
	cert, pool, err := load(c)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   c.ServerName,
		MinVersion:   tls.VersionTLS13,
	}), nil
}

// load reads the keypair and CA pool, requiring all three paths to be set.
func load(c Config) (tls.Certificate, *x509.CertPool, error) {
	if c.CertFile == "" || c.KeyFile == "" || c.CAFile == "" {
		return tls.Certificate{}, nil, fmt.Errorf("secure: cert, key and CA files are all required for mTLS")
	}
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("secure: load keypair: %w", err)
	}
	caPEM, err := os.ReadFile(c.CAFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("secure: read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, fmt.Errorf("secure: no certificates parsed from %s", c.CAFile)
	}
	return cert, pool, nil
}
