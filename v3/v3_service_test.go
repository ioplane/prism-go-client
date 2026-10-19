package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/keploy/go-sdk/integrations/khttpclient"
	"github.com/keploy/go-sdk/keploy"
	"github.com/keploy/go-sdk/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/internal"
	"github.com/nutanix-cloud-native/prism-go-client/internal/testhelpers"
	"github.com/nutanix-cloud-native/prism-go-client/v3/models"
)

func setup(t *testing.T) (*http.ServeMux, *internal.Client, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	creds := &prismgoclient.Credentials{
		URL:      "",
		Username: "username",
		Password: "password",
		Port:     "",
		Endpoint: "0.0.0.0",
		Insecure: true,
	}
	client, err := internal.NewClient(
		internal.WithCredentials(creds),
		internal.WithUserAgent(userAgent),
		internal.WithAbsolutePath(absolutePath),
		internal.WithBaseURL(server.URL))
	if err != nil {
		server.Close()
	}

	require.NoError(t, err)

	return mux, client, server
}

func TestOperations_CreateSubnet(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/subnets", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "subnet",
			},
			"spec": map[string]any{
				"cluster_reference": map[string]any{
					"kind": "cluster",
					"uuid": "00056024-6c13-4c74-0000-00000000ecb5",
				},
				"name":      "subnet.create",
				"resources": nil,
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "subnet",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	createRequest := &SubnetIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("subnet"),
		},
		Spec: &models.Subnet{
			ClusterReference: &models.ClusterReference{
				Kind: "cluster",
				UUID: new("00056024-6c13-4c74-0000-00000000ecb5"),
			},
			Name: new("subnet.create"),
		},
	}

	expected := &SubnetIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("subnet"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateSubnet", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateSubnet(context.Background(), createRequest)
		testOperationResult(t, "CreateSubnet", got, expected, err, false)
	})
}

func TestOperations_DeleteSubnet(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/subnets/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)

		_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "subnet",
					"categories": {
						"Project": "default"
					}
				}
			}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteSubnet OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteSubnet Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteSubnet(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteSubnet() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_UpdateSubnet(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/subnets/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "subnet",
			},
			"spec": map[string]any{
				"cluster_reference": map[string]any{
					"kind": "cluster",
					"uuid": "00056024-6c13-4c74-0000-00000000ecb5",
				},
				"name":      "subnet.create",
				"resources": nil,
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "subnet",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &SubnetIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("subnet"),
		},
		Spec: &models.Subnet{
			ClusterReference: &models.ClusterReference{
				Kind: "cluster",
				UUID: new("00056024-6c13-4c74-0000-00000000ecb5"),
			},
			Name: new("subnet.create"),
		},
	}

	expected := &SubnetIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("subnet"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test UpdateSubnet", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateSubnet(context.Background(), uuid, body)
		testOperationResult(t, "UpdateSubnet", got, expected, err, false)
	})
}

func TestOperations_CreateImage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/images", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "image",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"image_type": "DISK_IMAGE",
					"data_source_reference": map[string]any{
						"kind": "vm_disk",
						"uuid": "0005a238-f165-08ba-317e-ac1f6b6e5442",
					},
				},
				"name": "image.create",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "image",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	body := &ImageIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("image"),
		},
		Spec: &Image{
			Name: new("image.create"),
			Resources: &ImageResources{
				ImageType: new("DISK_IMAGE"),
				DataSourceReference: &Reference{
					Kind: new("vm_disk"),
					UUID: new("0005a238-f165-08ba-317e-ac1f6b6e5442"),
				},
			},
		},
	}

	expected := &ImageIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("image"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateImage", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateImage(context.Background(), body)
		testOperationResult(t, "CreateImage", got, expected, err, false)
	})
}

func TestOperations_UploadImageError(t *testing.T) {
	_, client, server := setup(t)

	defer server.Close()

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	filepath := "xx"

	t.Run("Test UploadImage ERROR (Cannot Open File)", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}
		if err := op.UploadImage(context.Background(), uuid, filepath); (err != nil) != true {
			t.Errorf("Operations.UploadImage() error = %v, wantErr %v", err, true)
		}
	})
}

func TestOperations_UploadImage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/images/cfde831a-4e87-4a75-960f-89b0148aa2cc/file", func(_ http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPut)

		bodyBytes, _ := io.ReadAll(r.Body)
		file, _ := os.ReadFile("v3.go")

		if !reflect.DeepEqual(bodyBytes, file) {
			t.Errorf("Operations.UploadImage() error: different uploaded files")
		}
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	filepath := "v3.go"

	t.Run("TestOperations_UploadImage Upload Image", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}
		if err := op.UploadImage(context.Background(), uuid, filepath); err != nil {
			t.Errorf("Operations.UploadImage() error = %v", err)
		}
	})
}

func TestOperations_DeleteImage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/images/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)

		_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "image",
					"categories": {
						"Project": "default"
					}
				}
			}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteImage OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteImage Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteImage(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteImage() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_GetImage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/images/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"image","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	response := &ImageIntentResponse{}
	response.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("image"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := response

	t.Run("Test GetImage OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetImage(context.Background(), uuid)
		testOperationResult(t, "GetImage", got, expected, err, false)
	})
}

func TestOperations_ListImage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/images/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"image","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	list := &ImageListIntentResponse{}
	list.Entities = make([]*ImageIntentResponse, 1)
	list.Entities[0] = &ImageIntentResponse{}
	list.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("image"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := list

	t.Run("Test ListImage OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListImage(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListImage", got, expected, err, false)
	})
}

func TestOperations_UpdateImage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/images/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "image",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"image_type": "DISK_IMAGE",
				},
				"name": "image.update",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "image",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &ImageIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("image"),
		},
		Spec: &Image{
			Resources: &ImageResources{
				ImageType: new("DISK_IMAGE"),
			},
			Name: new("image.update"),
		},
	}

	expected := &ImageIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("image"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test UpdateVM", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateImage(context.Background(), uuid, body)
		testOperationResult(t, "UpdateImage", got, expected, err, false)
	})
}

func TestOperations_CreateOrUpdateCategoryKey(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"description": "Testing Keys",
			"name":        "test_category_key",
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"description": "Testing Keys",
			"name": "test_category_key",
			"system_defined": false
		}`)
	})

	operationClient := client

	body := &CategoryKey{
		Description: new("Testing Keys"),
		Name:        new("test_category_key"),
	}

	expected := &CategoryKeyStatus{
		Description:   new("Testing Keys"),
		Name:          new("test_category_key"),
		SystemDefined: new(false),
	}

	t.Run("Test CreateOrUpdateCaegoryKey OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateOrUpdateCategoryKey(context.Background(), body)
		testOperationResult(t, "CreateOrUpdateCategoryKey", got, expected, err, false)
	})
}

func TestOperations_ListCategories(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{ "description": "Testing Keys", "name": "test_category_key", "system_defined": false }]}`)
	})

	list := &CategoryKeyListResponse{}
	list.Entities = make([]*CategoryKeyStatus, 1)
	list.Entities[0] = &CategoryKeyStatus{
		Description:   new("Testing Keys"),
		Name:          new("test_category_key"),
		SystemDefined: new(false),
	}

	input := &CategoryListMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := list

	t.Run("Test ListCategoryKey OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListCategories(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListCategories", got, expected, err, false)
	})
}

func TestOperations_DeleteCategoryKey(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key", func(_ http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		name string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteSubnet OK",
			fields{client},
			args{"test_category_key"},
			false,
		},

		{
			"Test SubnetVM Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if err := op.DeleteCategoryKey(context.Background(), testCase.args.name); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteCategoryKey() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_GetCategoryKey(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{
			"description": "Testing Keys",
			"name": "test_category_key",
			"system_defined": false
		}`)
	})

	response := &CategoryKeyStatus{
		Description:   new("Testing Keys"),
		Name:          new("test_category_key"),
		SystemDefined: new(false),
	}

	operationClient := client

	name := "test_category_key"

	expected := response

	t.Run("Test GetCategory OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetCategoryKey(context.Background(), name)
		testOperationResult(t, "GetCategoryKey", got, expected, err, false)
	})
}

func TestOperations_ListCategoryValues(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{ "description": "Testing Keys", "value": "test_category_value", "system_defined": false }]}`)
	})

	list := &CategoryValueListResponse{}
	list.Entities = make([]*CategoryValueStatus, 1)
	list.Entities[0] = &CategoryValueStatus{
		Description:   new("Testing Keys"),
		Value:         new("test_category_value"),
		SystemDefined: new(false),
	}

	input := &CategoryListMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	name := "test_category_key"

	getEntitiesRequest := input

	expected := list

	t.Run("Test ListCategoryKey OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListCategoryValues(context.Background(), name, getEntitiesRequest)
		testOperationResult(t, "ListCategoryValues", got, expected, err, false)
	})
}

