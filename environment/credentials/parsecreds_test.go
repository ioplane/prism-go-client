package credentials_test

import (
	"testing"

	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"

	"github.com/stretchr/testify/require"
)

const fixtureMissingAuthMessage = "either username and password, or an API key must be set in provider configuration"

const (
	parseInput1 = `not-json`
	parseInput2 = `[]`
	parseInput3 = `[
				{"type":"token","data":{"foo":"bar"}}
			]`
	parseInput4 = `[
				{
					"type":"basic_auth",
					"data":{
						"prismCentral":{"username":"test-user","password":"test-password"},
						"prismElements":null
					}
				}
			]`
	parseInput5 = `[
				{
					"type":"basic_auth",
					"data":"not-an-object"
				}
			]`
	parseInput6 = `[
				{
					"type":"basic_auth",
					"data":{
						"prismCentral":{"username":"","password":"test-password"},
						"prismElements":null
					}
				}
			]`
	parseInput7 = `[
				{
					"type":"basic_auth",
					"data":{
						"prismCentral":{"username":"test-user","password":""},
						"prismElements":null
					}
				}
			]`
	parseInput8 = `[
				{
					"type":"api_key",
					"data":{"prismCentral":{"apiKey":"test-api-key"}}
				}
			]`
	parseInput9 = `[
				{
					"type":"api_key",
					"data":"not-an-object"
				}
			]`
	parseInput10 = `[
				{
					"type":"api_key",
					"data":{"prismCentral":{"apiKey":""}}
				}
			]`
	parseInput11 = `[
				{
					"type":"basic_auth",
					"data":{
						"prismCentral":{"username":"first-user","password":"first-pass"},
						"prismElements":null
					}
				},
				{
					"type":"api_key",
					"data":{"prismCentral":{"apiKey":"second-api-key"}}
				}
			]`
	parseInput12 = `[
				{
					"type":"basic_auth",
					"data":{
						"prismCentral":{"username":"first-user","password":""},
						"prismElements":null
					}
				},
				{
					"type":"api_key",
					"data":{"prismCentral":{"apiKey":"second-api-key"}}
				}
			]`
)

type credentialCase struct {
	name        string
	credsData   []byte
	wantErr     bool
	errContains string
	wantBasic   *credentials.BasicAuthCredential
	wantAPIKey  *credentials.APIKeyCredential
}

func credentialErrorCase(name, data, detail string) credentialCase {
	return credentialCase{name: name, credsData: []byte(data), wantErr: true, errContains: detail, wantBasic: nil, wantAPIKey: nil}
}

func basicCredentialCase(name, data, username, password string) credentialCase {
	return credentialCase{
		name: name, credsData: []byte(data), wantErr: false, errContains: "", wantAPIKey: nil,
		wantBasic: &credentials.BasicAuthCredential{
			PrismCentral: credentials.PrismCentralBasicAuth{Username: username, Password: password}, PrismElements: nil,
		},
	}
}

func serviceCredentialCase(name, data, key string) credentialCase {
	return credentialCase{
		name: name, credsData: []byte(data), wantErr: false, errContains: "", wantBasic: nil,
		wantAPIKey: &credentials.APIKeyCredential{PrismCentral: credentials.PrismCentralAPIKey{APIKey: credentials.APIKey(key)}},
	}
}

func TestParseCredentials(t *testing.T) {
	tests := []credentialCase{
		credentialErrorCase("invalid top-level JSON", parseInput1, "failed to unmarshal the credentials data"),
		credentialErrorCase("empty credentials list", parseInput2, "no credentials found"),
		credentialErrorCase("unsupported credential type", parseInput3, "unsupported credentials type"),
		basicCredentialCase("basic auth success", parseInput4, "test-user", "test-password"),
		credentialErrorCase("basic auth data unmarshal failure", parseInput5, "failed to unmarshal the basic-auth data"),
		credentialErrorCase("basic auth missing username", parseInput6, fixtureMissingAuthMessage),
		credentialErrorCase("basic auth missing password", parseInput7, fixtureMissingAuthMessage),
		serviceCredentialCase("api key success", parseInput8, "test-api-key"),
		credentialErrorCase("api key data unmarshal failure", parseInput9, "failed to unmarshal the api key data"),
		credentialErrorCase("api key empty value", parseInput10, fixtureMissingAuthMessage),
		basicCredentialCase("multiple credentials uses first only", parseInput11, "first-user", "first-pass"),
		credentialErrorCase("multiple credentials first invalid fails even if second valid", parseInput12, fixtureMissingAuthMessage),
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			creds, err := credentials.ParseCredentials(test.credsData)
			if test.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, test.errContains)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, creds)

			if test.wantBasic != nil {
				require.Equal(t, test.wantBasic.PrismCentral.Username, creds.Username)
				require.Equal(t, test.wantBasic.PrismCentral.Password, creds.Password)
				require.Empty(t, creds.APIKey)

				return
			}

			if test.wantAPIKey != nil {
				require.Empty(t, creds.Username)
				require.Empty(t, creds.Password)
				require.Equal(t, string(test.wantAPIKey.PrismCentral.APIKey), creds.APIKey)

				return
			}

			require.Fail(t, "either wantBasic or wantAPIKey must be set for success cases")
		})
	}
}
