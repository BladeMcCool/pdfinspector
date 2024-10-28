package tuner

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xeipuuv/gojsonschema"
	"io"

	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/filesystem"
	"pdfinspector/pkg/job"
	"strings"
)

const TUNER_DEFAULT_OUTPUT_FILENAME = "Output.pdf"

// Custom error type that holds a list of validation errors
type SchemaValidationError struct {
	ValidationErrors []gojsonschema.ResultError
}

// Implement the error interface by defining the Error() method
func (ve SchemaValidationError) Error() string {
	if len(ve.ValidationErrors) == 0 {
		return "no validation errors"
	}

	// Format all validation errors into a single string
	var errMsg string
	for _, err := range ve.ValidationErrors {
		errMsg += fmt.Sprintf("- %s\n", err.String())
	}
	return fmt.Sprintf("validation failed with the following errors:\n%s", errMsg)
}

// Optional: Implement the Is() method to allow errors.Is to check this type
func (ve SchemaValidationError) Is(target error) bool {
	_, ok := target.(*SchemaValidationError)
	return ok
}

// Constructor function for creating a new SchemaValidationError
func NewSchemaValidationError(errors []gojsonschema.ResultError) error {
	return &SchemaValidationError{
		ValidationErrors: errors,
	}
}

type jdMeta struct {
	CompanyName string   `json:"company_name" validate:"required"`
	JobTitle    string   `json:"job_title" validate:"required"`
	Keywords    []string `json:"keywords" validate:"required"`
	Location    string   `json:"location" validate:"required"`
	RemoteOK    *bool    `json:"remote_ok" validate:"required"`
	SalaryInfo  *string  `json:"salary_info" validate:"required"`
	Process     *string  `json:"process" validate:"required"`
}

// APIResponse Struct to represent (parts of) the OpenAI API response (that we care about r/n)
type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type Tuner struct {
	config *config.ServiceConfig
	Fs     filesystem.FileSystem
}

func NewTuner(config *config.ServiceConfig) *Tuner {
	t := &Tuner{
		config: config,
	}
	t.configureFilesystem()
	return t
}

var defaultAcceptableRatio = 0.88

// var defaultMaxAttempts = 1 //this should probably come from env
var defaultMaxAttempts = 7 //this should probably come from env
const RESUME_FILENAME = "Resume.pdf"
const COVERLETTER_FILENAME = "Cover Letter.pdf"

type LayoutCustomization struct {
	//min and max length for page
	//prompts
	AcceptableRatio float64
	DefaultPrompt   string
	ComposePrompt   func(job *job.Job, keywords []string) string
	CanSupplement   bool //true if this type of layout might need to have prompt supplemented with some sort of data from gcs
	OutputFilename  string
}

var layoutDefaults = map[string]LayoutCustomization{
	"chrono": {
		AcceptableRatio: defaultAcceptableRatio,
		DefaultPrompt: strings.Join([]string{
			"The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. ",
			//"Your output JSON can simply omit anything which need not be seen in the rendered resume document (If all of the projects within a job are marked as hidden then the whole job will be hidden). ",
			//"The work_history contains a list of companies and projects within those companies. ",
			"Some of the information in the candidate resume JSON data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description below. ",
			"Your output JSON can simply omit anything which need not be seen in the rendered resume document. ",
			"Feel free to adjust any descriptive text fields at the company or project level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. ",
			"Embellishment of anything remotely factual or possibly tangential is encouraged. ",
			"Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to include it. ",
			"The goal is to keep the length of the final render at one page, while showing the most relevant information to make the candidate appear a perfect fit for the target job. ",
			"Be sure to include between 3 and 5 distinct company sections. Each company section can list separate projects within it, aim for 2-3 projects within each company. ",
			"Make sure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed. ",
			"The target Job Description for which this candidate should appear to perfectly match is below. ",
			"Pay special attention to any special tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability:",
		}, ""),
		ComposePrompt:  resumeComposePrompt,
		OutputFilename: RESUME_FILENAME,
	},
	"functional": {
		AcceptableRatio: defaultAcceptableRatio,
		DefaultPrompt: strings.Join([]string{
			//"This guy needs a job ASAP. You need to make his resume look PERFECT for the job. Fake it until you make it right? Fix it up, do what it takes. Aim for 3-5 Functional Areas, each with 2-4 examples of key contributions. Make them relate to the Job Description as best as possible, including possibly switching up industries and industry terms. ",
			//"Feel free to dig into those hidden companies and projects for inspiration, include whatever you think could be relevant. ",
			//"The target Job Description for which this candidate should appear to perfectly match is below. Pay special attention to any magic tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability (magic tokens should be placed somewhere that will get rendered such as a project description or a job title):\n",

			"The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. ",
			"Some of the information in the candidate resume JSON data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description below. ",
			"Your output JSON can simply omit anything which need not be seen in the rendered resume document. ",
			"Feel free to adjust any descriptive text fields at the functional area or key contribution level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. ",
			"Embellishment of anything remotely factual or possibly tangential is encouraged. ",
			"Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to include it. ",
			"The goal is to keep the length of the final render at one page, while showing the most relevant information to make the candidate appear a perfect fit for the target job. ",
			"Be sure to include between 3 and 5 distinct functional areas. Each functional area can list separate key contributions within it, aim for 2-3 examples within each. ",
			"Ensure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed, ",
			"The target Job Description for which this candidate should appear to perfectly match is below. ",
			"Pay special attention to any special tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability:",
		}, ""),
		ComposePrompt:  resumeComposePrompt,
		OutputFilename: RESUME_FILENAME,
	},
	"coverletter": {
		AcceptableRatio: 0.55,
		DefaultPrompt: strings.Join([]string{
			"Write a cover letter for the candidate which matches the Job Description below.",
		}, ""),
		ComposePrompt:  coverletterComposePrompt,
		CanSupplement:  true,
		OutputFilename: COVERLETTER_FILENAME,
	},
}

