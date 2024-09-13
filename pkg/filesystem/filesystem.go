package filesystem

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileSystem interface defines methods to interact with different types of file systems.
type FileSystem interface {
	WriteFile(filename string, data []byte) error
	Writer(filename string) (io.Writer, error)
	ReadFile(filename string) ([]byte, error)
}

// LocalFileSystem implements FileSystem interface for local file operations.
type LocalFileSystem struct {
	BasePath string
}

// WriteFile writes the job result to a local file.
func (lfs *LocalFileSystem) WriteFile(filename string, data []byte) error {
	filePath := filepath.Join(lfs.BasePath, filename)
	return os.WriteFile(filePath, data, 0644)
}

// Writer for LocalFilesystem
func (lfs *LocalFileSystem) Writer(filename string) (io.Writer, error) {
	// Open the file for writing (create it if it doesn't exist)
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return file, nil
}
func (lfs *LocalFileSystem) ReadFile(filename string) ([]byte, error) {
	filePath := filepath.Join(lfs.BasePath, filename)
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return fileData, nil
}

// GCSFileSystem implements FileSystem interface for Google Cloud Storage.
type GCSFileSystem struct {
	Client     *storage.Client
	BucketName string
}

//func New

// WriteFile writes the job result to a GCS object.
func (gcs *GCSFileSystem) WriteFile(filename string, data []byte) error {
	ctx := context.Background()
	wc := gcs.Client.Bucket(gcs.BucketName).Object(filename).NewWriter(ctx)

	_, err := wc.Write(data)

	// Close the writer and return the error if any
	if err = wc.Close(); err != nil {
		return fmt.Errorf("failed to finalize write operation to GCS: %v", err)
	}

	return nil
}

// Writer for GCSFilesystem
func (gcs *GCSFileSystem) Writer(filename string) (io.Writer, error) {
	// Create a GCS object writer
	ctx := context.Background()
	bucket := gcs.Client.Bucket(gcs.BucketName)
	writer := bucket.Object(filename).NewWriter(ctx)
	return writer, nil
}

func (gcs *GCSFileSystem) ReadFile(filename string) ([]byte, error) {
	ctx := context.Background()
	rc, err := gcs.Client.Bucket(gcs.BucketName).Object(filename).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	fileData, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return fileData, nil
}
