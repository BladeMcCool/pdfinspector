package job

import (
	"fmt"
	"github.com/google/uuid"
	"log"
	"os"
	"path/filepath"
)

// JobResult represents a job result with its status and result data.
type JobResult struct {
	ID     int
	Status string
	Result string
}

type JobStatus struct {
	Message string `json:"message"`
}

// Job represents the structure for a job
type Job struct {
	JobDescription string `json:"jd"`
	Baseline       string `json:"baseline"`      //the actual layout to use is a property of the baseline resumedata.
	BaselineJSON   string `json:"baseline_json"` //the actual layout to use is a property of the baseline resumedata.
	CustomPrompt   string `json:"prompt"`
	StyleOverride  string `json:"style_override"` //eg fluffy
	Id             uuid.UUID

	AcceptableRatio float64
	MaxAttempts     int
	//acceptableRatio = 0.88
	//maxAttempts     = 1
	//anything else we want as options per-job? i was thinking include_bio might be a good option. (todo: ability to not show it on functional, ability to show it on chrono, and then json schema tuning depending on if it is set or not so that the gpt can know to specify it - and dont include it when it shouldn't!)
}

var defaultAcceptableRatio = 0.88
var defaultMaxAttempts = 1

//	func newJob(acceptableRatio float64, maxAttempts int) *Job {
//		job := &Job{}
//		if acceptableRatio != 0 {
//			job.AcceptableRatio = acceptableRatio
//		} else {
//			job.AcceptableRatio = defaultAcceptableRatio
//		}
//
//		if maxAttempts != 0 {
//			job.MaxAttempts = defaultMaxAttempts
//		} else {
//			job.MaxAttempts = defaultMaxAttempts
//		}
//		return job
//	}

func NewDefaultJob() *Job {
	return &Job{
		Id:              uuid.New(),
		AcceptableRatio: defaultAcceptableRatio,
		MaxAttempts:     defaultMaxAttempts,
	}
}
func (job *Job) PrepareDefault() {
	job.Id = uuid.New()
	job.AcceptableRatio = defaultAcceptableRatio
	job.MaxAttempts = defaultMaxAttempts
}

// Input struct to hold the contents of jd.txt and expect_response.json
type Input struct {
	InputDir             string
	JD                   string
	ExpectResponseSchema interface{}
	APIKey               string
}

// ReadInput reads the input files from the "input" directory and returns an Input struct
func ReadInput(dir string) (*Input, error) {
	// Define file paths
	jdFilePath := filepath.Join(dir, "jd.txt")
	apiKeyFilePath := filepath.Join(dir, "api_key.txt")

	// Read jd.txt
	jdContent, err := os.ReadFile(jdFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read jd.txt: %v", err)
	}

	// Retrieve the API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// If the environment variable is not set, try to read it from api_key.txt
		apiKeyContent, err := os.ReadFile(apiKeyFilePath)
		if err != nil {
			return nil, fmt.Errorf("API key not found in environment variable or api_key.txt: %v", err)
		}
		apiKey = string(apiKeyContent)
		log.Println("Got API Key from input text file")
	} else {
		log.Println("Got API Key from env var")
	}

	// Return the populated Input struct
	return &Input{
		InputDir: dir,
		JD:       string(jdContent),
		//ExpectResponse: string(expectResponseContent),
		//ExpectResponseSchema: expectResponseSchema,
		APIKey: apiKey,
	}, nil
}