func (t *Tuner) PopulateJob(job *job.Job, updates chan job.JobStatus) error {
	job.OutputDir = fmt.Sprintf("%s/%s", t.config.LocalPath, job.Id)

	if job.BaselineJSON == "" && job.Baseline != "" && job.IsForAdmin {
		baselineJSON, err := t.GetBaselineJSON(job.Baseline)
		if err != nil {
			return fmt.Errorf("error from reading baseline JSON: %v", err)
		}
		job.BaselineJSON = baselineJSON
	}
	SendJobUpdate(updates, "got baseline JSON")

	acceptableRatio, err := t.GetAcceptableRatio(job.Layout)
	if err != nil {
		job.Log().Error().Msgf("error from reading input prompt: %s", err.Error())
		return err
	}
	job.AcceptableRatio = acceptableRatio
	job.MaxAttempts = defaultMaxAttempts

	//var err error
	mainPrompt := job.CustomPrompt
	if mainPrompt == "" {
		mainPrompt, err = t.GetDefaultPrompt(job.Layout)
		if err != nil {
			job.Log().Error().Msgf("error from reading input prompt: %s", err.Error())
			return err
		}
		job.Log().Info().Msgf("used standard main prompt: %s", mainPrompt)
	}
	job.MainPrompt = mainPrompt

	job.SupplementData = t.GetJobSupplement(job)

	//todo possibly move the call to GetCompletePromptForLayout into here? and set the full prompt into the job? (might have to add new field)
	// or ... MainPrompt just becomes the full complete actual prompt to use incl layout specific stuff? idk ... maybe thats better.
	return nil
}

func (t *Tuner) GetBaselineJSON(baseline string) (string, error) {
	// get JSON of the current complete resume including all the hidden stuff, this hits an express server that imports the reactresume resumedata.mjs and outputs it as json.
	jsonRequestURL := fmt.Sprintf("%s?baseline=%s", t.config.JsonServerURL, baseline)

	client, err := createAuthenticatedClient(context.Background(), t.config.ReactAppURL) //we use the reactapp url because we have to implement the security in the application due to the way the react app will load the json -- having to first send an OPTIONS request which needs to just be allowed through since it will never have a bearer token. And then the bearer token we do send for the gotenberg request to the react app will always be the one forwarded in the json fetch request, we cannot override it with the correct one even! so here, we just be consistent.
	if err != nil {
		return "", fmt.Errorf("failed to create authenticated client: %v", err)
	}
	req, err := http.NewRequest("GET", jsonRequestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to define HTTP request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Failed to make the HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Got undesired status code in response from JSON server: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read the response body: %v", err)
	}
	log.Info().Msgf("got %d bytes of json from the json-server via %s", len(body), jsonRequestURL)
	return string(body), nil
}

func (t *Tuner) GetStyleFromBaselineJSON(baselineJSON string) (string, string, error) {
	// deprecated outside of the 'main' run. server submitted jobs should be explicit about the layout
	// layout/style override dont get baked into the baseline data until right before we save to GCS.

	log.Trace().Msgf("dbg baselinejson %s", baselineJSON)
	var decoded map[string]interface{}
	err := json.Unmarshal([]byte(baselineJSON), &decoded)
	if err != nil {
		return "", "", err
	}

	//Check if the "layout" key exists and is a string
	layout, ok := decoded["layout"].(string)
	if !ok {
		return "", "", errors.New("layout is missing or not a string")
	}
	//Check if the "style" key exists and is a string (its ok if its not there but if it is we should keep it)
	style, _ := decoded["style"].(string)

	return layout, style, nil
}