func TestOperations_CreateOrUpdateCategoryValue(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key/test_category_value", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"description": "Testing Value",
			"value":       "test_category_value",
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"description": "Testing Value",
			"name": "test_category_key",
			"value": "test_category_value",
			"system_defined": false
		}`)
	})

	operationClient := client

	name := "test_category_key"

	body := &CategoryValue{
		Description: new("Testing Value"),
		Value:       new("test_category_value"),
	}

	expected := &CategoryValueStatus{
		Description:   new("Testing Value"),
		Value:         new("test_category_value"),
		Name:          new("test_category_key"),
		SystemDefined: new(false),
	}

	t.Run("Test CreateOrUpdateCategoryValue OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateOrUpdateCategoryValue(context.Background(), name, body)
		testOperationResult(t, "CreateOrUpdateCategoryValue", got, expected, err, false)
	})
}

func TestOperations_GetCategoryValue(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key/test_category_value", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{
			"description": "Testing Value",
			"name": "test_category_key",
			"value": "test_category_value",
			"system_defined": false
		}`)
	})

	response := &CategoryValueStatus{
		Description:   new("Testing Value"),
		Name:          new("test_category_key"),
		Value:         new("test_category_value"),
		SystemDefined: new(false),
	}

	operationClient := client

	name := "test_category_key"

	value := "test_category_value"

	expected := response

	t.Run("Test GetCategoryValue OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetCategoryValue(context.Background(), name, value)
		testOperationResult(t, "GetCategoryValue", got, expected, err, false)
	})
}

