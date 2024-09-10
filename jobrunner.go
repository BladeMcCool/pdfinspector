package main

import (
	"log"
	config2 "pdfinspector/config"
	jobPackage "pdfinspector/job"
	"pdfinspector/tuner"
)

type jobRunner struct {
	config *config2.ServiceConfig
	tuner  *tuner.Tuner
}

func (j *jobRunner) RunJob(job *jobPackage.Job, updates chan jobPackage.JobStatus) {
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
		baselineJSON, err = t.GetBaselineJSON(job.Baseline)
		if err != nil {
			log.Fatalf("error from reading baseline JSON: %v", err)
		}
	}
	tuner.SendJobUpdate(updates, "got baseline JSON")

	layout, style, err := t.GetLayoutFromBaselineJSON(baselineJSON)
	if err != nil {
		log.Fatalf("error from extracting layout from baseline JSON: %v", err)
	}
	if job.StyleOverride != "" {
		style = job.StyleOverride
	}

	expectResponseSchema, err := t.GetExpectedResponseJsonSchema(layout)
	//todo refactor this stuff lol
	inputTemp := &jobPackage.Input{
		JD:                   job.JobDescription,
		ExpectResponseSchema: expectResponseSchema,
		APIKey:               j.config.OpenAiApiKey,
	}
	if err != nil {
		log.Fatalf("Error reading input files: %v", err)
	}

	mainPrompt := job.CustomPrompt
	if mainPrompt == "" {
		mainPrompt, err = t.GetDefaultPrompt(layout)
		if err != nil {
			log.Println("error from reading input prompt: ", err)
			return
		}
	}

	//todo: fix this calls arguments it should probably just be one struct.
	err = t.TuneResumeContents(inputTemp, mainPrompt, baselineJSON, layout, style, job.Id.String(), t.Fs, j.config, job, updates)
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

func (j *jobRunner) RunJobStreaming(job *jobPackage.Job) chan jobPackage.JobStatus {
	//job.Id = uuid.New()
	log.Printf("running job %s", job.Id.String())

	updates := make(chan jobPackage.JobStatus)
	go j.RunJob(job, updates)
	//j.RunJob(job, nil)

	//// Add job to queue
	return updates
}
