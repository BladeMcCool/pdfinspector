package main

import "log"

type jobRunner struct{}

func (j *jobRunner) runJob(job *Job) {
	log.Printf("running job %s", job.Id.String())
}