func TestOperations_DeleteCategoryValue(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/categories/test_category_key/test_category_value", func(_ http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		name  string
		value string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteSubnet OK",
			fields{client},
			args{"test_category_key", "test_category_value"},
			false,
		},

		{
			"Test SubnetVM Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if err := op.DeleteCategoryValue(context.Background(), testCase.args.name, testCase.args.value); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteCategoryValue() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_GetCategoryQuery(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/category/query", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"results":[{ "kind": "category_result" }]}`)
	})

	response := &CategoryQueryResponse{}
	response.Results = make([]*CategoryQueryResponseResults, 1)
	response.Results[0] = &CategoryQueryResponseResults{
		Kind: new("category_result"),
	}

	input := &CategoryQueryInput{
		UsageType: new("APPLIED_TO"),
	}

	operationClient := client

	query := input

	expected := response

	t.Run("Test Category Query OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetCategoryQuery(context.Background(), query)
		testOperationResult(t, "GetCategoryQuery", got, expected, err, false)
	})
}

func TestOperations_CreateNetworkSecurityRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/network_security_rules", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "network_security_rule",
			},
			"spec": map[string]any{
				"description": "Network Create",
				"name":        "network.create",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "network_security_rule",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &NetworkSecurityRuleIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("network_security_rule"),
		},
		Spec: &NetworkSecurityRule{
			Name:        new("network.create"),
			Description: new("Network Create"),
			Resources:   nil,
		},
	}

	expected := &NetworkSecurityRuleIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("network_security_rule"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateNetwork", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateNetworkSecurityRule(context.Background(), request)
		testOperationResult(t, "CreateNetworkSecurityRule", got, expected, err, false)
	})
}

func TestOperations_DeleteNetworkSecurityRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/network_security_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc",
		func(w http.ResponseWriter, r *http.Request) {
			testHTTPMethod(t, r, http.MethodDelete)

			_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "network_security_rule",
					"categories": {
						"Project": "default"
					}
				}
			}`)
		})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteNetwork OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteNetowork Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteNetworkSecurityRule(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteNetworkSecurityRule() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_GetNetworkSecurityRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/network_security_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc",
		func(w http.ResponseWriter, r *http.Request) {
			testHTTPMethod(t, r, http.MethodGet)

			_, _ = fmt.Fprint(w, `{"metadata": {"kind":"network_security_rule","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
		})

	response := &NetworkSecurityRuleIntentResponse{}
	response.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("network_security_rule"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := response

	t.Run("Test GetNetworkSecurityRule OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetNetworkSecurityRule(context.Background(), uuid)
		testOperationResult(t, "GetNetworkSecurityRule", got, expected, err, false)
	})
}

func TestOperations_ListNetworkSecurityRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/network_security_rules/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"network_security_rule","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	list := &NetworkSecurityRuleListIntentResponse{}
	list.Entities = make([]*NetworkSecurityRuleIntentResource, 1)
	list.Entities[0] = &NetworkSecurityRuleIntentResource{}
	list.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("network_security_rule"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := list

	t.Run("Test ListNetwork OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListNetworkSecurityRule(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListNetworkSecurityRule", got, expected, err, false)
	})
}

func TestOperations_UpdateNetworkSecurityRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/network_security_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc",
		func(writer http.ResponseWriter, request *http.Request) {
			testHTTPMethod(t, request, http.MethodPut)

			expected := map[string]any{
				"api_version": "3.1",
				"metadata": map[string]any{
					"kind": "network_security_rule",
				},
				"spec": map[string]any{
					"description": "Network Update",
					"name":        "network.update",
				},
			}

			var actual map[string]any

			err := json.NewDecoder(request.Body).Decode(&actual)
			if err != nil {
				t.Errorf("decode json: %v", err)

				return
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
			}

			_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "network_security_rule",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
		})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &NetworkSecurityRuleIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("network_security_rule"),
		},
		Spec: &NetworkSecurityRule{
			Resources:   nil,
			Description: new("Network Update"),
			Name:        new("network.update"),
		},
	}

	expected := &NetworkSecurityRuleIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("network_security_rule"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test UpdateNetwork", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateNetworkSecurityRule(context.Background(), uuid, body)
		testOperationResult(t, "UpdateNetworkSecurityRule", got, expected, err, false)
	})
}

func TestOperations_CreateVolumeGroup(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/volume_groups", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "volume_group",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"flash_mode": "ON",
				},
				"name": "volume.create",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "volume_group",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &VolumeGroupInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("volume_group"),
		},
		Spec: &VolumeGroup{
			Name: new("volume.create"),
			Resources: &VolumeGroupResources{
				FlashMode: new("ON"),
			},
		},
	}

	expected := &VolumeGroupResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("volume_group"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateVolumeGroup", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateVolumeGroup(context.Background(), request)
		testOperationResult(t, "CreateVolumeGroup", got, expected, err, false)
	})
}

func TestOperations_DeleteVolumeGroup(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/volume_groups/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(_ http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteVolumeGroup OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteVolumeGroup Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if err := op.DeleteVolumeGroup(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteVolumeGroup() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_GetVolumeGroup(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/volume_groups/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"volume_group","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	response := &VolumeGroupResponse{}
	response.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("volume_group"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := response

	t.Run("Test GetVolumeGroup OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetVolumeGroup(context.Background(), uuid)
		testOperationResult(t, "GetVolumeGroup", got, expected, err, false)
	})
}

func TestOperations_ListVolumeGroup(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/volume_groups/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"volume_group","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	list := &VolumeGroupListResponse{}
	list.Entities = make([]*VolumeGroupResponse, 1)
	list.Entities[0] = &VolumeGroupResponse{}
	list.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("volume_group"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := list

	t.Run("Test ListVolumeGroup OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListVolumeGroup(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListVolumeGroup", got, expected, err, false)
	})
}

func TestOperations_UpdateVolumeGroup(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/volume_groups/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"kind": "volume_group",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"flash_mode": "ON",
				},
				"name": "volume.update",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "volume_group",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &VolumeGroupInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("volume_group"),
		},
		Spec: &VolumeGroup{
			Resources: &VolumeGroupResources{
				FlashMode: new("ON"),
			},
			Name: new("volume.update"),
		},
	}

	expected := &VolumeGroupResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("volume_group"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test UpdateVolumeGroup", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateVolumeGroup(context.Background(), uuid, body)
		testOperationResult(t, "UpdateVolumeGroup", got, expected, err, false)
	})
}

func testHTTPMethod(t *testing.T, r *http.Request, expected string) {
	if expected != r.Method {
		t.Errorf("Request method = %v, expected %v", r.Method, expected)
	}
}

func TestOperations_GetHost(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/hosts/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	hostResponse := &HostResponse{}
	hostResponse.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := hostResponse

	t.Run("Test GetHost OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetHost(context.Background(), uuid)
		testOperationResult(t, "GetHost", got, expected, err, false)
	})
}

func TestOperations_ListHost(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/hosts/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	hostList := &HostListResponse{}
	hostList.Entities = make([]*HostResponse, 1)
	hostList.Entities[0] = &HostResponse{}
	hostList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := hostList

	t.Run("Test ListSubnet OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListHost(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListHost", got, expected, err, false)
	})
}

func TestOperations_CreateProject(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/projects", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "project_test_name",
				"kind": "project",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"resource_domain": map[string]any{
						"resources": []any{
							map[string]any{
								"limit":         float64(4),
								"resource_type": "resource_type_test",
							},
						},
					},
				},
				"name":        "project_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "project",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &Project{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Name: new("project_test_name"),
			Kind: new("project"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &ProjectSpec{
			Name:       "project_name",
			Descripion: "description_test",
			Resources: &ProjectResources{
				ResourceDomain: &ResourceDomain{
					Resources: []*Resources{
						{
							Limit:        new(int64(4)),
							ResourceType: "resource_type_test",
						},
					},
				},
			},
		},
	}

	expected := &Project{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("project"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateProject", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateProject(context.Background(), request)
		testOperationResult(t, "CreateProject", got, expected, err, false)
	})
}

func TestOperations_GetProject(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/projects/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	hostResponse := &Project{}
	hostResponse.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := hostResponse

	t.Run("Test GetProject OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetProject(context.Background(), uuid)
		testOperationResult(t, "GetProject", got, expected, err, false)
	})
}

func TestOperations_ListProject(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/projects/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	hostList := &ProjectListResponse{}
	hostList.Entities = make([]*Project, 1)
	hostList.Entities[0] = &Project{}
	hostList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := hostList

	t.Run("Test ListSubnet OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListProject(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListProject", got, expected, err, false)
	})
}

func TestOperations_UpdateProject(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/projects/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"metadata": map[string]any{
				"name": "project_test_name",
				"kind": "project",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"resource_domain": map[string]any{
						"resources": []any{
							map[string]any{
								"limit":         float64(4),
								"resource_type": "resource_type_test",
							},
						},
					},
				},
				"name":        "project_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "project",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &Project{
		Metadata: &Metadata{
			Name: new("project_test_name"),
			Kind: new("project"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &ProjectSpec{
			Name:       "project_name",
			Descripion: "description_test",
			Resources: &ProjectResources{
				ResourceDomain: &ResourceDomain{
					Resources: []*Resources{
						{
							Limit:        new(int64(4)),
							ResourceType: "resource_type_test",
						},
					},
				},
			},
		},
	}

	expected := &Project{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("project"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateProject", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateProject(context.Background(), uuid, body)
		testOperationResult(t, "UpdateProject", got, expected, err, false)
	})
}

func TestOperations_DeleteProject(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/projects/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)

		_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "projects",
					"categories": {
						"Project": "default"
					}
				}
			}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteProject OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteProject Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteProject(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteProject() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_CreateAccessControlPolicy(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/access_control_policies", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "access_control_policy_test_name",
				"kind": "access_control_policy",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"role_reference": map[string]any{
						"name": "access_control_policy_test_name",
						"kind": "role",
						"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
					},
				},
				"name":        "access_control_policy_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "access_control_policy",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &AccessControlPolicy{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Name: new("access_control_policy_test_name"),
			Kind: new("access_control_policy"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &AccessControlPolicySpec{
			Name:        new("access_control_policy_name"),
			Description: new("description_test"),
			Resources: &AccessControlPolicyResources{
				RoleReference: &Reference{
					Kind: new("role"),
					UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
					Name: new("access_control_policy_test_name"),
				},
			},
		},
	}

	expected := &AccessControlPolicy{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("access_control_policy"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateAccessControlPolicy", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateAccessControlPolicy(context.Background(), request)
		testOperationResult(t, "CreateAccessControlPolicy", got, expected, err, false)
	})
}

func TestOperations_GetAccessControlPolicy(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/access_control_policies/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	hostResponse := &AccessControlPolicy{}
	hostResponse.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := hostResponse

	t.Run("Test GetAccessControlPolicy OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetAccessControlPolicy(context.Background(), uuid)
		testOperationResult(t, "GetAccessControlPolicy", got, expected, err, false)
	})
}

func TestOperations_ListAccessControlPolicy(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/access_control_policies/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	hostList := &AccessControlPolicyListResponse{}
	hostList.Entities = make([]*AccessControlPolicy, 1)
	hostList.Entities[0] = &AccessControlPolicy{}
	hostList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := hostList

	t.Run("Test ListSubnet OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListAccessControlPolicy(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListAccessControlPolicy", got, expected, err, false)
	})
}

func TestOperations_UpdateAccessControlPolicy(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/access_control_policies/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"metadata": map[string]any{
				"name": "access_control_policy_test_name",
				"kind": "access_control_policy",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"role_reference": map[string]any{
						"name": "access_control_policy_test_name_2",
						"kind": "role",
						"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
					},
				},
				"name":        "access_control_policy_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "access_control_policy",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &AccessControlPolicy{
		Metadata: &Metadata{
			Name: new("access_control_policy_test_name"),
			Kind: new("access_control_policy"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &AccessControlPolicySpec{
			Name:        new("access_control_policy_name"),
			Description: new("description_test"),
			Resources: &AccessControlPolicyResources{
				RoleReference: &Reference{
					Kind: new("role"),
					UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
					Name: new("access_control_policy_test_name_2"),
				},
			},
		},
	}

	expected := &AccessControlPolicy{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("access_control_policy"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateAccessControlPolicy", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateAccessControlPolicy(context.Background(), uuid, body)
		testOperationResult(t, "UpdateAccessControlPolicy", got, expected, err, false)
	})
}

func TestOperations_DeleteAccessControlPolicy(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/access_control_policies/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)

		_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "access_control_policy",
					"categories": {
						"Project": "default"
					}
				}
			}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteAccessControlPolicy OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteAccessControlPolicy Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteAccessControlPolicy(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteAccessControlPolicy() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_CreateRole(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/roles", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "role_test_name",
				"kind": "role",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"permission_reference_list": []any{
						map[string]any{
							"name": "role_test_name",
							"kind": "role",
							"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
						},
					},
				},
				"name":        "role_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "role",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &Role{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Name: new("role_test_name"),
			Kind: new("role"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &RoleSpec{
			Name:        new("role_name"),
			Description: new("description_test"),
			Resources: &RoleResources{
				PermissionReferenceList: []*Reference{
					{
						Kind: new("role"),
						UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
						Name: new("role_test_name"),
					},
				},
			},
		},
	}

	expected := &Role{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("role"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateRole", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateRole(context.Background(), request)
		testOperationResult(t, "CreateRole", got, expected, err, false)
	})
}

func TestOperations_GetRole(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/roles/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	hostResponse := &Role{}
	hostResponse.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := hostResponse

	t.Run("Test GetRole OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetRole(context.Background(), uuid)
		testOperationResult(t, "GetRole", got, expected, err, false)
	})
}

func TestOperations_ListRole(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/roles/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"host","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	hostList := &RoleListResponse{}
	hostList.Entities = make([]*Role, 1)
	hostList.Entities[0] = &Role{}
	hostList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("host"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := hostList

	t.Run("Test ListSubnet OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListRole(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListRole", got, expected, err, false)
	})
}

func TestOperations_UpdateRole(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/roles/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"metadata": map[string]any{
				"name": "role_test_name",
				"kind": "role",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"permission_reference_list": []any{
						map[string]any{
							"name": "role_test_name",
							"kind": "role",
							"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
						},
					},
				},
				"name":        "role_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "role",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &Role{
		Metadata: &Metadata{
			Name: new("role_test_name"),
			Kind: new("role"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &RoleSpec{
			Name:        new("role_name"),
			Description: new("description_test"),
			Resources: &RoleResources{
				PermissionReferenceList: []*Reference{
					{
						Kind: new("role"),
						UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
						Name: new("role_test_name"),
					},
				},
			},
		},
	}

	expected := &Role{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("role"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateRole", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateRole(context.Background(), uuid, body)
		testOperationResult(t, "UpdateRole", got, expected, err, false)
	})
}

func TestOperations_DeleteRole(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/roles/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)

		_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "role",
					"categories": {
						"Project": "default"
					}
				}
			}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteRole OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteRole Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteRole(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteRole() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_CreateUser(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/users", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "user_name",
				"kind": "user",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"directory_service_user": map[string]any{
						"directory_service_reference": map[string]any{
							"kind": "directory_service",
							"uuid": "d8b53470-c432-4556-badd-a11c937d89c9",
						},
						"user_principal_name": "user-dummy-tbd@ntnx.local",
					},
				},
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "user",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &UserIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Name: new("user_name"),
			Kind: new("user"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &UserSpec{
			Resources: &UserResources{
				DirectoryServiceUser: &DirectoryServiceUser{
					DirectoryServiceReference: &Reference{
						Kind: new("directory_service"),
						UUID: new("d8b53470-c432-4556-badd-a11c937d89c9"),
					},
					UserPrincipalName: new("user-dummy-tbd@ntnx.local"),
				},
			},
		},
	}

	expected := &UserIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("user"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateUser", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateUser(context.Background(), request)
		testOperationResult(t, "CreateUser", got, expected, err, false)
	})
}

func TestOperations_GetUser(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/users/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"user","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	hostResponse := &UserIntentResponse{}
	hostResponse.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("user"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := hostResponse

	t.Run("Test GetUser OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetUser(context.Background(), uuid)
		testOperationResult(t, "GetUser", got, expected, err, false)
	})
}

func TestOperations_ListUser(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/users/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"user","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	hostList := &UserListResponse{}
	hostList.Entities = make([]*UserIntentResponse, 1)
	hostList.Entities[0] = &UserIntentResponse{}
	hostList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("user"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := hostList

	t.Run("Test ListUser OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListUser(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListUser", got, expected, err, false)
	})
}

func TestOperations_UpdateUser(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/users/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "user_name",
				"kind": "user",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"directory_service_user": map[string]any{
						"directory_service_reference": map[string]any{
							"kind": "directory_service",
							"uuid": "d8b53470-c432-4556-badd-a11c937d89c9",
						},
						"user_principal_name": "user-dummy-tbd@ntnx.local",
					},
				},
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "user",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &UserIntentInput{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Name: new("user_name"),
			Kind: new("user"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &UserSpec{
			Resources: &UserResources{
				DirectoryServiceUser: &DirectoryServiceUser{
					DirectoryServiceReference: &Reference{
						Kind: new("directory_service"),
						UUID: new("d8b53470-c432-4556-badd-a11c937d89c9"),
					},
					UserPrincipalName: new("user-dummy-tbd@ntnx.local"),
				},
			},
		},
	}

	expected := &UserIntentResponse{
		APIVersion: new("3.1"),
		Metadata: &Metadata{
			Kind: new("user"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateUser", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateUser(context.Background(), uuid, body)
		testOperationResult(t, "UpdateUser", got, expected, err, false)
	})
}

func TestOperations_DeleteUser(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/users/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)

		_, _ = fmt.Fprintf(w, `{
				"status": {
					"state": "DELETE_PENDING",
					"execution_context": {
						"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"
					}
				},
				"spec": "",
				"api_version": "3.1",
				"metadata": {
					"kind": "user",
					"categories": {
						"Project": "default"
					}
				}
			}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteUser OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteUser Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteUser(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteUser() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_CreateProtectionRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/protection_rules", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "protection_rule_test_name",
				"kind": "protection_rule",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"start_time": "00h:00m",
					"ordered_availability_zone_list": []any{
						map[string]any{
							"availability_zone_url": "url test",
							"cluster_uuid":          "cfde831a-4e87-4a75-960f-89b0148aa2cc",
						},
					},
					"availability_zone_connectivity_list": []any{
						map[string]any{
							"destination_availability_zone_index": float64(0),
							"source_availability_zone_index":      float64(0),
							"snapshot_schedule_list": []any{
								map[string]any{
									"recovery_point_objective_secs": float64(0),
									"auto_suspend_timeout_secs":     float64(0),
									"snapshot_type":                 "CRASH_CONSISTENT",
									"local_snapshot_retention_policy": map[string]any{
										"num_snapshots": float64(1),
										"rollup_retention_policy": map[string]any{
											"snapshot_interval_type": "HOURLY",
											"multiple":               float64(1),
										},
									},
									"remote_snapshot_retention_policy": map[string]any{
										"num_snapshots": float64(1),
										"rollup_retention_policy": map[string]any{
											"snapshot_interval_type": "HOURLY",
											"multiple":               float64(1),
										},
									},
								},
							},
						},
					},
					"category_filter": map[string]any{
						"type":      "CATEGORIES_MATCH_ALL",
						"kind_list": []any{"1", "2"},
					},
				},
				"name":        "protection_rule_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		assertions := assert.New(t)
		if !assertions.Equal(actual, expected, "The response should be the same") {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "protection_rule",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	request := &ProtectionRuleInput{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Name: new("protection_rule_test_name"),
			Kind: new("protection_rule"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &ProtectionRuleSpec{
			Name:        "protection_rule_name",
			Description: "description_test",
			Resources: &ProtectionRuleResources{
				StartTime: "00h:00m",
				OrderedAvailabilityZoneList: []*OrderedAvailabilityZoneList{
					{
						AvailabilityZoneURL: "url test",
						ClusterUUID:         "cfde831a-4e87-4a75-960f-89b0148aa2cc",
					},
				},
				AvailabilityZoneConnectivityList: []*AvailabilityZoneConnectivityList{
					{
						DestinationAvailabilityZoneIndex: new(int64(0)),
						SourceAvailabilityZoneIndex:      new(int64(0)),
						SnapshotScheduleList: []*SnapshotScheduleList{
							{
								RecoveryPointObjectiveSecs: new(int64(0)),
								AutoSuspendTimeoutSecs:     new(int64(0)),
								SnapshotType:               "CRASH_CONSISTENT",
								LocalSnapshotRetentionPolicy: &SnapshotRetentionPolicy{
									NumSnapshots: new(int64(1)),
									RollupRetentionPolicy: &RollupRetentionPolicy{
										SnapshotIntervalType: "HOURLY",
										Multiple:             new(int64(1)),
									},
								},
								RemoteSnapshotRetentionPolicy: &SnapshotRetentionPolicy{
									NumSnapshots: new(int64(1)),
									RollupRetentionPolicy: &RollupRetentionPolicy{
										SnapshotIntervalType: "HOURLY",
										Multiple:             new(int64(1)),
									},
								},
							},
						},
					},
				},
				CategoryFilter: &CategoryFilter{
					Type:     new("CATEGORIES_MATCH_ALL"),
					KindList: []*string{new("1"), new("2")},
				},
			},
		},
	}

	expected := &ProtectionRuleResponse{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("protection_rule"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateProtectionRule", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.CreateProtectionRule(context.Background(), request)
		testOperationResult(t, "CreateProtectionRule", got, expected, err, false)
	})
}

func TestOperations_GetProtectionRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/protection_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"protection_rule","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	response := &ProtectionRuleResponse{}
	response.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("protection_rule"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := response

	t.Run("Test GetProtectionRules OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetProtectionRule(context.Background(), uuid)
		testOperationResult(t, "GetProtectionRules", got, expected, err, false)
	})
}

func TestOperations_ListProtectionRules(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/protection_rules/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"protection_rule","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	responseList := &ProtectionRulesListResponse{}
	responseList.Entities = make([]*ProtectionRuleResponse, 1)
	responseList.Entities[0] = &ProtectionRuleResponse{}
	responseList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("protection_rule"),
	}

	input := &DSMetadata{
		Length: new(int64(1)),
	}

	operationClient := client

	getEntitiesRequest := input

	expected := responseList

	t.Run("Test ListProtectionRules OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListProtectionRules(context.Background(), getEntitiesRequest)
		testOperationResult(t, "ListProtectionRules", got, expected, err, false)
	})
}

func TestOperations_UpdateProtectionRules(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/protection_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPut)

		expected := map[string]any{
			"api_version": "3.1",
			"metadata": map[string]any{
				"name": "protection_rule_test_name",
				"kind": "protection_rule",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc",
			},
			"spec": map[string]any{
				"resources": map[string]any{
					"start_time": "00h:00m",
					"ordered_availability_zone_list": []any{
						map[string]any{
							"availability_zone_url": "url test",
							"cluster_uuid":          "cfde831a-4e87-4a75-960f-89b0148aa2cc",
						},
					},
					"availability_zone_connectivity_list": []any{
						map[string]any{
							"destination_availability_zone_index": float64(0),
							"source_availability_zone_index":      float64(0),
							"snapshot_schedule_list": []any{
								map[string]any{
									"recovery_point_objective_secs": float64(0),
									"auto_suspend_timeout_secs":     float64(0),
									"snapshot_type":                 "CRASH_CONSISTENT",
									"local_snapshot_retention_policy": map[string]any{
										"num_snapshots": float64(1),
										"rollup_retention_policy": map[string]any{
											"snapshot_interval_type": "HOURLY",
											"multiple":               float64(1),
										},
									},
									"remote_snapshot_retention_policy": map[string]any{
										"num_snapshots": float64(1),
										"rollup_retention_policy": map[string]any{
											"snapshot_interval_type": "HOURLY",
											"multiple":               float64(1),
										},
									},
								},
							},
						},
					},
					"category_filter": map[string]any{
						"type":      "CATEGORIES_MATCH_ALL",
						"kind_list": []any{"1", "2"},
					},
				},
				"name":        "protection_rule_name",
				"description": "description_test",
			},
		}

		var actual map[string]any

		err := json.NewDecoder(request.Body).Decode(&actual)
		if err != nil {
			t.Errorf("decode json: %v", err)

			return
		}

		assertions := assert.New(t)
		if !assertions.Equal(actual, expected, "The response should be the same") {
			t.Errorf("Request body\n got=%#v\nwant=%#v", actual, expected)
		}

		_, _ = fmt.Fprintf(writer, `{
			"api_version": "3.1",
			"metadata": {
				"kind": "protection_rule",
				"uuid": "cfde831a-4e87-4a75-960f-89b0148aa2cc"
			}
		}`)
	})

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	body := &ProtectionRuleInput{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Name: new("protection_rule_test_name"),
			Kind: new("protection_rule"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
		Spec: &ProtectionRuleSpec{
			Name:        "protection_rule_name",
			Description: "description_test",
			Resources: &ProtectionRuleResources{
				StartTime: "00h:00m",
				OrderedAvailabilityZoneList: []*OrderedAvailabilityZoneList{
					{
						AvailabilityZoneURL: "url test",
						ClusterUUID:         "cfde831a-4e87-4a75-960f-89b0148aa2cc",
					},
				},
				AvailabilityZoneConnectivityList: []*AvailabilityZoneConnectivityList{
					{
						DestinationAvailabilityZoneIndex: new(int64(0)),
						SourceAvailabilityZoneIndex:      new(int64(0)),
						SnapshotScheduleList: []*SnapshotScheduleList{
							{
								RecoveryPointObjectiveSecs: new(int64(0)),
								AutoSuspendTimeoutSecs:     new(int64(0)),
								SnapshotType:               "CRASH_CONSISTENT",
								LocalSnapshotRetentionPolicy: &SnapshotRetentionPolicy{
									NumSnapshots: new(int64(1)),
									RollupRetentionPolicy: &RollupRetentionPolicy{
										SnapshotIntervalType: "HOURLY",
										Multiple:             new(int64(1)),
									},
								},
								RemoteSnapshotRetentionPolicy: &SnapshotRetentionPolicy{
									NumSnapshots: new(int64(1)),
									RollupRetentionPolicy: &RollupRetentionPolicy{
										SnapshotIntervalType: "HOURLY",
										Multiple:             new(int64(1)),
									},
								},
							},
						},
					},
				},
				CategoryFilter: &CategoryFilter{
					Type:     new("CATEGORIES_MATCH_ALL"),
					KindList: []*string{new("1"), new("2")},
				},
			},
		},
	}

	expected := &ProtectionRuleResponse{
		APIVersion: "3.1",
		Metadata: &Metadata{
			Kind: new("protection_rule"),
			UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		},
	}

	t.Run("Test CreateProtectionRule", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.UpdateProtectionRule(context.Background(), uuid, body)
		testOperationResult(t, "UpdateProtectionRules", got, expected, err, false)
	})
}

