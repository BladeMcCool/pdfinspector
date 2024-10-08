package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"pdfinspector/pkg/job"
	"pdfinspector/pkg/tuner"
)

func (s *pdfInspectorServer) extractResumeHandler(w http.ResponseWriter, r *http.Request) {
	//Verify first that we have space to even create a template at this stage.
	ctx := r.Context()
	userID, _ := ctx.Value("ssoSubject").(string)
	templateCount, err := s.getUserTemplateCount(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to retrieve template count", http.StatusInternalServerError)
		return
	}
	if templateCount >= MAX_TEMPLATES_ALLOWED_PER_SSO {
		http.Error(w, "Template limit reached - Delete template(s) first.", http.StatusForbidden)
		return
	}

	// Limit request size (500KB for file size limit)
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	// Parse multipart form
	err = r.ParseMultipartForm(0) // Just call this to parse form data
	if err != nil {
		log.Trace().Msgf("parse multipart error: %v", err)
		http.Error(w, "Error parsing form data", http.StatusBadRequest)
		return
	}

	// Retrieve file from form
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check MIME type by reading the first few bytes of the file
	// (512 bytes is a good size to detect the content type)
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Reset file reader position to the start of the file
	file.Seek(0, io.SeekStart)

	// Detect MIME type
	contentType := http.DetectContentType(buffer)
	if contentType != "application/pdf" {
		http.Error(w, "Invalid file type. Only PDF is allowed.", http.StatusUnsupportedMediaType)
		return
	}

	// Read file content into a byte slice
	fileContent, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	//todo think about whether this should be something that deducts api credit.

	// Get layout parameter from request
	layout := chi.URLParam(r, "layout")

	// Set headers for streaming response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	updates := make(chan job.JobStatus)
	finalResult := make(chan job.ExtractResult, 1)
	go func() {
		//do the actual job and send updates.
		defer close(updates)
		updates <- job.JobStatus{Message: "Starting resume processing"}

		// Process the resume contents using the extractResumeContents method
		extractionResult, err := s.jobRunner.Tuner.ExtractResumeContents(&tuner.ResumeExtractionJob{
			FileContent: fileContent,
			Layout:      layout,
			UseSystemGs: s.config.UseSystemGs,
			UserID:      userID,
		}, updates)
		log.Trace().Msgf("got resume result: %v", extractionResult)
		if err == nil {
			updates <- job.JobStatus{Message: "Extracted resume data into JSON"}
		} else {
			log.Error().Msgf("error from extraction: %v", err)
			finalResult <- job.ExtractResult{JobStatus: job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}}
			return
		}

		var decodedResumeData interface{}
		if err := json.Unmarshal([]byte(extractionResult.ResumeJSONRaw), &decodedResumeData); err != nil {
			log.Error().Msgf("error from decoding resume json extraction: %v", err)
			finalResult <- job.ExtractResult{JobStatus: job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}}
			return
		}

		err = s.validateResumeDataAgainstTemplateSchema(layout, decodedResumeData)
		if err == nil {
			updates <- job.JobStatus{Message: "ResumeData format appears to be valid"}
		} else {
			finalResult <- job.ExtractResult{JobStatus: job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}}
			return
		}

		candidateNameBestGuess, _ := s.jobRunner.Tuner.GuessCandidateName(decodedResumeData)
		template := &Template{
			Name:          fmt.Sprintf("Generated Template for %s with %s layout", candidateNameBestGuess, layout),
			Layout:        layout,
			Prompt:        "",
			StyleOverride: nil,
			ResumeData:    decodedResumeData,
		}
		err = s.saveAsTemplate(r.Context(), userID, template)
		if err == nil {
			updates <- job.JobStatus{Message: "Saved template"}
		} else {
			log.Error().Msgf("error from saving template: %v", err)
			finalResult <- job.ExtractResult{JobStatus: job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}}
			return
		}

		finalResult <- job.ExtractResult{
			JobStatus: job.JobStatus{
				Message: "Finished successfully - saved template",
			},
			TemplateName: &template.Name,
		}
	}()
	for status := range updates {
		if status.Error != nil {
			log.Info().Msgf("Resume Data Extract Error: %s", status.Message)
		} else {
			log.Info().Msgf("Resume Data Extract Status Update: %s", status.Message)
		}

		data, err := json.Marshal(status)
		if err != nil {
			http.Error(w, "Error encoding status", http.StatusInternalServerError)
			return
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		if err != nil {
			log.Debug().Msg("Client connection lost.")
			return
		}

		// Flush the response writer to ensure the data is sent immediately
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	var resumeResult interface{} = <-finalResult
	finalData, err := json.Marshal(resumeResult)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
		finalData = []byte(`{"message":"processing error","error": true}`)
		return
	}

	// Send the final JSON result to the client
	fmt.Fprintf(w, "%s\n", finalData)

	// Flush final response to the client
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *pdfInspectorServer) saveAsTemplate(ctx context.Context, userID string, template *Template) error {
	// Step 5: Save the template to GCS.
	templateID := uuid.New().String()
	objectName := fmt.Sprintf("sso/%s/template/%s-%s.json", userID, templateID, sanitizeFileName(template.Name))

	if err := s.saveTemplateToGCS(ctx, objectName, template); err != nil {
		return err
	}

	return nil
}