func (t *Tuner) takeNotesOnJD(job *job.Job) (string, error) {
	jDResponseSchemaRaw, err := os.ReadFile(filepath.Join("response_templates", "jdinfo-schema.json"))
	if err != nil {
		return "", fmt.Errorf("failed to read expect_response.json: %v", err)
	}
	// Validate the JSON content
	jDResponseSchema, err := DecodeJSON(string(jDResponseSchemaRaw))
	if err != nil {
		return "", fmt.Errorf("failed to decode JSON: %v", err)
	}
	prompt := strings.Join([]string{
		"Extract information from the following Job Description. Take note of the name of the company, the job title, and most importantly the list of key words that a candidate will have in their CV in order to get through initial screening. Additionally, extract any location, remote-ok status, salary info and hiring process notes which can be succinctly captured.",
		"\n--- start job description ---\n",
		job.JobDescription,
		"\n--- end job description ---\n",
	}, "")

	apirequest := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "You are a Job Description info extractor assistant.",
			},
			//before switching to structured output we had to prompt it here with a message telling it to respond in json and then providing a fake response from the assistant in the expected json format. now we just send a json schema in a different part of the request :D
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "job_description",
				"strict": true,
				"schema": jDResponseSchema,
			},
		},
		"temperature": 0.7,
		"user":        job.UserID,
	}
	api_request_pretty, err := serializeToJSON(apirequest)
	writeToFile(api_request_pretty, 0, "jd_info_request_pretty", job.OutputDir)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal final JSON: %v", err)
	}

	exists, output, err := checkForPreexistingAPIOutput(job.OutputDir, "jd_info_response_raw", 0)
	if err != nil {
		return "", fmt.Errorf("Error checking for pre-existing API output: %v", err)
	}
	if !exists {
		output, err = t.makeAPIRequest(apirequest, 0, "jd_info_response_raw", job.OutputDir)
		if err != nil {
			return "", fmt.Errorf("Error making API request: %v", err)
		}
	}

	//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
	var apiResponse APIResponse
	err = json.Unmarshal([]byte(output), &apiResponse)
	if err != nil {
		return "", fmt.Errorf("Error deserializing API response: %v\n", err)
	}

	//Extract the message content
	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("No choices found in the API response")
	}

	content := apiResponse.Choices[0].Message.Content

	err = validateJSON(content)
	if err != nil {
		return "", fmt.Errorf("Error validating JSON content: %v\n", err)
	}
	log.Info().Msgf("Got %d bytes of JSON content about the JD (at least well formed enough to be decodable) out of that last response", len(content))

	outputFilePath := filepath.Join(job.OutputDir, "jdinfo-out.json")
	err = WriteValidatedContent(content, outputFilePath)
	if err != nil {
		return "", fmt.Errorf("Error writing content to file: %v\n", err)
	}
	log.Info().Msgf("JD Info Content successfully written to: %s", outputFilePath)
	return content, nil
}

// configureFilesystem sets up the filesystem based on the command line flags.
func (t *Tuner) configureFilesystem() filesystem.FileSystem {
	if t.config.FsType == "local" {
		t.Fs = &filesystem.LocalFileSystem{BasePath: t.config.LocalPath}
	} else if t.config.FsType == "gcs" {
		// Create a new GCS client
		log.Info().Msg("setting up gcs client ...")
		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatal().Msgf("Failed to create GCS client: %v", err)
		}
		if t.config.GcsBucket == "" {
			log.Fatal().Msg("gcs-bucket arg needs to have a value")
		}
		t.Fs = &filesystem.GCSFileSystem{Client: client, BucketName: t.config.GcsBucket}
	}
	return nil
}

func (t *Tuner) GetExpectedResponseJsonSchema(layout string) (interface{}, error) {
	completeSchema, err := t.readAndDecodeJsonSchema(layout)
	if err != nil {
		return nil, err
	}
	stripped := config.ExtractRelevantSchema(completeSchema)
	return stripped, nil
}

func (t *Tuner) GetRendererJsonSchema(layout string) (interface{}, error) {
	//which is essentially response schema format with a few other fields that only the renderer (ReactResume project) will use.

	completeSchema, err := t.readAndDecodeJsonSchema(layout)
	if err != nil {
		return nil, err
	}
	stripped := config.ExtractRelevantSchema(completeSchema)
	enhanced, err := config.EnhanceSchemaWithRendererFields(layout, stripped)
	if err != nil {
		return nil, err
	}

	return enhanced, nil
}

func (t *Tuner) GetCompleteJsonSchema(layout string) (interface{}, error) {
	completeSchema, err := t.readAndDecodeJsonSchema(layout)
	if err != nil {
		return nil, err
	}
	return completeSchema, nil
}

