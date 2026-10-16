package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const uploadFixture = "prefix-image-bytes"

func uploadFile(t *testing.T, offset int64) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "upload")
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	_, err = file.WriteString(uploadFixture)
	require.NoError(t, err)
	_, err = file.Seek(offset, io.SeekStart)
	require.NoError(t, err)
	return file
}

func TestUploadReplayAndOwnership(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		for _, offset := range []int64{0, 7, int64(len(uploadFixture)), int64(len(uploadFixture)) + 1} {
			t.Run(fmt.Sprintf("auth=%t/offset=%d", authenticated, offset), func(t *testing.T) {
				t.Parallel()
				_, client, server := setup(t)
				t.Cleanup(server.Close)
				file := uploadFile(t, offset)
				builder := client.NewUnAuthUploadRequest
				if authenticated {
					builder = client.NewUploadRequest
				}
				req, err := builder(http.MethodPut, "/upload", file)
				require.NoError(t, err)
				want := uploadFixture[min(offset, int64(len(uploadFixture))):]
				require.Equal(t, int64(len(want)), req.ContentLength)
				require.Equal(t, authenticated, req.Header.Get("Authorization") != "")
				readUploadBody(t, req.Body, want)
				first, err := req.GetBody()
				require.NoError(t, err)
				second, err := req.GetBody()
				require.NoError(t, err)
				// Advancing one body must not advance another replay or the caller's file.
				prefix := make([]byte, min(1, len(want)))
				_, err = io.ReadFull(first, prefix)
				require.NoError(t, err)
				readUploadBody(t, second, want)
				readUploadBody(t, first, want[len(prefix):])
				position, err := file.Seek(0, io.SeekCurrent)
				require.NoError(t, err)
				require.Equal(t, offset, position)
				if want == "" {
					require.Equal(t, http.NoBody, req.Body)
				}
			})
		}
	}
}

func readUploadBody(t *testing.T, body io.ReadCloser, want string) {
	t.Helper()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, want, string(data))
	require.NoError(t, body.Close())
}

func TestUploadRedirect(t *testing.T) {
	t.Parallel()
	mux, client, server := setup(t)
	t.Cleanup(server.Close)
	file := uploadFile(t, 0)
	received := make(chan string, 2)
	mux.HandleFunc("/"+testAbsolutePath+"/upload", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		received <- string(data)
		w.Header().Set("Location", "/"+testAbsolutePath+"/target")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/"+testAbsolutePath+"/target", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		received <- string(data)
		w.WriteHeader(http.StatusOK)
	})
	req, err := client.NewUploadRequest(http.MethodPut, "/upload", file)
	require.NoError(t, err)
	require.NoError(t, client.Do(context.Background(), req, nil))
	require.Len(t, received, 2)
	require.Equal(t, uploadFixture, <-received)
	require.Equal(t, uploadFixture, <-received)
	_, err = file.Stat()
	require.NoError(t, err, "HTTP transport must not close the caller's file")
}

func TestUploadNonRegularFiles(t *testing.T) {
	t.Parallel()
	_, client, server := setup(t)
	t.Cleanup(server.Close)
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	for _, builder := range []func(string, string, *os.File) (*http.Request, error){
		client.NewUploadRequest, client.NewUnAuthUploadRequest,
	} {
		req, err := builder(http.MethodPut, "/upload", reader)
		require.Error(t, err)
		require.Nil(t, req)
	}
}
