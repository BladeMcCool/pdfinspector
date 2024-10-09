package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
)

// todo maybe this could include a flag about if it was an error so that we can detect that at the server and refund them?
type JobStatus struct {
	Message string `json:"message"`
	Error   *bool  `json:"error,omitempty"`
}
type ExtractResult struct {
	JobStatus
	TemplateName *string `json:"template_name,omitempty"`
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
	Id             string
	OverrideJobId  *string `json:"job_id,omitempty"` //generally speaking this can't be set by a user, is just for admin/testing
	Layout         string  `json:"layout""`

	MainPrompt string
	//ExpectResponseSchema interface{} //will get a json schema based on the layout.

	OutputDir       string
	AcceptableRatio float64
	MaxAttempts     int
	IsForAdmin      bool

	//anything else we want as options per-job? i was thinking include_bio might be a good option. (todo: ability to not show it on functional, ability to show it on chrono, and then json schema tuning depending on if it is set or not so that the gpt can know to specify it - and dont include it when it shouldn't!)
	//idk but i want to report to the user their balance and i dont really want to make a whole new struct for it
	UserKey             string
	UserCreditRemaining int
	UserID              string //like sso subject id, so we can put generation ids into a bucket path for them to recall later.

	//
	Logger *zerolog.Logger
}

var defaultAcceptableRatio = 0.88

var defaultMaxAttempts = 7 //this should probably come from env
//var defaultMaxAttempts = 1 //this should probably come from env

func NewDefaultJob() *Job {
	job := &Job{}
	job.PrepareDefault(nil)
	return job
}
func (job *Job) PrepareDefault(jobId *string) {
	if jobId == nil || *jobId == "" {
		job.Id = uuid.New().String()
	} else {
		job.Id = *jobId
	}
	job.AcceptableRatio = defaultAcceptableRatio
	job.MaxAttempts = defaultMaxAttempts
	job.Logger = getLogger(job.Id)
}

//	func (job *Job) OverrideJobId(jobId string) {
//		job.Id = jobId
//		job.Logger = job.setLogger()
//	}
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

	// Check if the layout value is either "functional" or "chrono"
	if job.Layout == "functional" || job.Layout == "chrono" {
		log.Info().Msgf("The layout is: %s", job.Layout)
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
		log.Info().Msgf("Got API Key from input text file")
	} else {
		log.Info().Msgf("Got API Key from env var")
	}

	// Return the populated Input struct
	return &Input{
		InputDir: dir,
		JD:       string(jdContent),
		APIKey:   apiKey,
	}, nil
}

func (job *Job) Log() *zerolog.Logger {
	return job.Logger
}

type RenderJob struct {
	BaselineJSON  string `json:"baseline_json"`  //the actual layout to use is a property of the baseline resumedata.
	StyleOverride string `json:"style_override"` //eg fluffy
	Id            string
	Layout        string `json:"layout""`

	OutputDir string
	UserKey   string
	UserID    string //like sso subject id, so we can put generation ids into a bucket path for them to recall later.

	Logger *zerolog.Logger
}

func (job *RenderJob) Log() *zerolog.Logger {
	return job.Logger
}

func (job *RenderJob) PrepareDefault(jobId *string, ctx context.Context) {
	job.Id = uuid.New().String()
	job.Logger = getLogger(job.Id)

	userID, _ := ctx.Value("ssoSubject").(string)
	job.UserID = userID
	job.Log().Trace().Msgf("PrepareDefault: sso subject userId believed to be %s", userID)
}
func getLogger(jobId string) *zerolog.Logger {
	logger := log.With().
		Str("job_id", jobId).
		Logger()
	return &logger
}
