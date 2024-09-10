package main

import (
	//"github.com/google/uuid"
	"github.com/google/uuid"
	"log"
)

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
func newDefaultJob() *Job {
	return &Job{
		Id:              uuid.New(),
		AcceptableRatio: defaultAcceptableRatio,
		MaxAttempts:     defaultMaxAttempts,
	}
}
func (job *Job) prepareDefault() {
	job.Id = uuid.New()
	job.AcceptableRatio = defaultAcceptableRatio
	job.MaxAttempts = defaultMaxAttempts
}

// JobResult represents a job result with its status and result data.
type JobResult struct {
	ID     int
	Status string
	Result string
}

type jobRunner struct {
	config *serviceConfig
	tuner  *tuner
}

func (j *jobRunner) RunJob(job *Job, updates chan JobStatus) {
	if updates != nil {
		defer close(updates)
	}
	//mu.Lock()
	//jobID := jobIDCounter
	//jobIDCounter++
	//mu.Unlock()
	log.Printf("do something with this job: %#v", job)

	t := j.tuner

	baselineJSON := job.BaselineJSON //i think really this is where it should come from
	var err error
	if baselineJSON == "" {
		//but for some testing (and the initial implementation, predating the json server even, where my personal info was baked into the react project ... anyway, that got moved to the json server and some variants got names. but that should all get deprecated i think)
		baselineJSON, err = t.getBaselineJSON(job.Baseline)
		if err != nil {
			log.Fatalf("error from reading baseline JSON: %v", err)
		}
	}
	sendJobUpdate(updates, "got baseline JSON")

	layout, style, err := t.getLayoutFromBaselineJSON(baselineJSON)
	if err != nil {
		log.Fatalf("error from extracting layout from baseline JSON: %v", err)
	}
	if job.StyleOverride != "" {
		style = job.StyleOverride
	}

	expectResponseSchema, err := t.getExpectedResponseJsonSchema(layout)
	//todo refactor this stuff lol
	inputTemp := &Input{
		JD:                   job.JobDescription,
		ExpectResponseSchema: expectResponseSchema,
		APIKey:               j.config.openAiApiKey,
	}
	if err != nil {
		log.Fatalf("Error reading input files: %v", err)
	}

	mainPrompt := job.CustomPrompt
	if mainPrompt == "" {
		mainPrompt, err = t.getDefaultPrompt(layout)
		if err != nil {
			log.Println("error from reading input prompt: ", err)
			return
		}
	}

	//todo: fix this calls arguments it should probably just be one struct.
	err = t.tuneResumeContents(inputTemp, mainPrompt, baselineJSON, layout, style, job.Id.String(), t.fs, j.config, job, updates)
	if err != nil {
		log.Fatalf("Error from resume tuning: %v", err)
	}

	// Simulate job processing
	//time.Sleep(5 * time.Second)
	//result := fmt.Sprintf("Processed job with field1: %s, field2: %s, field3: %s", job.Field1, job.Field2, job.Field3)

	//jobResult := JobResult{ID: jobID, Status: "Completed", Result: result}

	// Store job result in filesystem
	//if err := fs.WriteFile(jobID, jobResult); err != nil {
	//	log.Printf("Error writing job result: %v", err)
	//}
}

func (j *jobRunner) RunJobStreaming(job *Job) chan JobStatus {
	//job.Id = uuid.New()
	log.Printf("running job %s", job.Id.String())

	updates := make(chan JobStatus)
	go j.RunJob(job, updates)
	//j.RunJob(job, nil)

	//// Add job to queue
	return updates
}
