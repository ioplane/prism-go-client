// Package certutils provides certificate fixtures for tests.
package certutils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"time"
)

const (
	testCertificateSerialNumber  = 2022
	testCertificateValidityYears = 10
	testRSAKeyBits               = 4096
)

// GenerateCACertForTesting generates a CA Certificate and returns a PEM encoded version of the certificate
// and a base64 encoded version of the PEM encoded certificate.
func GenerateCACertForTesting() (string, string, error) {
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(testCertificateSerialNumber),
		Subject: pkix.Name{
			Organization:  []string{"Nutanix, Inc."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Jose"},
			StreetAddress: []string{"Technology Drive"},
			PostalCode:    []string{"95110"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(testCertificateValidityYears, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Create the private and public key.
	caPrivKey, err := rsa.GenerateKey(rand.Reader, testRSAKeyBits)
	if err != nil {
		return "", "", err
	}

	// Create the CA.
	caBytes, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return "", "", err
	}

	// PEM encode.
	caPEM := new(bytes.Buffer)

	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return "", "", err
	}

	b64Cert := base64.StdEncoding.EncodeToString(caPEM.Bytes())

	return caPEM.String(), b64Cert, nil
}