func TestOperations_DeleteProtectionRules(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/protection_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(_ http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteProtectionRules OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			true,
		},

		{
			"Test DeleteProtectionRules Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if _, err := op.DeleteProtectionRule(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteProtectionRules() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_TestProcessProtectionRule(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/protection_rules/cfde831a-4e87-4a75-960f-89b0148aa2cc/process",
		func(_ http.ResponseWriter, r *http.Request) {
			testHTTPMethod(t, r, http.MethodPost)
		},
	)

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test ProcessProtectionRule OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test ProcessProtectionRule Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if err := op.ProcessProtectionRule(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.ProcessProtectionRule() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_CreateRecoveryPlan(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	creds.Insecure = true

	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})
	recoveryPlan := &models.RecoveryPlanIntentInput{
		APIVersion: "3.1",
		Metadata: &models.RecoveryPlanMetadata{
			Kind: "recovery_plan",
		},
		Spec: &models.RecoveryPlan{
			Name: new("recovery_plan_test_name-4"),
			Resources: &models.RecoveryPlanResources{
				VolumeGroupRecoveryInfoList: []*models.RecoveryPlanVolumeGroupRecoveryInfo{
					{
						CategoryFilter: &models.CategoryFilter{
							KindList: []string{},
							Params: map[string][]string{
								"test-category": {"test-key"},
							},
						},
						VolumeGroupConfigInfoList: []*models.RecoveryPlanVolumeGroupRecoveryInfoVolumeGroupConfigInfoListItems0{},
					},
				},
				StageList: []*models.RecoveryPlanStage{},
				Parameters: &models.RecoveryPlanResourcesParameters{
					DataServiceIPMappingList: []*models.RecoveryPlanResourcesParametersDataServiceIPMappingListItems0{},
					FloatingIPAssignmentList: []*models.RecoveryPlanResourcesParametersFloatingIPAssignmentListItems0{},
					NetworkMappingList:       []*models.RecoveryPlanResourcesParametersNetworkMappingListItems0{},
					WitnessConfigurationList: []*models.WitnessConfiguration{
						{
							WitnessAddress:             "4b194a2d-5468-4e92-8953-571cb39be910",
							WitnessFailoverTimeoutSecs: 1,
						},
					},

					PrimaryLocationIndex: 0,
					AvailabilityZoneList: []*models.AvailabilityZoneInformation{
						{
							AvailabilityZoneURL: new("4b194a2d-5468-4e92-8953-571cb39be910"),
							ClusterReferenceList: []*models.ClusterReference{
								{
									Kind: "cluster",
									UUID: new("0006200e-fe43-236a-0000-00000000601e"),
								},
							},
						},
						{
							AvailabilityZoneURL: new("4b194a2d-5468-4e92-8953-571cb39be910"),
							ClusterReferenceList: []*models.ClusterReference{
								{
									Kind: "cluster",
									UUID: new("0006200e-f1f5-ef7f-0000-00000000601d"),
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := v3Client.V3.CreateRecoveryPlan(kctx, recoveryPlan)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	require.NotNil(t, resp.Status)
	require.NotNil(t, resp.Status.ExecutionContext)
	assert.Equal(t, "0fd3ad6f-7243-4e7d-967d-bfdfaf918f5b", resp.Status.ExecutionContext.TaskUUID)
	assert.Equal(t, "PENDING", resp.Status.State)
}

func TestOperations_GetRecoveryPlan(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	creds.Insecure = true

	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	resp, err := v3Client.V3.GetRecoveryPlan(kctx, "b2fe0a67-3be2-407c-8e83-9b5d6af14cec")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	assert.Equal(t, "recovery_plan_test_name-4", *resp.Spec.Name)
}

func TestOperations_ListRecoveryPlans(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	creds.Insecure = true

	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})
	_, err = v3Client.V3.ListRecoveryPlans(kctx, &DSMetadata{Length: new(int64(10))})
	assert.NoError(t, err)
}

func TestOperations_UpdateRecoveryPlans(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	creds.Insecure = true

	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})
	recoveryPlan := &models.RecoveryPlanIntentInput{
		APIVersion: "3.1",
		Metadata: &models.RecoveryPlanMetadata{
			Kind: "recovery_plan",
		},
		Spec: &models.RecoveryPlan{
			Name: new("recovery_plan_test_name-5"),
			Resources: &models.RecoveryPlanResources{
				VolumeGroupRecoveryInfoList: []*models.RecoveryPlanVolumeGroupRecoveryInfo{
					{
						CategoryFilter: &models.CategoryFilter{
							KindList: []string{},
							Params: map[string][]string{
								"test-category": {"test-key"},
							},
						},
						VolumeGroupConfigInfoList: []*models.RecoveryPlanVolumeGroupRecoveryInfoVolumeGroupConfigInfoListItems0{},
					},
				},
				StageList: []*models.RecoveryPlanStage{},
				Parameters: &models.RecoveryPlanResourcesParameters{
					DataServiceIPMappingList: []*models.RecoveryPlanResourcesParametersDataServiceIPMappingListItems0{},
					FloatingIPAssignmentList: []*models.RecoveryPlanResourcesParametersFloatingIPAssignmentListItems0{},
					NetworkMappingList:       []*models.RecoveryPlanResourcesParametersNetworkMappingListItems0{},
					WitnessConfigurationList: []*models.WitnessConfiguration{
						{
							WitnessAddress:             "4b194a2d-5468-4e92-8953-571cb39be910",
							WitnessFailoverTimeoutSecs: 1,
						},
					},

					PrimaryLocationIndex: 0,
					AvailabilityZoneList: []*models.AvailabilityZoneInformation{
						{
							AvailabilityZoneURL: new("4b194a2d-5468-4e92-8953-571cb39be910"),
							ClusterReferenceList: []*models.ClusterReference{
								{
									Kind: "cluster",
									UUID: new("0006200e-fe43-236a-0000-00000000601e"),
								},
							},
						},
						{
							AvailabilityZoneURL: new("4b194a2d-5468-4e92-8953-571cb39be910"),
							ClusterReferenceList: []*models.ClusterReference{
								{
									Kind: "cluster",
									UUID: new("0006200e-f1f5-ef7f-0000-00000000601d"),
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = v3Client.V3.UpdateRecoveryPlan(kctx, "b2fe0a67-3be2-407c-8e83-9b5d6af14cec", recoveryPlan)
	assert.NoError(t, err)
	assert.Equal(t, "recovery_plan_test_name-5", *recoveryPlan.Spec.Name)
}

func TestOperations_DeleteRecoveryPlan(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	creds.Insecure = true

	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})
	_, err = v3Client.V3.DeleteRecoveryPlan(kctx, "b2fe0a67-3be2-407c-8e83-9b5d6af14cec")
	assert.NoError(t, err)
}

func TestOperations_GetRecoveryPlanJob(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"recovery_plan_jobs","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}`)
	})

	response := &RecoveryPlanJobIntentResponse{}
	response.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("recovery_plan_jobs"),
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := response

	t.Run("Test GetRecoveryPlanJob OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetRecoveryPlanJob(context.Background(), uuid)
		testOperationResult(t, "GetRecoveryPlanJob", got, expected, err, false)
	})
}

func TestOperations_GetRecoveryPlanJobStatus(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	// status in the URL can have one of the 2 following values: execution_status, cleanup_status
	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/cfde831a-4e87-4a75-960f-89b0148aa2cc/execution_status", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"operation_status": {"percentage_complete":55,"status":"Running"}}`)
	})

	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/cfde831a-4e87-4a75-960f-89b0148aa2cc/cleanup_status", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"operation_status": {"percentage_complete":55,"status":"Running"},"preprocessing_status": {"percentage_complete":100,"status":"Complete"}}`)
	})

	executionStatusResponse := &RecoveryPlanJobExecutionStatus{
		OperationStatus: &RecoveryPlanJobPhaseExecutionStatus{
			PercentageComplete: 55,
			Status:             "Running",
		},
	}

	cleanupStatusResponse := &RecoveryPlanJobExecutionStatus{
		OperationStatus: &RecoveryPlanJobPhaseExecutionStatus{
			PercentageComplete: 55,
			Status:             "Running",
		},
		PreprocessingStatus: &RecoveryPlanJobPhaseExecutionStatus{
			PercentageComplete: 100,
			Status:             "Complete",
		},
	}

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID   string
		Status string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *RecoveryPlanJobExecutionStatus
		wantErr bool
	}{
		{
			"Test GetRecoveryPlanJobStatus ExecutionStatus OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc", "execution_status"},
			executionStatusResponse,
			false,
		},
		{
			"Test GetRecoveryPlanJobStatus CleanupStatus OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc", "cleanup_status"},
			cleanupStatusResponse,
			false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}

			got, err := op.GetRecoveryPlanJobStatus(context.Background(), testCase.args.UUID, testCase.args.Status)
			testOperationResult(t, "GetRecoveryPlanJobStatus", got, testCase.want, err, testCase.wantErr)
		})
	}
}

