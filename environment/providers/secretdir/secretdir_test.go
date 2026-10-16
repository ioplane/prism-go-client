package secretdir

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/types"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	fixturePortSuffix   = ":9440"
	fixtureValueLength  = 10
	fixtureCertLength   = 512
	fixtureAPIKeyLength = 32
	fixtureFileMode     = 0o600
	fixtureDirLogFormat = "Temporary directory %s"
)

// Write regular and symlink file for given key-val pair.
func writeParam(t *testing.T, path, key, val string) error {
	t.Helper()

	filename := rand.String(fixtureValueLength)
	keyPath := filepath.Join(path, key)
	dataPath := filepath.Join(path, filename)

	if err := os.WriteFile(dataPath, []byte(val), fixtureFileMode); err != nil {
		return err
	}

	return os.Symlink(filename, keyPath)
}

func TestBasicAuth(t *testing.T) {
	matcher := gomega.NewWithT(t)
	path := t.TempDir()
	t.Logf(fixtureDirLogFormat, path)
	t.Setenv(envSecretDir, path)

	endpointHost := rand.String(fixtureValueLength)
	user := rand.String(fixtureValueLength)
	passwd := rand.String(fixtureValueLength)
	key := fmt.Sprintf("%s:9440:%s:%s", endpointHost, user, passwd)
	matcher.Expect(writeParam(t, path, secretKeyName, key)).To(gomega.Succeed())
	t.Log("Key", key)

	prov := &provider{}
	managementEndpoint, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).To(gomega.Succeed())
	matcher.Expect(managementEndpoint.Address.String()).To(gomega.BeIdenticalTo("https://" + endpointHost + fixturePortSuffix))
	matcher.Expect(managementEndpoint.ApiCredentials.Username).To(gomega.BeIdenticalTo(user))
	matcher.Expect(managementEndpoint.ApiCredentials.Password).To(gomega.BeIdenticalTo(passwd))
	matcher.Expect(managementEndpoint.ApiCredentials.KeyPair).To(gomega.BeEmpty())
}

func TestTLSAuth(t *testing.T) {
	matcher := gomega.NewWithT(t)
	path := t.TempDir()
	endpointHost := rand.String(fixtureValueLength)
	cert := rand.String(fixtureCertLength)

	t.Logf(fixtureDirLogFormat, path)
	t.Setenv(envSecretDir, path)
	matcher.Expect(writeParam(t, path, secretKeyCertName, cert)).To(gomega.Succeed())
	matcher.Expect(writeParam(t, path, secretKeyEndpoint,
		endpointHost+fixturePortSuffix)).To(gomega.Succeed())

	prov := &provider{}
	managementEndpoint, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).To(gomega.Succeed())
	matcher.Expect(managementEndpoint.Address.String()).To(gomega.BeIdenticalTo("https://" + endpointHost + fixturePortSuffix))
	matcher.Expect(managementEndpoint.ApiCredentials.Username).To(gomega.BeEmpty())
	matcher.Expect(managementEndpoint.ApiCredentials.Password).To(gomega.BeEmpty())
	matcher.Expect(managementEndpoint.ApiCredentials.KeyPair).To(gomega.BeIdenticalTo(cert))
}

func TestNutanixAuth(t *testing.T) {
	matcher := gomega.NewWithT(t)
	path := t.TempDir()
	endpointHost := rand.String(fixtureValueLength)
	user := rand.String(fixtureValueLength)
	passwd := rand.String(fixtureValueLength)
	creds := fmt.Sprintf(`
		[
			{
			  "type": "basic_auth", 
			  "data": { 
				"prismCentral":{
				  "username": "%s", 
				  "password": "%s"
				},
				"prismElements": null
			  }
			}
		]
	`, user, passwd)
	endpoint := endpointHost + fixturePortSuffix

	t.Logf(fixtureDirLogFormat, path)
	t.Setenv(envSecretDir, path)

	// Write three keys
	matcher.Expect(writeParam(t, path, secretKeyCredentials, creds)).To(gomega.Succeed())
	matcher.Expect(writeParam(t, path, secretKeyInsecure, "true")).To(gomega.Succeed())
	matcher.Expect(writeParam(t, path, secretKeyEndpoint, endpoint)).To(gomega.Succeed())

	prov := &provider{}
	managementEndpoint, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).To(gomega.Succeed())

	addr := &url.URL{Scheme: "https", Host: endpointHost + fixturePortSuffix}
	matcher.Expect(managementEndpoint).To(gomega.BeEquivalentTo(&types.ManagementEndpoint{
		Username: user,
		Password: passwd,
		Insecure: true,
		Address:  addr,
	}))
}

