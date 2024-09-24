package server

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"io"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/jobrunner"
	"pdfinspector/pkg/tuner"
	"strings"
	"testing"
)

// MockFileSystem is a mock implementation of the FileSystem interface.
type MockFileSystem struct {
	files map[string][]byte
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files: make(map[string][]byte),
	}
}

func (mfs *MockFileSystem) WriteFile(filename string, data []byte) error {
	mfs.files[filename] = data
	return nil
}

func (mfs *MockFileSystem) Writer(filename string) (io.Writer, error) {
	// For simplicity, we'll just return a buffer that writes to our map
	buf := &bytes.Buffer{}
	return &mockWriter{
		buf:      buf,
		filename: filename,
		fs:       mfs,
	}, nil
}

func (mfs *MockFileSystem) ReadFile(ctx context.Context, filename string) ([]byte, error) {
	data, ok := mfs.files[filename]
	if !ok {
		// Simulate GCS storage.ErrObjectNotExist error
		return nil, storage.ErrObjectNotExist
	}
	return data, nil
}

// mockWriter is a helper to implement Writer method
type mockWriter struct {
	buf      *bytes.Buffer
	filename string
	fs       *MockFileSystem
}

func (mw *mockWriter) Write(p []byte) (n int, err error) {
	return mw.buf.Write(p)
}

func (mw *mockWriter) Close() error {
	// Save the data to the mock filesystem when closed
	mw.fs.files[mw.filename] = mw.buf.Bytes()
	return nil
}

func TestGetBestApiKeyForUser(t *testing.T) {
	ctx := context.Background()

	// Create a mock filesystem
	mfs := NewMockFileSystem()

	// Set up the pdfInspectorServer with the mock filesystem
	server := &pdfInspectorServer{
		jobRunner: &jobrunner.JobRunner{
			Tuner: &tuner.Tuner{
				Fs: mfs,
			},
		},
		config: &config.ServiceConfig{},
	}

	// Define test cases
	testCases := []struct {
		name           string
		userID         string
		setupFiles     map[string]string // filename to content
		expectedApiKey string
		expectedError  string
	}{
		// Your specified test case
		{
			name:   "Multiple API keys with positive credits (Specific Case)",
			userID: "userSpecific",
			setupFiles: map[string]string{
				"sso/userSpecific/apikeys": "apiKey1\napiKey2\napiKey3\napiKey4\napiKey5",
				"users/apiKey1/credit":     "5",
				"users/apiKey2/credit":     "10",
				"users/apiKey3/credit":     "3",
				"users/apiKey4/credit":     "0",
				"users/apiKey5/credit":     "-2",
			},
			expectedApiKey: "apiKey3", // Least positive credits
			expectedError:  "",
		},
		{
			name:   "Multiple API keys with positive credits",
			userID: "user1",
			setupFiles: map[string]string{
				"sso/user1/apikeys":    "apiKey1\napiKey2\napiKey3",
				"users/apiKey1/credit": "5",
				"users/apiKey2/credit": "10",
				"users/apiKey3/credit": "3",
			},
			expectedApiKey: "apiKey3", // Least positive credits
			expectedError:  "",
		},
		{
			name:   "API keys with zero or negative credits",
			userID: "user2",
			setupFiles: map[string]string{
				"sso/user2/apikeys":    "apiKey4\napiKey5",
				"users/apiKey4/credit": "0",
				"users/apiKey5/credit": "-2",
			},
			expectedApiKey: "",
			expectedError:  "no API keys with positive credits found for user user2",
		},
		{
			name:   "API keys without credit files",
			userID: "user3",
			setupFiles: map[string]string{
				"sso/user3/apikeys": "apiKey6\napiKey7",
				// No credit files for apiKey6 and apiKey7
			},
			expectedApiKey: "",
			expectedError:  "no API keys with positive credits found for user user3",
		},
		{
			name:   "Single API key with positive credits",
			userID: "user4",
			setupFiles: map[string]string{
				"sso/user4/apikeys":    "apiKey8",
				"users/apiKey8/credit": "7",
			},
			expectedApiKey: "apiKey8",
			expectedError:  "",
		},
		{
			name:   "No API keys",
			userID: "user5",
			setupFiles: map[string]string{
				"sso/user5/apikeys": "", // Empty file
			},
			expectedApiKey: "",
			expectedError:  "no API keys found for user user5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mock filesystem
			mfs.files = make(map[string][]byte)

			// Set up files in the mock filesystem
			for filename, content := range tc.setupFiles {
				mfs.WriteFile(filename, []byte(content))
			}

			// Call the function under test
			apiKey, err := server.GetBestApiKeyForUser(ctx, tc.userID)

			// Check for expected error
			if tc.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error containing '%v', got '%v'", tc.expectedError, err)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check expected API key
			if apiKey != tc.expectedApiKey {
				t.Errorf("Expected API key '%s', got '%s'", tc.expectedApiKey, apiKey)
			}
		})
	}
}