func TestOperations_DeleteRecoveryPlanJob(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(_ http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodDelete)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			"Test DeleteRecoveryPlanJobs OK",
			fields{client},
			args{"cfde831a-4e87-4a75-960f-89b0148aa2cc"},
			false,
		},

		{
			"Test DeleteRecoveryPlanJobs Errored",
			fields{client},
			args{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			if err := op.DeleteRecoveryPlanJob(context.Background(), testCase.args.UUID); (err != nil) != testCase.wantErr {
				t.Errorf("Operations.DeleteRecoveryPlanJob() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestOperations_PerformRecoveryPlanJobAction(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/cfde831a-4e87-4a75-960f-89b0148aa2cc/cleanup", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"}`)
	})

	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/cfde831a-4e87-4a75-960f-89b0148aa2cc/rerun", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"task_uuid": "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e"}`)
	})

	type fields struct {
		client *internal.Client
	}

	type args struct {
		UUID    string
		action  string
		request *RecoveryPlanJobActionRequest
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *RecoveryPlanJobResponse
		wantErr bool
	}{
		{
			"Test PerformRecoveryPlanJobAction Cleanup OK",
			fields{client},
			args{
				"cfde831a-4e87-4a75-960f-89b0148aa2cc",
				"cleanup",
				&RecoveryPlanJobActionRequest{
					ShouldContinueRerunOnValidationFailure: new(true),
				},
			},
			&RecoveryPlanJobResponse{
				TaskUUID: "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e",
			},
			false,
		},
		{
			"Test PerformRecoveryPlanJobAction Rerun OK",
			fields{client},
			args{
				"cfde831a-4e87-4a75-960f-89b0148aa2cc",
				"rerun",
				&RecoveryPlanJobActionRequest{
					ShouldContinueRerunOnValidationFailure: new(true),
				},
			},
			&RecoveryPlanJobResponse{
				TaskUUID: "ff1b9547-dc9a-4ebd-a2ff-f2b718af935e",
			},
			false,
		},
		{
			"Test PerformRecoveryPlanJobAction Unsupported Action Errored",
			fields{client},
			args{
				"cfde831a-4e87-4a75-960f-89b0148aa2cc",
				"jump",
				&RecoveryPlanJobActionRequest{
					ShouldContinueRerunOnValidationFailure: new(true),
				},
			},
			&RecoveryPlanJobResponse{},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			op := Operations{
				client: testCase.fields.client,
			}
			got, err := op.PerformRecoveryPlanJobAction(context.Background(),
				testCase.args.UUID, testCase.args.action, testCase.args.request)

			testOperationResult(t, "PerformRecoveryPlanJobAction", got, testCase.want, err, testCase.wantErr)
		})
	}
}