func TestNutanixAPIKeyAuth(t *testing.T) {
	matcher := gomega.NewWithT(t)
	path := t.TempDir()
	endpointHost := rand.String(fixtureValueLength)
	apiKey := rand.String(fixtureAPIKeyLength)
	creds := fmt.Sprintf(`
		[
			{
			  "type": "api_key",
			  "data": {
				"prismCentral":{
				  "apiKey": "%s"
				}
			  }
			}
		]
	`, apiKey)
	endpoint := endpointHost + fixturePortSuffix

	t.Logf(fixtureDirLogFormat, path)
	t.Setenv(envSecretDir, path)

	// Write three keys
	matcher.Expect(writeParam(t, path, secretKeyCredentials, creds)).To(gomega.Succeed())
	matcher.Expect(writeParam(t, path, secretKeyInsecure, "true")).To(gomega.Succeed())
	matcher.Expect(writeParam(t, path, secretKeyEndpoint, endpoint)).To(gomega.Succeed())

	prov := &provider{}
	managementEndpoint, err := prov.GetManagementEndpoint(nil)
	matcher.Expect(err).To(gomega.Succeed())

	addr := &url.URL{Scheme: "https", Host: endpointHost + fixturePortSuffix}
	matcher.Expect(managementEndpoint).To(gomega.BeEquivalentTo(&types.ManagementEndpoint{
		APIKey:   apiKey,
		Insecure: true,
		Address:  addr,
	}))
}

func TestEndpointDiagnostics(t *testing.T) {
	const (
		marker      = "synthetic-secretdir-url-secret-marker"
		userinfo    = "fixture:" + marker + "@%41.invalid"
		query       = "%41.invalid?token=" + marker
		fixtureData = `[{"type":"api_key","data":{"prismCentral":{"apiKey":"fixture-api-key"}}}]`
	)

	tests := []struct{ name, authKey, authValue, endpoint string }{
		{"JSON userinfo", secretKeyCredentials, fixtureData, userinfo},
		{"JSON query", secretKeyCredentials, fixtureData, query},
		{"TLS userinfo", secretKeyCertName, "fixture-keypair", userinfo},
		{"TLS query", secretKeyCertName, "fixture-keypair", query},
		{"legacy query", secretKeyName, query + ":9440:fixture-user:fixture-password", query + fixturePortSuffix},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matcher := gomega.NewWithT(t)
			path := t.TempDir()
			t.Setenv(envSecretDir, path)
			matcher.Expect(writeParam(t, path, test.authKey, test.authValue)).To(gomega.Succeed())

			if test.authKey != secretKeyName {
				matcher.Expect(writeParam(t, path, secretKeyEndpoint, test.endpoint)).To(gomega.Succeed())
			}

			endpoint, err := NewProvider().GetManagementEndpoint(nil)
			matcher.Expect(endpoint).To(gomega.BeNil())
			matcher.Expect(err).To(gomega.HaveOccurred())
			matcher.Expect(err.Error()).NotTo(gomega.ContainSubstring(marker))

			var cause *url.Error
			matcher.Expect(errors.As(err, &cause)).To(gomega.BeTrue())
			matcher.Expect(cause.URL).To(gomega.Equal("https://" + test.endpoint))
			matcher.Expect(cause.Op).To(gomega.Equal("parse"))
			matcher.Expect(cause.Err).To(gomega.HaveOccurred())
		})
	}
}
