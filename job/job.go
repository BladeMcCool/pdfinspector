package job

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"os"
	"path/filepath"
	//"pdfinspector/tuner"
	//"pdfinspector/tuner"
	//"pdfinspector/tuner"
)

//type JobResult struct {
//	ID     int
//	Status string
//	Result string
//}

// todo maybe this could include a flag about if it was an error so that we can detect that at the server and refund them?
type JobStatus struct {
	Message string `json:"message"`
}

type JobResult struct {
	Status  string `json:"status"`
	Details string `json:"details"`
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
	IsForAdmin      bool
	//acceptableRatio = 0.88
	//maxAttempts     = 1
	//anything else we want as options per-job? i was thinking include_bio might be a good option. (todo: ability to not show it on functional, ability to show it on chrono, and then json schema tuning depending on if it is set or not so that the gpt can know to specify it - and dont include it when it shouldn't!)
}

var defaultAcceptableRatio = 0.88
var defaultMaxAttempts = 7

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
func (job *Job) ValidateForNonAdmin() error {
	//this is just more of a thought than perhaps a good idea. the failure modes can be many and we should just return api credits if job failed. todo.
	if job.Baseline != "" {
		return errors.New("disallowed")
	}
	if job.BaselineJSON == "" {
		return errors.New("disallowed")
	}

	var result interface{}
	if err := json.Unmarshal([]byte(job.BaselineJSON), &result); err != nil {
		return fmt.Errorf("error decoding JSON into interface{}: %v", err)
	}

	// Type assertion: result is expected to be a map[string]interface{}
	data, ok := result.(map[string]interface{})
	if !ok {
		return errors.New("Result is not a valid map")
	}

	// Extract the "layout" field from the map
	layoutValue, ok := data["layout"].(string)
	if !ok {
		return errors.New("Layout key not found or is not a string")
	}

	// Check if the layout value is either "functional" or "chrono"
	if layoutValue == "functional" || layoutValue == "chrono" {
		fmt.Printf("The layout is: %s\n", layoutValue)
	} else {
		return errors.New("The layout is neither 'functional' nor 'chrono'")
	}
	// List of valid styles (expandable in the future)
	validStyles := map[string]bool{
		"fluffy": true, // Starting with only "fluffy"
	}

	// Check if the "style" field exists and is valid
	if styleValue, ok := data["style"]; ok {
		styleStr, ok := styleValue.(string)
		if !ok {
			return errors.New("Style key is present but not a valid string")
		}
		if !validStyles[styleStr] {
			return fmt.Errorf("Invalid style: %s", styleStr)
		}
	}
	return nil
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