func TestOperations_ListRecoveryPlanJobs(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/recovery_plan_jobs/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"entities":[{"metadata": {"kind":"recovery_plan_jobs","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}}]}`)
	})

	responseList := &RecoveryPlanJobListResponse{}
	responseList.Entities = make([]*RecoveryPlanJobIntentResponse, 1)
	responseList.Entities[0] = &RecoveryPlanJobIntentResponse{}
	responseList.Entities[0].Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("recovery_plan_jobs"),
	}

	operationClient := client

	request := &DSMetadata{
		Length: new(int64(1)),
	}

	expected := responseList

	t.Run("Test ListRecoveryPlanJobs OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.ListRecoveryPlanJobs(context.Background(), request)
		testOperationResult(t, "ListRecoveryPlanJobs", got, expected, err, false)
	})
}

func TestOperations_GroupsGetEntities(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/groups", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{"filtered_group_count": 3, "group_results": [{"entity_results": [{"entity_id":"some_entity_id", "data":[{"values":[{"time":100, "values":["a", "b", "c"]}]}]}]}]}`)
	})

	operationClient := client

	request := &GroupsGetEntitiesRequest{
		EntityType: new("volume_group_config"),
		GroupMemberAttributes: []*GroupsRequestedAttribute{
			{
				Attribute: new("synchronous_replication_status"),
			},
			{
				Attribute: new("_master_cluster_uuid_"),
			},
		},
		FilterCriteria: "uuid=in=uuid1|uuid2",
	}

	expected := &GroupsGetEntitiesResponse{
		FilteredGroupCount: 3,
		GroupResults: []*GroupsGroupResult{
			{
				EntityResults: []*GroupsEntity{
					{
						EntityID: "some_entity_id",
						Data: []*GroupsFieldData{
							{
								Values: []*GroupsTimevaluePair{
									{
										Time:   100,
										Values: []string{"a", "b", "c"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	t.Run("Test GroupsGetEntities OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}
		got, err := op.GroupsGetEntities(context.Background(), request)

		testOperationResult(t, "GroupsGetEntities", got, expected, err, false)
	})
}

func TestOperations_GetAvailabilityZone(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/availability_zones/cfde831a-4e87-4a75-960f-89b0148aa2cc", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodGet)

		_, _ = fmt.Fprint(w, `{"metadata": {"kind":"availability_zone","uuid":"cfde831a-4e87-4a75-960f-89b0148aa2cc"}, "status":{"resources":{"management_url":"dfde831a-4e87-4a75-960f-89b0148aa2cc"}}}`)
	})

	response := &AvailabilityZoneIntentResponse{}
	response.Metadata = &Metadata{
		UUID: new("cfde831a-4e87-4a75-960f-89b0148aa2cc"),
		Kind: new("availability_zone"),
	}
	resources := &AvailabilityZoneResources{
		ManagementUrl: new("dfde831a-4e87-4a75-960f-89b0148aa2cc"),
	}
	response.Status = &AvailabilityZoneStatus{
		Resources: resources,
	}

	operationClient := client

	uuid := "cfde831a-4e87-4a75-960f-89b0148aa2cc"

	expected := response

	t.Run("Test GetAvailabilityZone OK", func(t *testing.T) {
		op := Operations{
			client: operationClient,
		}

		got, err := op.GetAvailabilityZone(context.Background(), uuid)
		testOperationResult(t, "GetAvailabilityZone", got, expected, err, false)
	})
}

func TestOperations_GetPrismCentral(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	// Changing insecure to true as that leads to modifying the transport underneath
	creds.Insecure = true

	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})
	pc, err := v3Client.V3.GetPrismCentral(kctx)
	require.NoError(t, err)
	assert.NotNil(t, pc)
	assert.Equal(t, "PC", *pc.Resources.Type)
	assert.Equal(t, "pc.2024.1.0.1", *pc.Resources.Version)
}

func TestOperations_ListSubnet(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	subnets, err := v3Client.V3.ListSubnet(kctx, &DSMetadata{})
	require.NoError(t, err)
	assert.Len(t, subnets.Entities, 3)

	for _, subnet := range subnets.Entities {
		assert.Equal(t, "subnet", *subnet.Metadata.Kind)
	}
}

func TestOperations_GetSubnet(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	subnet, err := v3Client.V3.GetSubnet(kctx, "c7938dc6-7659-453e-a688-e26020c68e43")
	require.NoError(t, err)
	assert.NotNil(t, subnet)
	assert.Equal(t, "subnet", *subnet.Metadata.Kind)
	assert.Equal(t, "sherlock_net", *subnet.Spec.Name)
	assert.Equal(t, false, subnet.Spec.Resources.IsExternal)
}

func TestOperations_GetCluster(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	cluster, err := v3Client.V3.GetCluster(kctx, "0005b0f1-8f43-a0f2-02b7-3cecef193712")
	require.NoError(t, err)
	assert.NotNil(t, cluster)
	assert.Equal(t, "cluster", *cluster.Metadata.Kind)
	assert.Equal(t, "ganon", cluster.Status.Name)
}

func TestOperations_ListCluster(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	clusters, err := v3Client.V3.ListCluster(kctx, &DSMetadata{})
	require.NoError(t, err)
	assert.Len(t, clusters.Entities, 2)
	assert.Equal(t, "cluster", *clusters.Entities[0].Metadata.Kind)
	assert.Equal(t, "cluster", *clusters.Entities[1].Metadata.Kind)
	assert.False(t, clusters.Entities[0].IsPrismCentral())
	assert.True(t, clusters.Entities[1].IsPrismCentral())

	prismElements := clusters.GetPrismElements()
	assert.Len(t, prismElements, 1)
	assert.Equal(t, "cluster", *prismElements[0].Metadata.Kind)
	assert.Equal(t, "ganon", prismElements[0].Status.Name)
}

func TestOperations_CreateIdempotenceIdentifers(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	resp, err := v3Client.V3.CreateIdempotenceIdentifiers(
		kctx,
		&models.IdempotenceIdentifiersInput{
			ClientIdentifier:       "testclient",
			Count:                  new(int64(2)),
			ValidDurationInMinutes: 10,
		},
	)
	require.NoError(t, err)
	assert.Equal(t, resp.ClientIdentifier, "testclient")
	assert.NotZero(t, resp.ExpirationTime)
	assert.Len(t, resp.UUIDList, 2)
}

func TestOperations_GetIdempotenceIdentifers(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	resp, err := v3Client.V3.GetIdempotenceIdentifiers(
		kctx,
		"testclient",
	)
	require.NoError(t, err)
	assert.Equal(t, resp.ClientIdentifier, "testclient")
	assert.Equal(t, *resp.Count, int64(2))
	assert.Len(t, resp.UUIDList, 2)
}

func TestOperations_DeleteIdempotenceIdentifers(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	err = v3Client.V3.DeleteIdempotenceIdentifiers(
		kctx,
		"testclient",
	)
	require.NoError(t, err)
}

func createVMIntentInput() *VMIntentInput {
	return &VMIntentInput{
		Spec: &VM{
			ClusterReference: &Reference{
				Kind: new("cluster"),
				UUID: new("00061f7f-44f7-19dc-3be1-7cc25586ee44"),
			},
			Name:        new("test-vm"),
			Description: new("test-vm-description"),
			Resources: &VMResources{
				NumThreads:    new(int64(1)),
				NumSockets:    new(int64(1)),
				MemorySizeMib: new(int64(1024)),
				NicList: []*VMNic{
					{
						SubnetReference: &Reference{
							UUID: new("b2e46975-2cde-4a49-9dda-815eb4fcd679"),
							Kind: new("subnet"),
						},
					},
				},
				BootConfig: &VMBootConfig{
					BootType: new("UEFI"),
				},
				DiskList: []*VMDisk{
					{
						DataSourceReference: &Reference{
							Kind: new("image"),
							UUID: new("70cfd137-a8d9-4169-a5ac-cf9ad6ddb03a"),
						},
						DiskSizeMib: new(int64(8192)),
					},
				},
			},
		},
		Metadata: &Metadata{
			Kind:        new("vm"),
			SpecVersion: new(int64(1)),
		},
	}
}

func TestOperations_CreateVM(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	vmIntent := createVMIntentInput()

	virtualMachine, err := v3Client.V3.CreateVM(kctx, vmIntent)
	require.NoError(t, err)
	require.NotNil(t, virtualMachine)
	require.NotNil(t, virtualMachine.Status)
	require.NotNil(t, virtualMachine.Status.ExecutionContext)
	assert.NotNil(t, virtualMachine.Status.ExecutionContext.TaskUUID)
	assert.IsType(t, "", virtualMachine.Status.ExecutionContext.TaskUUID)
}

func TestOperations_GetVM(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	vmIntent := createVMIntentInput()

	vm, err := v3Client.V3.GetVM(kctx, "bdbbad7a-a401-4f9a-a789-2b8406905b56")
	require.NoError(t, err)
	require.NotNil(t, vm)
	assert.Equal(t, "bdbbad7a-a401-4f9a-a789-2b8406905b56", *vm.Metadata.UUID)
	assert.Equal(t, *vmIntent.Spec.Name, *vm.Status.Name)
	assert.IsType(t, []any{}, vm.Status.ExecutionContext.TaskUUID)
}

func TestOperations_ListVM(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	vmIntent := createVMIntentInput()

	vms, err := v3Client.V3.ListVM(kctx, &DSMetadata{Filter: new("vm_name==test-vm"), SortAttribute: new("name"), SortOrder: new("ASCENDING")})
	require.NoError(t, err)
	require.NotNil(t, vms)
	require.NotEmpty(t, vms.Entities)
	assert.Equal(t, "bdbbad7a-a401-4f9a-a789-2b8406905b56", *vms.Entities[0].Metadata.UUID)
	assert.Equal(t, *vmIntent.Spec.Name, *vms.Entities[0].Status.Name)
}

func TestOperations_UpdateVM(t *testing.T) {
	creds := testhelpers.CredentialsFromEnvironment(t)
	interceptor := khttpclient.NewInterceptor(http.DefaultTransport)
	v3Client, err := NewV3Client(creds, WithRoundTripper(interceptor))
	require.NoError(t, err)

	kctx := mock.NewContext(mock.Config{
		Mode: keploy.MODE_TEST,
		Name: t.Name(),
	})

	vm, err := v3Client.V3.GetVM(kctx, "bdbbad7a-a401-4f9a-a789-2b8406905b56")
	require.NoError(t, err)

	vmIntent := createVMIntentInput()
	vmIntent.Spec = vm.Spec
	vmIntent.Spec.Description = new("updated-test-vm-description")

	uvm, err := v3Client.V3.UpdateVM(kctx, "bdbbad7a-a401-4f9a-a789-2b8406905b56", vmIntent)
	require.NoError(t, err)
	require.NotNil(t, uvm)
	require.NotNil(t, uvm.Status)
	require.NotNil(t, uvm.Status.ExecutionContext)
	require.NotNil(t, uvm.Status.ExecutionContext.TaskUUID)
	assert.IsType(t, "", uvm.Status.ExecutionContext.TaskUUID)
}

func TestOperations_ListAvailabilityZones(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/availability_zones/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{
			"api_version": "3.1",
			"metadata": {"kind": "availability_zone", "length": 2, "offset": 0, "total_matches": 2},
			"entities": [
				{"metadata": {"kind": "availability_zone", "uuid": "a5ee5f7f-c4eb-4ba6-8926-8cb7c5b81855"}, "status": {"name": "Local AZ", "resources": {"management_plane_type": "Local", "management_url": "70381d7a-5226-47ee-96b6-d1d450e790f5"}}},
				{"metadata": {"kind": "availability_zone", "uuid": "a1e195fb-a42d-463d-902f-cf8efb4f59e8"}, "status": {"name": "PC_10.33.19.169", "resources": {"management_plane_type": "PC", "management_url": "a1e195fb-a42d-463d-902f-cf8efb4f59e8"}}}
			]
		}`)
	})

	op := Operations{client: client}
	got, err := op.ListAvailabilityZones(context.Background(), &DSMetadata{Kind: new("availability_zone")})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.Metadata)
	assert.Equal(t, int64(2), ptr.Deref(got.Metadata.TotalMatches, 0))
	require.Len(t, got.Entities, 2)
	assert.Equal(t, "Local AZ", ptr.Deref(got.Entities[0].Status.Name, ""))
	assert.Equal(t, "a1e195fb-a42d-463d-902f-cf8efb4f59e8", ptr.Deref(got.Entities[1].Metadata.UUID, ""))
}

func TestOperations_ListAllAvailabilityZones(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	// Serve two pages so pagination is exercised: total_matches=150, with
	// itemsPerPage (100) on the first page and 50 on the second.
	mux.HandleFunc("/api/nutanix/v3/availability_zones/list", func(writer http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		var body DSMetadata
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			return
		}
		offset := ptr.Deref(body.Offset, 0)

		count := int64(100)
		if offset >= 100 {
			count = 50
		}

		entities := make([]string, 0, count)
		for i := int64(0); i < count; i++ {
			entities = append(entities, fmt.Sprintf(
				`{"metadata": {"kind": "availability_zone", "uuid": "uuid-%d-%d"}}`, offset, i))
		}

		_, _ = fmt.Fprintf(writer, `{"api_version": "3.1", "metadata": {"kind": "availability_zone", "length": %d, "offset": %d, "total_matches": 150}, "entities": [%s]}`,
			count, offset, strings.Join(entities, ","))
	})

	op := Operations{client: client}
	got, err := op.ListAllAvailabilityZones(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got.Entities, 150)
}

func TestOperations_ListAllAvailabilityZonesSinglePage(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/availability_zones/list", func(w http.ResponseWriter, r *http.Request) {
		testHTTPMethod(t, r, http.MethodPost)

		_, _ = fmt.Fprint(w, `{
			"api_version": "3.1",
			"metadata": {"kind": "availability_zone", "length": 1, "offset": 0, "total_matches": 1},
			"entities": [
				{"metadata": {"kind": "availability_zone", "uuid": "a5ee5f7f-c4eb-4ba6-8926-8cb7c5b81855"}, "status": {"name": "Local AZ"}}
			]
		}`)
	})

	op := Operations{client: client}
	got, err := op.ListAllAvailabilityZones(context.Background(), "")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Entities, 1)
	assert.Equal(t, "Local AZ", ptr.Deref(got.Entities[0].Status.Name, ""))
}

func TestOperations_GetSyncReplicationCapableClusters(t *testing.T) {
	mux, client, server := setup(t)

	defer server.Close()

	mux.HandleFunc("/api/nutanix/v3/clusters/synchronous_replication_capable", func(writer http.ResponseWriter, request *http.Request) {
		testHTTPMethod(t, request, http.MethodPost)

		expected := map[string]any{
			"remote_availability_zone_reference": map[string]any{
				"kind": "availability_zone",
				"uuid": "a5ee5f7f-c4eb-4ba6-8926-8cb7c5b81855",
			},
			"remote_cluster_reference": map[string]any{
				"kind": "cluster",
				"uuid": "00064ca8-05e8-56c6-2f37-7cc2558d16f0",
			},
		}

		var v map[string]any
		if !assert.NoError(t, json.NewDecoder(request.Body).Decode(&v)) {
			return
		}

		if !reflect.DeepEqual(v, expected) {
			t.Errorf("Request body\n got=%#v\nwant=%#v", v, expected)
		}

		// Note: rtt_msecs is returned by PC as a JSON number, not a string.
		_, _ = fmt.Fprint(writer, `[{"rtt_msecs": 0.37, "cluster_reference": {"kind": "cluster", "name": "pe-1", "uuid": "00064ca8-0573-8976-1db0-7cc2558d1648"}}]`)
	})

	op := Operations{client: client}
	got, err := op.GetSyncReplicationCapableClusters(context.Background(), &ClusterSyncReplicationCapableInput{
		RemoteClusterReference: &Reference{
			Kind: new("cluster"),
			UUID: new("00064ca8-05e8-56c6-2f37-7cc2558d16f0"),
		},
		RemoteAvailabilityZoneReference: &Reference{
			Kind: new("availability_zone"),
			UUID: new("a5ee5f7f-c4eb-4ba6-8926-8cb7c5b81855"),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, *got, 1)

	entry := (*got)[0]
	require.NotNil(t, entry.RttMsecs)
	assert.Equal(t, "0.37", entry.RttMsecs.String())
	require.NotNil(t, entry.ClusterReference)
	assert.Equal(t, "00064ca8-0573-8976-1db0-7cc2558d1648", ptr.Deref(entry.ClusterReference.UUID, ""))
}

func testOperationResult(t *testing.T, operation string, got, want any, err error, wantErr bool) {
	t.Helper()

	if (err != nil) != wantErr {
		t.Errorf("Operations.%s() error = %v, wantErr %v", operation, err, wantErr)
		return
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Operations.%s() = %v, want %v", operation, got, want)
	}
}
