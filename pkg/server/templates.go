package server

import (
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/xeipuuv/gojsonschema"
	"google.golang.org/api/iterator"
	"io"
	"net/http"
	"path/filepath"
	"pdfinspector/pkg/tuner"
	"sort"
	"strings"
	"time"
)

const MAX_TEMPLATES_ALLOWED_PER_SSO = 100

type Template struct {
	Name          string      `json:"name"`
	Layout        string      `json:"layout"`
	Prompt        string      `json:"prompt"`
	StyleOverride interface{} `json:"style_override"`
	ResumeData    interface{} `json:"resumedata"`
}

// generationInfo holds template metadata.
type templateInfo struct {
	UUID    string    `json:"uuid"`
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// CreateTemplateHandler handles the creation of a new template.
func (s *pdfInspectorServer) CreateTemplateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, _ := ctx.Value("ssoSubject").(string)

	// Step 1: Check API credits.
	_, credits, err := s.GetBestApiKeyForUser(ctx, userID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if credits <= 0 {
		http.Error(w, "Insufficient API credits", http.StatusForbidden)
		return
	}

	template, err := s.getValidatedTemplateFromRequest(w, r)
	if err != nil {
		http.Error(w, "Could not validate Template", http.StatusBadRequest)
		return
	}

	// Step 4: Check if the user has fewer than 100 templates.
	templateCount, err := s.getUserTemplateCount(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to retrieve template count", http.StatusInternalServerError)
		return
	}
	if templateCount >= MAX_TEMPLATES_ALLOWED_PER_SSO {
		http.Error(w, "Template limit reached", http.StatusForbidden)
		return
	}
	log.Info().Msgf("CreateTemplateHandler: user has %d templates", templateCount)

	// Step 5: Save the template to GCS.
	templateID := uuid.New().String()
	objectName := fmt.Sprintf("sso/%s/template/%s-%s.json", userID, templateID, sanitizeFileName(template.Name))

	if err := s.saveTemplateToGCS(ctx, objectName, template); err != nil {
		http.Error(w, "Failed to save template", http.StatusInternalServerError)
		return
	}

	// Step 6: Respond with the template ID.
	response := map[string]string{"template_id": templateID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ReadTemplateHandler handles reading a template.
func (s *pdfInspectorServer) ReadTemplateHandler(w http.ResponseWriter, r *http.Request) {
	templateObjectName := s.getTemplateObjectName(r)
	log.Info().Msgf("ReadTemplateHandler for %s", templateObjectName)
	// Step 2: Read the template from GCS.
	templateData, err := s.readTemplateFromGCS(r.Context(), templateObjectName)
	if err != nil {
		log.Error().Msgf("ReadTemplateHandler error %s", err.Error())
		http.Error(w, "Failed to read template", http.StatusInternalServerError)
		return
	}

	// Step 3: Write the template data to the response.
	w.Header().Set("Content-Type", "application/json")
	w.Write(templateData)
}

// UpdateTemplateHandler handles updating an existing template.
func (s *pdfInspectorServer) UpdateTemplateHandler(w http.ResponseWriter, r *http.Request) {
	templateObjectName := s.getTemplateObjectName(r)
	template, err := s.getValidatedTemplateFromRequest(w, r)
	if err != nil {
		return
	}

	// Step 4: Overwrite the template in GCS.
	if err := s.saveTemplateToGCS(r.Context(), templateObjectName, template); err != nil {
		http.Error(w, "Failed to update template", http.StatusInternalServerError)
		return
	}

	// Step 5: Respond with a success message.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Template updated successfully"))
}

// DeleteTemplateHandler handles deleting a template.
func (s *pdfInspectorServer) DeleteTemplateHandler(w http.ResponseWriter, r *http.Request) {
	templateObjectName := s.getTemplateObjectName(r)
	log.Info().Msgf("DeleteTemplateHandler: here to remove %s", templateObjectName)

	// Step 2: Delete the template from GCS.
	if err := s.deleteTemplateFromGCS(r.Context(), templateObjectName); err != nil {
		http.Error(w, "Failed to delete template", http.StatusInternalServerError)
		return
	}

	// Step 3: Respond with a success message.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Template deleted successfully"))
}

// ListTemplatesHandler handles listing all templates for a user.
func (s *pdfInspectorServer) ListTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, _ := ctx.Value("ssoSubject").(string)

	// Step 1: List templates from GCS.
	templates, err := s.listUserTemplates(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to list templates", http.StatusInternalServerError)
		return
	}

	// Step 2: Respond with the list of templates.
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

func (s *pdfInspectorServer) getValidatedTemplateFromRequest(w http.ResponseWriter, r *http.Request) (*Template, error) {
	// Step 1: Parse the updated template from the request body.
	var template Template
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return nil, err
	}

	// Step 2: Validate 'resumedata' against the schema.
	err := s.validateResumeDataAgainstTemplateSchema(template.Layout, template.ResumeData, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("Schema validation error: %s", err.Error()), http.StatusInternalServerError)
		return nil, err
	}
	return &template, nil
}

func (s *pdfInspectorServer) validateResumeDataAgainstTemplateSchema(layout string, resumeData interface{}, allowRendererFields bool) error {
	//Validate 'resumedata' against the schema.
	var schemaInterface interface{}
	var err error
	if allowRendererFields {
		schemaInterface, err = s.jobRunner.Tuner.GetRendererJsonSchema(layout)
	} else {
		schemaInterface, err = s.jobRunner.Tuner.GetExpectedResponseJsonSchema(layout)
	}
	if err != nil {
		return err
	}
	schemaLoader := gojsonschema.NewGoLoader(schemaInterface)
	documentLoader := gojsonschema.NewGoLoader(resumeData)
	log.Trace().Msgf("here with schema: %#v", schemaInterface)
	log.Trace().Msgf("here with resumeData: %#v", resumeData)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	log.Trace().Msgf("validateResumeDataAgainstTemplateSchema err? %#v", err)
	log.Trace().Msgf("validateResumeDataAgainstTemplateSchema result? %#v", result)
	if err != nil {
		return err
	}
	if !result.Valid() {
		return tuner.NewSchemaValidationError(result.Errors())
	}
	return nil
}

func (s *pdfInspectorServer) getTemplateObjectName(r *http.Request) string {
	userID, _ := r.Context().Value("ssoSubject").(string)
	//templateID := chi.URLParam(r, "templateID")
	//objectName := chi.URLParam(r, "template")
	objectName := r.URL.Query().Get("t")

	// switched from path var to query param because some of my data has parenthesis chars in it. why should that matter? great question....
	// https://github.com/go-chi/chi/issues/642 THIS BUG IS 3 YEARS OLD FFS.
	// https://github.com/go-chi/chi/issues/641 has more info it seems (found in comments of https://git.frostfs.info/TrueCloudLab/frostfs-s3-gw/commit/2733ff9147477ce6b54ee2dad7c968ba5c04ec26 which implements a workaround as mentioned in issue 642 thread)
	// https://github.com/go-chi/chi/issues/832 seems to be about the same thing
	// any others? could go digging ... i kinda dgaf tho at the moment, actually irritated that there is such an old fundamental issue in a project that is being recommended to noobs.
	// .....
	// maybe if i can find the time and inspiration i'll go rip into it and create a PR to fix it.

	templateObjectName := formatTemplateObjectName(userID, objectName)
	return templateObjectName
}

// Helper function to get the count of user templates.
func (s *pdfInspectorServer) getUserTemplateCount(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, errors.New("No userID available for getUserTemplateCount")
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	prefix := fmt.Sprintf("sso/%s/template/", userID)

	query := &storage.Query{Prefix: prefix}
	it := bucket.Objects(ctx, query)

	count := 0
	for {
		_, err := it.Next()
		if errors.Is(err, storage.ErrObjectNotExist) || errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}

// Helper function to save a template to GCS.
func (s *pdfInspectorServer) saveTemplateToGCS(ctx context.Context, objectName string, template *Template) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	obj := bucket.Object(objectName)

	data, err := json.Marshal(template)
	if err != nil {
		return err
	}

	writer := obj.NewWriter(ctx)
	defer writer.Close()

	if _, err := writer.Write(data); err != nil {
		return err
	}

	return nil
}

// Helper function to read a template from GCS.
func (s *pdfInspectorServer) readTemplateFromGCS(ctx context.Context, objectName string) ([]byte, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	obj := bucket.Object(objectName)

	reader, err := obj.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, fmt.Errorf("object not found")
		}
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// Helper function to delete a template from GCS.
func (s *pdfInspectorServer) deleteTemplateFromGCS(ctx context.Context, objectName string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	obj := bucket.Object(objectName)

	if err := obj.Delete(ctx); err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return fmt.Errorf("object not found")
		}
		return err
	}

	return nil
}

// Helper function to list user templates.
const uuidLen = 36

func (s *pdfInspectorServer) listUserTemplates(ctx context.Context, userID string) ([]templateInfo, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	bucket := client.Bucket(s.config.GcsBucket)
	prefix := fmt.Sprintf("sso/%s/template/", userID)

	query := &storage.Query{Prefix: prefix}
	it := bucket.Objects(ctx, query)

	var templates []templateInfo
	for {
		objAttr, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}

		// Extract UUID and name from the object name.
		fileName := strings.TrimPrefix(objAttr.Name, prefix)
		fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

		templates = append(templates, templateInfo{
			UUID:    fileName[:uuidLen],
			Name:    fileName[uuidLen+1:],
			Created: objAttr.Created,
			Updated: objAttr.Updated,
		})
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Updated.After(templates[j].Updated)
	})

	return templates, nil
}

// Helper function to sanitize file names.
func sanitizeFileName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}
