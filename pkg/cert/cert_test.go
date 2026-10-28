package cert

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	caPair, serverPair, err := GenerateSelfSignedCert("my-webhook", "default")
	if err != nil {
		t.Fatalf("failed to generate certs: %v", err)
	}

	// Verify server cert can be parsed by tls.X509KeyPair
	tlsCert, err := tls.X509KeyPair(serverPair.CertPEM, serverPair.KeyPEM)
	if err != nil {
		t.Fatalf("failed to parse tls key pair: %v", err)
	}
	if len(tlsCert.Certificate) == 0 {
		t.Fatalf("no certificates loaded")
	}

	// Verify CA cert pool verifies server cert
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPair.CertPEM) {
		t.Fatalf("failed to append CA cert to pool")
	}

	serverCert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse server x509 certificate: %v", err)
	}

	opts := x509.VerifyOptions{
		Roots:   caPool,
		DNSName: "my-webhook.default.svc",
	}
	if _, err := serverCert.Verify(opts); err != nil {
		t.Fatalf("failed to verify server cert against CA: %v", err)
	}
}
