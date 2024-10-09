package jobrunner

import (
	"fmt"
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
	job.Log().Trace().Msgf("do something with this job: %#v", job)

	err := j.Tuner.PopulateJob(job, updates)
	if err != nil {
		job.Log().Error().Msgf("Error from PopulateJob: %v", err)
	}
	job.Log().Trace().Msgf("debug here job output dir: %s", job.OutputDir)

	err = j.Tuner.TuneResumeContents(job, updates)
	if err != nil {
		job.Log().Error().Msgf("Error from resume tuning: %v", err)
		tuner.SendJobErrorUpdate(updates, fmt.Sprintf("Error from resume tuning: %v", err))
		//todo send an update that is flagged as an error so that runner can report the failure.
	}
}

func (j *JobRunner) RunJobStreaming(inputJob *job.Job) chan job.JobStatus {
	inputJob.Log().Info().Msgf("running job")
	updates := make(chan job.JobStatus)
	go j.RunJob(inputJob, updates)

	//// Add job to queue
	return updates
}

func (j *JobRunner) RunRenderJob(job *job.RenderJob, updates chan job.JobStatus) {
	if updates != nil {
		defer close(updates)
	}
	job.Log().Trace().Msgf("do something with this job: %#v", job)

	err := j.Tuner.PopulateRenderJob(job, updates)
	if err != nil {
		job.Log().Error().Msgf("Error from PopulateRenderJob: %v", err)
	}
	job.Log().Trace().Msgf("debug here job output dir: %s", job.OutputDir)

	err = j.Tuner.RenderResume(job, updates)
	if err != nil {
		job.Log().Error().Msgf("Error from resume rendering: %v", err)
		tuner.SendJobErrorUpdate(updates, fmt.Sprintf("Error from resume tuning: %v", err))
		//todo send an update that is flagged as an error so that runner can report the failure.
	}
}

func (j *JobRunner) RunRenderStreaming(inputJob *job.RenderJob) chan job.JobStatus {
	inputJob.Log().Info().Msgf("running job")
	updates := make(chan job.JobStatus)
	go j.RunRenderJob(inputJob, updates)

	return updates
}
