package tuner

import (
	"errors"
	"fmt"
	"pdfinspector/pkg/job"
	"time"
)

func (t *Tuner) PopulateRenderJob(job *job.RenderJob, updates chan job.JobStatus) error {
	job.OutputDir = fmt.Sprintf("%s/%s", t.config.LocalPath, job.Id)

	return nil
}

func (t *Tuner) RenderResume(renderJob *job.RenderJob, updates chan job.JobStatus) error {
	renderJob.Log().Info().Str("user_key", renderJob.UserKey).Msgf("starting RenderResume")

	//a chopped down version of resume tune, take the json and render it. one attempt, and that was the 'best' one. then we're done!

	content := renderJob.BaselineJSON
	err := validateJSON(content)
	if err != nil {
		renderJob.Log().Error().Msgf("Error validating JSON content: %v", err)
		return err
	}
	renderJob.Log().Info().Msgf("Got %d bytes of JSON content (at least well formed enough to be decodable) out of that last response", len(content))
	attemptNum := 0 //there will only be this attempt since we're not changing it.
	SendJobUpdate(updates, fmt.Sprintf("got JSON for attempt %d, will request PDF", attemptNum))
	compatibilityJob := &job.Job{
		Id:            renderJob.Id,
		UserID:        renderJob.UserID,
		BaselineJSON:  renderJob.BaselineJSON,
		StyleOverride: renderJob.StyleOverride,
		Layout:        renderJob.Layout,
		Logger:        renderJob.Logger,
		OutputDir:     renderJob.OutputDir,
	}
	err = WriteAttemptResumedataJSON(content, compatibilityJob, attemptNum, t.Fs, t.config)

	//we should be able to render that updated content proposal now via gotenberg + ghostscript
	maxGotenAttempts := 3
	for k := 0; k < maxGotenAttempts; k++ {
		err = makePDFRequestAndSave(attemptNum, t.config, compatibilityJob)
		if err == nil {
			break
		}
		var httpErr *GotenbergHTTPError
		if errors.As(err, &httpErr) {
			// Handle the error based on the HTTP code
			renderJob.Log().Info().Msgf("Got a retryable error from Gotenberg, code %d", httpErr.HttpResponseCode)
			time.Sleep(1 * time.Second)
			continue
		} else if err != nil {
			return err
		}
	}
	SendJobUpdate(updates, fmt.Sprintf("got PDF for attempt %d, will dump to PNG", attemptNum))

	//this part is now just for information, we do inspect for render error so its not entirely useless.
	//ghostscript dump to pngs ...
	err = dumpPDFToPNG(attemptNum, renderJob.OutputDir, t.config)
	if err != nil {
		return fmt.Errorf("Error during pdf to image dump: %v", err)
	}
	SendJobUpdate(updates, fmt.Sprintf("got PNGs for attempt %d, will check it", attemptNum))

	result, err := inspectPNGFiles(renderJob.OutputDir, attemptNum)
	if err != nil {
		renderJob.Log().Error().Msgf("Error inspecting png files: %v", err)
		return err
	}
	SendJobUpdate(updates, fmt.Sprintf("attempt %d png inspection, content ratio: %.2f, page count: %d", attemptNum, result.LastPageContentRatio, result.NumberOfPages))

	attemptsLog := []inspectResult{result}
	err = saveBestAttemptToGCS(attemptsLog, t.Fs, t.config, compatibilityJob, updates)
	if err != nil {
		return err
	}

	return nil
}
