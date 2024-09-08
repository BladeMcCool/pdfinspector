package filesystem

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

//// JobResult represents a job result with its status and result data.
//type JobResult struct {
//	ID     int
//	Status string
//	Result string
//}

// FileSystem interface defines methods to interact with different types of file systems.
type FileSystem interface {
	WriteFile(filename string, data []byte) error
	//ReadFile(jobID int) (JobResult, error)
}

// LocalFileSystem implements FileSystem interface for local file operations.
type LocalFileSystem struct {
	BasePath string
}

// WriteFile writes the job result to a local file.
func (lfs *LocalFileSystem) WriteFile(filename string, data []byte) error {
	filePath := filepath.Join(lfs.BasePath, filename)
	//fileData, err := json.Marshal(data)
	//if err != nil {
	//	return err
	//}
	return os.WriteFile(filePath, data, 0644)
}

//// ReadFile reads the job result from a local file.
//func (lfs *LocalFileSystem) ReadFile(jobID int) (JobResult, error) {
//	filePath := filepath.Join(lfs.BasePath, fmt.Sprintf("job_%d.json", jobID))
//	fileData, err := ioutil.ReadFile(filePath)
//	if err != nil {
//		return JobResult{}, err
//	}
//	var jobResult JobResult
//	if err := json.Unmarshal(fileData, &jobResult); err != nil {
//		return JobResult{}, err
//	}
//	return jobResult, nil
//}

// GCSFileSystem implements FileSystem interface for Google Cloud Storage.
type GCSFileSystem struct {
	Client     *storage.Client
	BucketName string
}

//func New

// WriteFile writes the job result to a GCS object.
func (gcs *GCSFileSystem) WriteFile(filename string, data []byte) error {
	ctx := context.Background()
	//fileData, err := json.Marshal(data)
	//if err != nil {
	//	return err
	//}
	//objectPath := fmt.Sprintf("job_%d.json", jobID)
	//objectPath := filename
	//wc := gcs.Client.Bucket(gcs.BucketName).Object(objectPath).NewWriter(ctx)
	wc := gcs.Client.Bucket(gcs.BucketName).Object(filename).NewWriter(ctx)

	_, err := wc.Write(data)

	// Close the writer and return the error if any
	if err = wc.Close(); err != nil {
		return fmt.Errorf("failed to finalize write operation to GCS: %v", err)
	}

	return nil
}

//// ReadFile reads the job result from a GCS object.
//func (gcs *GCSFileSystem) ReadFile(jobID int) (JobResult, error) {
//	ctx := context.Background()
//	objectPath := fmt.Sprintf("job_%d.json", jobID)
//	rc, err := gcs.Client.Bucket(gcs.BucketName).Object(objectPath).NewReader(ctx)
//	if err != nil {
//		return JobResult{}, err
//	}
//	defer rc.Close()
//
//	fileData, err := ioutil.ReadAll(rc)
//	if err != nil {
//		return JobResult{}, err
//	}
//
//	var jobResult JobResult
//	if err := json.Unmarshal(fileData, &jobResult); err != nil {
//		return JobResult{}, err
//	}
//	return jobResult, nil
//}
