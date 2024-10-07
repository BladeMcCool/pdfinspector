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

	//todo take da money before hitting openai.

	// Get layout parameter from request
	layout := chi.URLParam(r, "layout")
	_, _ = fileContent, layout
	// Retrieve the layout schema based on the requested layout
	//layoutSchema := getLayoutSchema(layout)

	// Set headers for streaming response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	updates := make(chan job.JobStatus)
	finalResult := make(chan job.JobStatus, 1)
	go func() {
		//do the actual job and send updates.
		defer close(updates)
		_ = updates
		updates <- job.JobStatus{Message: "Starting resume processing"}

		// Process the resume contents using the extractResumeContents method
		resumeResult, err := s.jobRunner.Tuner.ExtractResumeContents(fileContent, layout, s.config.UseSystemGs, updates)
		if err == nil {
			updates <- job.JobStatus{Message: "Extracted resume data into JSON"}
		} else {
			log.Error().Msgf("error from extraction: %v", err)
			finalResult <- job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}
			return
		}

		var decodedResumeData interface{}
		if err := json.Unmarshal([]byte(resumeResult.ResumeJSONRaw), &decodedResumeData); err != nil {
			log.Error().Msgf("error from decoding resume json extraction: %v", err)
			finalResult <- job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}
			return
		}

		err = s.validateResumeDataAgainstTemplateSchema(layout, decodedResumeData)
		if err == nil {
			updates <- job.JobStatus{Message: "ResumeData format appears to be valid"}
		} else {
			finalResult <- job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}
			return
		}

		_ = resumeResult
		log.Trace().Msgf("got result: %v", resumeResult)
		candidateNameBestGuess, _ := s.jobRunner.Tuner.GuessCandidateName(decodedResumeData)
		//template, err := s.jobRunner.Tuner.SaveAsTemplate(resumeResult)
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
			finalResult <- job.JobStatus{Message: err.Error(), Error: &tuner.TrueVal}
			return
		}

		finalResult <- job.JobStatus{Message: "Finished successfully - saved template "}
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
		// Write the JSON status update to the response
		//if status.Error != nil {
		//	encounteredError = true
		//}
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

	////temp/debug//////
	//var resumeResult interface{} = map[string]struct{}{}
	var resumeResult interface{} = <-finalResult
	//var resumeResult = <- finalResult
	err = nil
	////end temp/debug//////

	if err != nil {
		log.Error().Err(err).Msg("Failed to extract resume contents")
		http.Error(w, "Error processing resume", http.StatusInternalServerError)
		return
	}

	// Return the result as JSON response
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(resumeResult)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode JSON response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
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
