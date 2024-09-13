package jobrunner

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/job"
	"pdfinspector/pkg/tuner"
)

type JobRunner struct {
	Config *config.ServiceConfig
	Tuner  *tuner.Tuner
}

func (j *JobRunner) RunJob(job *job.Job, updates chan job.JobStatus) {
	if updates != nil {
		defer close(updates)
	}
	if !job.IsForAdmin {
		tuner.SendJobUpdate(updates, fmt.Sprintf("credit remaining: %d", job.UserCreditRemaining))
	}
	log.Trace().Msgf("do something with this job: %#v", job)

	t := j.Tuner

	err := t.PopulateJob(job, updates)
	if err != nil {
		log.Error().Msgf("Error from PopulateJob: %v", err)
	}

	err = t.TuneResumeContents(job, updates)
	if err != nil {
		log.Error().Msgf("Error from resume tuning: %v", err)
		tuner.SendJobErrorUpdate(updates, fmt.Sprintf("Error from resume tuning: %v", err))
		//todo send an update that is flagged as an error so that runner can report the failure.
	}
}

func (j *JobRunner) RunJobStreaming(inputJob *job.Job) chan job.JobStatus {
	log.Info().Msgf("running job %s", inputJob.Id.String())
	updates := make(chan job.JobStatus)
	go j.RunJob(inputJob, updates)

	//// Add job to queue
	return updates
}
