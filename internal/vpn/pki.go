// Package vpn handles VPN certificates and client configuration.
package vpn

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CA is the certificate authority that signs client certificates.
//
// Each user gets their own certificate, with their email as the common name.
// That matters for more than tidiness: OpenVPN reports the common name to the
// connect hook, so per-user certificates are what let the VPN know *who* has
// connected. With one shared certificate every client is indistinguishable,
// and two people connecting cannot be told apart.
type CA struct {
	cert    *x509.Certificate
	key     *rsa.PrivateKey
	certPEM []byte
}

// LoadCA reads an existing certificate authority from disk.
func LoadCA(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("CA certificate at %s is not valid PEM", certPath)
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("CA key at %s is not valid PEM", keyPath)
	}
	key, err := parsePrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing CA key: %w", err)
	}

	return &CA{cert: cert, key: key, certPEM: certPEM}, nil
}

// CreateCA generates a new certificate authority and writes it to disk.
//
// Intended for development and first-time setup. A production deployment
// should keep its CA key offline or in a hardware module; whoever holds it can
// mint credentials for anyone.
func CreateCA(certPath, keyPath, commonName string, validity time.Duration) (*CA, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("generating CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{"Dvarpala"}},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().Add(validity),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("creating CA certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "PRIVATE KEY", Bytes: mustMarshalKey(key),
	})

	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, fmt.Errorf("writing CA certificate: %w", err)
	}
	// The CA key signs every client credential: readable by its owner only.
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, fmt.Errorf("writing CA key: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &CA{cert: cert, key: key, certPEM: certPEM}, nil
}

// LoadOrCreateCA loads a CA, creating one if it does not exist yet.
func LoadOrCreateCA(certPath, keyPath, commonName string, validity time.Duration) (*CA, error) {
	if _, err := os.Stat(certPath); err == nil {
		return LoadCA(certPath, keyPath)
	}
	return CreateCA(certPath, keyPath, commonName, validity)
}

// CertificatePEM returns the CA certificate, for embedding in client profiles.
func (ca *CA) CertificatePEM() string { return string(ca.certPEM) }

// NotAfter reports when the CA itself expires.
func (ca *CA) NotAfter() time.Time { return ca.cert.NotAfter }

// ClientCredential is one user's certificate and private key.
type ClientCredential struct {
	CommonName  string
	Certificate string // PEM
	PrivateKey  string // PEM
	Serial      string
	NotAfter    time.Time
}

// IssueClient signs a certificate for one user.
//
// The common name is the user's email, so OpenVPN can report exactly who has
// connected. Extended key usage is restricted to client authentication: this
// certificate cannot be used to impersonate a server.
func (ca *CA) IssueClient(commonName string, validity time.Duration) (*ClientCredential, error) {
	if commonName == "" {
		return nil, fmt.Errorf("a common name is required")
	}
	if validity <= 0 {
		validity = 365 * 24 * time.Hour
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating client key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName, Organization: []string{"Dvarpala"}},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, fmt.Errorf("signing client certificate: %w", err)
	}

	return &ClientCredential{
		CommonName: commonName,
		Certificate: string(pem.EncodeToMemory(&pem.Block{
			Type: "CERTIFICATE", Bytes: der,
		})),
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{
			Type: "PRIVATE KEY", Bytes: mustMarshalKey(key),
		})),
		Serial:   serial.String(),
		NotAfter: template.NotAfter,
	}, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generating serial number: %w", err)
	}
	return serial, nil
}

func parsePrivateKey(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("CA key is not RSA")
	}
	return key, nil
}

func mustMarshalKey(key *rsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		// Only possible for an unsupported key type, which cannot occur here.
		panic(fmt.Sprintf("marshalling private key: %v", err))
	}
	return der
}

// IssueServer signs a certificate for the VPN server itself.
//
// Clients verify this with remote-cert-tls server, so the extended key usage
// must be server authentication. Using one authority for both ends means a
// client profile carries a single CA certificate and trusts exactly the server
// this deployment runs.
func (ca *CA) IssueServer(commonName string, hosts []string, validity time.Duration) (*ClientCredential, error) {
	if commonName == "" {
		return nil, fmt.Errorf("a common name is required")
	}
	if validity <= 0 {
		validity = 5 * 365 * 24 * time.Hour
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating server key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName, Organization: []string{"Dvarpala"}},
		NotBefore:    time.Now().Add(-5 * time.Minute),
		NotAfter:     time.Now().Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else if h != "" {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return nil, fmt.Errorf("signing server certificate: %w", err)
	}

	return &ClientCredential{
		CommonName:  commonName,
		Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		PrivateKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustMarshalKey(key)})),
		Serial:      serial.String(),
		NotAfter:    template.NotAfter,
	}, nil
}