func (t *Tuner) readAndDecodeJsonSchema(layout string) (interface{}, error) {
	//todo cache this stuff in a map - there will only ever be a few of these, its going to be reading it from the filesystem every time!
	expectResponseFilePath := filepath.Join(t.config.SchemasPath, fmt.Sprintf("%s-schema.json", layout))
	// Read expect_response.json
	log.Trace().Msgf("readAndDecodeJsonSchema, expectResponseFilePath is: %s", expectResponseFilePath)

	expectResponseContent, err := os.ReadFile(expectResponseFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read expect_response.json: %v", err)
	}
	// Validate the JSON content
	expectResponseSchema, err := DecodeJSON(string(expectResponseContent))
	if err != nil {
		return nil, err
	}
	return expectResponseSchema, nil
}

func (t *Tuner) GetLayoutDefaults(layout string) (*LayoutCustomization, error) {
	defaults, ok := layoutDefaults[layout]
	if !ok {
		return nil, fmt.Errorf("Unknown layout: %s", layout)
	}

	return &defaults, nil
}

func (t *Tuner) GetAcceptableRatio(layout string) (float64, error) {
	defaults, err := t.GetLayoutDefaults(layout)
	if err != nil {
		return 0.0, err
	}

	return defaults.AcceptableRatio, nil
}

func (t *Tuner) GetDefaultPrompt(layout string) (string, error) {
	defaults, err := t.GetLayoutDefaults(layout)
	if err != nil {
		return "", err
	}

	return defaults.DefaultPrompt, nil
}

func (t *Tuner) GetJobSupplement(job *job.Job) []byte {
	if job.Supplement == "" {
		return nil
	}
	if job.UserID == "" {
		//idk this shouldnt really happen but if it did we wont be able to find any supplement anyway.
		log.Error().Msgf("Why are we trying to supplment when we dont have a UserID?")
		return nil
	}
	defaults, err := t.GetLayoutDefaults(job.Layout)
	if err != nil {
		return nil
	}
	if !defaults.CanSupplement {
		return nil
	}

	//only type of supplement atm is a named template from the users collection.
	//the idea is that the form submission for document tuning will include the jd and the baseline 'coverletter' json, so that they can tune an existing cover letter, same as the resume tuning.
	//however, without resumedata to inform the tune of the cover letter along with the JD, the context for the cover letter will be pretty weak (just a JD and some personal info in the baseline cover letter).
	// sooooo ... what i came up with to allow this but not break everything else is you can select a template of resumedata (or a coverletter but that would be less useful) to go along with the prompt
	supplementGcsObjectname := fmt.Sprintf("sso/%s/template/%s.json", job.UserID, job.Supplement)
	log.Info().Msgf("GetJobSupplement for a layout (%s) that allows it, on a job that specifies it: %s - object name: %s", job.Layout, job.Supplement, supplementGcsObjectname)
	fileBytes, err := t.Fs.ReadFile(context.Background(), supplementGcsObjectname)
	if err != nil {
		log.Info().Msgf("error obtaining suppplement, will just skip it: %s", err.Error())
		panic("stop here while testing. should be getting it")
		return nil
	}
	log.Info().Msgf("GetJobSupplement obtained %d bytes of template json to be able to use in prompting.", len(fileBytes))

	return fileBytes
}

func (t *Tuner) GetCompletePromptForLayout(job *job.Job, keywords []string) (string, error) {
	defaults, err := t.GetLayoutDefaults(job.Layout)
	if err != nil {
		return "", err
	}

	return defaults.ComposePrompt(job, keywords), nil
}

func (t *Tuner) GetOuputFileName(layout string) string {
	defaults, err := t.GetLayoutDefaults(layout)
	if err != nil {
		return TUNER_DEFAULT_OUTPUT_FILENAME
	}

	return defaults.OutputFilename
}

func (t *Tuner) makeAPIRequest(apiBody interface{}, counter int, name, outputDir string) (string, error) {
	//panic("slow down there son, you really want to hit the paid api at this time?")
	log.Info().Msgf("Make request to OpenAI ...")
	// Serialize the interface to pretty-printed JSON
	jsonData, err := json.Marshal(apiBody)
	if err != nil {
		return "", fmt.Errorf("failed to serialize API request body to JSON: %v", err)
	}

	// Create a new HTTP POST request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Set the Content-Type and Authorization headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.config.OpenAiApiKey))

	// Send the request using the default HTTP client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Convert the response body to a string
	responseString := string(respBody)

	// Write the response to the filesystem
	err = writeToFile(responseString, counter, name, outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to write response to file: %v", err)
	}
	log.Info().Msgf("Got response from OpenAI API ... (and should have wrote it to the file system)")

	// Return the response string
	return responseString, nil
}
