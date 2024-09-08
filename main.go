package main

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"image"
	_ "image/png" // Importing PNG support
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"pdfinspector/filesystem"
	"sort"
	"strconv"
	"strings"
)

// Job represents the structure for a job
type Job struct {
	JobDescription string `json:"jd"`
	Baseline       string `json:"baseline"`      //the actual layout to use is a property of the baseline resumedata.
	BaselineJSON   string `json:"baseline_json"` //the actual layout to use is a property of the baseline resumedata.
	CustomPrompt   string `json:"prompt"`
	StyleOverride  string `json:"style_override"` //eg fluffy
	Id             uuid.UUID

	//anything else we want as options per-job? i was thinking include_bio might be a good option. (todo: ability to not show it on functional, ability to show it on chrono, and then json schema tuning depending on if it is set or not so that the gpt can know to specify it - and dont include it when it shouldn't!)
}

// JobResult represents a job result with its status and result data.
type JobResult struct {
	ID     int
	Status string
	Result string
}

// Global variables for job management
var (
	jobQueue = make(chan Job, 100) // Queue to hold jobs
	//jobIDCounter = 1
	//mu           sync.Mutex
	fs filesystem.FileSystem // Filesystem interface to handle storage

	//this stuff that should probably be cli overridable at least, todo.
	acceptableRatio = 0.88
	maxAttempts     = 7
)

// serviceConfig struct to hold the configuration values
type serviceConfig struct {
	gotenbergURL  string
	jsonServerURL string
	reactAppURL   string
	fsType        string
	mode          string
	localPath     string
	gcsBucket     string
	openAiApiKey  string //oh noes the capitalization *hand waving* guess what? idgaf :) my way.
	useSystemGs   bool   //in the deployed environment we will bake a gs into the image that runs this part, so we can just use a 'gs' command locally.
}

// main function: Either runs the web server or executes the main functionality.
func main() {
	// Get the service configuration
	config := getServiceConfig()

	//mode := parseFlags()
	//log.Printf("mode: %s, fsType: %s", mode, fsType)
	//panic("stop for now")

	//gotenbergURL := getEnv("GOTENBERG_URL", "http://localhost:3000")
	//jsonServerURL := getEnv("JSON_SERVER_URL", "http://localhost:4000")
	//reactAppURL := getEnv("REACT_APP_URL", "http://localhost:5000")
	//gcsBucket := getEnv("GCS_BUCKET", "my-stinky-bucket")

	// Configure filesystem
	localPath := flag.Lookup("local-path").Value.(flag.Getter).Get().(string)
	_ = localPath
	//gcsBucket := flag.Lookup("gcs-bucket").Value.(flag.Getter).Get().(string)
	fs = configureFilesystem(config)

	if config.mode == "cli" {
		// CLI execution mode
		fmt.Println("Running main functionality from CLI...")
		oldMain(fs, config)
		// Replace the following with your main functionality
		fmt.Println("Finished Executing main functionality via CLI")
		return
	}

	// Start worker
	go worker(config)

	// Web server mode
	http.HandleFunc("/submitjob", handleJobSubmission)
	http.HandleFunc("/checkjob/", handleJobStatus)

	fmt.Println("Starting server on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// todo: investigate if we can try out that generic stuff so that i dont have to have 2 versions of a function one for string and one for bool.
// Helper function to get value from CLI args, env vars, or default
func getConfig(cliValue *string, envVar string, defaultValue string) string {
	if *cliValue != "" {
		return *cliValue
	}
	if value, exists := os.LookupEnv(envVar); exists {
		return value
	}
	return defaultValue
}
func getConfigBool(cliValue *bool, envVar string, defaultValue bool) bool {
	// First, check if the CLI value is provided
	if *cliValue {
		return *cliValue
	} else if envVal, exists := os.LookupEnv(envVar); exists {
		// Otherwise, check if the environment variable exists and is parseable as a bool
		parsedValue, err := strconv.ParseBool(envVal)
		if err != nil {
			return defaultValue
		}
		return parsedValue
	}

	// If neither is provided, return the default value
	return defaultValue
}

// getServiceConfig function to return a pointer to serviceConfig
func getServiceConfig() *serviceConfig {
	// Define CLI flags
	gotenbergURL := flag.String("gotenberg-url", "", "URL for Gotenberg service")
	jsonServerURL := flag.String("json-server-url", "", "URL for JSON server")
	reactAppURL := flag.String("react-app-url", "", "URL for React app")
	gcsBucket := flag.String("gcs-bucket", "", "File system type (local or gcs)")
	openAiApiKey := flag.String("api-key", "", "OpenAI API Key")
	localPath := flag.String("local-path", "", "Mode of the application (server or cli)")
	fstype := flag.String("fstype", "", "File system type (local or gcs)")
	mode := flag.String("mode", "", "Mode of the application (server or cli)")
	useSystemGs := flag.Bool("use-system-gs", false, "Use GhostScript from the system instead of via docker run")

	// Parse CLI flags
	flag.Parse()

	//var useSystemGsEnvVar
	//useSystemGsX, err := strconv.ParseBool(getConfig(useSystemGs, "USE_SYSTEM_GS", "false"))
	//if err == nil {
	//	log.Fatalf("%v", err)
	//}
	//, // Default to "server"

	// Populate the serviceConfig struct
	config := &serviceConfig{
		gotenbergURL:  getConfig(gotenbergURL, "GOTENBERG_URL", "http://localhost:80"),
		jsonServerURL: getConfig(jsonServerURL, "JSON_SERVER_URL", "http://localhost:3002"),
		reactAppURL:   getConfig(reactAppURL, "REACT_APP_URL", "http://host.docker.internal:3000"),
		openAiApiKey:  getConfig(openAiApiKey, "OPENAI_API_KEY", ""),
		fsType:        getConfig(fstype, "FSTYPE", "gcs"),
		gcsBucket:     getConfig(gcsBucket, "GCS_BUCKET", "my-stinky-bucket"),
		localPath:     getConfig(localPath, "LOCAL_PATH", "output"),
		mode:          getConfig(mode, "MODE", "server"), // Default to "server"
		useSystemGs:   getConfigBool(useSystemGs, "USE_SYSTEM_GS", false),
	}

	//Validation
	if config.fsType == "gcs" && config.gcsBucket == "" {
		log.Fatal("GCS bucket name must be specified for GCS filesystem")
	}

	if config.fsType == "local" && config.localPath == "" {
		log.Fatal("Local path must be specified for local filesystem")
	}
	if config.openAiApiKey == "" {
		log.Fatal("An Open AI (what a misnomer lol) API Key is required for the server to be able to do anything interesting.")
	}

	return config
}

func oldMain(fs filesystem.FileSystem, config *serviceConfig) {

	// Example call to ReadInput
	inputDir := "input" // The directory where jd.txt and expect_response.json are located

	//baseline := "functional"
	baseline := "chrono"
	//baseline := "resumeproject"
	//baseline := "retailchrono"
	outputDir := "output"
	styleOverride := "fluffy"
	//styleOverride := ""

	baselineJSON, err := getBaselineJSON(baseline, config)
	if err != nil {
		log.Fatalf("error from reading baseline JSON: %v", err)
	}
	layout, style, err := getLayoutFromBaselineJSON(baselineJSON)
	if err != nil {
		log.Fatalf("error from extracting layout from baseline JSON: %v", err)
	}
	if styleOverride != "" {
		style = styleOverride
	}

	input, err := ReadInput(inputDir)
	if err != nil {
		log.Fatalf("Error reading input files: %v", err)
	}

	input.ExpectResponseSchema, err = getExpectedResponseJsonSchema(layout)
	if err != nil {
		log.Println("error from getExpectedResponseJsonSchema: ", err)
		return
	}

	mainPrompt, err := getInputPrompt(inputDir)
	if err != nil {
		log.Println("error from reading input prompt: ", err)
		return
	}
	if mainPrompt == "" {
		mainPrompt, err = getDefaultPrompt(layout)
		if err != nil {
			log.Println("error from reading input prompt: ", err)
			return
		}
	}

	//todo: fix this calls arguments it should probably just be one struct.
	err = tuneResumeContents(input, mainPrompt, baselineJSON, layout, style, outputDir, acceptableRatio, maxAttempts, fs, config, &Job{})
	if err != nil {
		log.Fatalf("Error from resume tuning: %v", err)
	}
}

// worker simulates a worker that processes jobs.
func worker(config *serviceConfig) {
	for job := range jobQueue {
		//mu.Lock()
		//jobID := jobIDCounter
		//jobIDCounter++
		//mu.Unlock()
		log.Printf("do something with this job: %#v", job)

		baselineJSON := job.BaselineJSON //i think really this is where it should come from
		var err error
		if baselineJSON == "" {
			//but for some testing (and the initial implementation, predating the json server even, where my personal info was baked into the react project ... anyway, that got moved to the json server and some variants got names. but that should all get deprecated i think)
			baselineJSON, err = getBaselineJSON(job.Baseline, config)
			if err != nil {
				log.Fatalf("error from reading baseline JSON: %v", err)
			}
		}

		layout, style, err := getLayoutFromBaselineJSON(baselineJSON)
		if err != nil {
			log.Fatalf("error from extracting layout from baseline JSON: %v", err)
		}
		if job.StyleOverride != "" {
			style = job.StyleOverride
		}

		expectResponseSchema, err := getExpectedResponseJsonSchema(layout)
		//todo refactor this stuff lol
		inputTemp := &Input{
			JD:                   job.JobDescription,
			ExpectResponseSchema: expectResponseSchema,
			APIKey:               config.openAiApiKey,
		}
		if err != nil {
			log.Fatalf("Error reading input files: %v", err)
		}

		mainPrompt := job.CustomPrompt
		if mainPrompt == "" {
			mainPrompt, err = getDefaultPrompt(layout)
			if err != nil {
				log.Println("error from reading input prompt: ", err)
				return
			}
		}

		//todo: fix this calls arguments it should probably just be one struct.
		err = tuneResumeContents(inputTemp, mainPrompt, baselineJSON, layout, style, job.Id.String(), acceptableRatio, maxAttempts, fs, config, &job)
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
}

// handleJobSubmission handles incoming job submissions via POST.
func handleJobSubmission(w http.ResponseWriter, r *http.Request) {
	log.Println("here in handleJobSubmission")
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var job Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}
	log.Printf("here in handleJobSubmission with job like: %#v", job)

	//// Add job to queue
	//mu.Lock()
	//jobID := jobIDCounter
	//jobIDCounter++
	//mu.Unlock()
	job.Id = uuid.New()
	jobQueue <- job

	// Respond with a job URL
	jobURL := fmt.Sprintf("/checkjob/%d", 12345)
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf("Job submitted successfully. Poll job status at: %s", jobURL)))
}

// handleJobStatus handles checking job status via GET.
func handleJobStatus(w http.ResponseWriter, r *http.Request) {
	jobIDStr := r.URL.Path[len("/checkjob/"):]
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	_ = jobID
	log.Printf("NYI: checking of job results via http")
	jobResult := &JobResult{}
	//jobResult, err := fs.ReadFile(jobID)
	//if err != nil {
	//	http.Error(w, "Job not found", http.StatusNotFound)
	//	return
	//}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobResult)
}

// parseFlags parses the command line arguments.
// func parseFlags() (string, string) {
func parseFlags() string {
	mode := flag.String("mode", "server", "Mode: either 'server' or 'cli'")
	//fsType := flag.String("fs", "local", "Filesystem type: 'local' or 'gcs'")
	//localPath := flag.String("local-path", "./jobs", "Local file path for job storage (only for 'local' filesystem)")
	//gcsBucket := flag.String("gcs-bucket", "", "GCS bucket name for job storage (only for 'gcs' filesystem)")

	flag.Parse()

	// Validation
	//if *fsType == "gcs" && *gcsBucket == "" {
	//	log.Fatal("GCS bucket name must be specified for GCS filesystem")
	//}
	//
	//if *fsType == "local" && *localPath == "" {
	//	log.Fatal("Local path must be specified for local filesystem")
	//}
	//
	//return *mode, *fsType
	return *mode
}

// configureFilesystem sets up the filesystem based on the command line flags.
func configureFilesystem(config *serviceConfig) filesystem.FileSystem {
	if config.fsType == "local" {
		return &filesystem.LocalFileSystem{BasePath: config.localPath}
	} else if config.fsType == "gcs" {
		// Create a new GCS client
		log.Printf("setting up gcs client ...")
		ctx := context.Background()
		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create GCS client: %v", err)
		}
		if config.gcsBucket == "" {
			log.Fatal("gcs-bucket arg needs to have a value")
		}
		return &filesystem.GCSFileSystem{Client: client, BucketName: config.gcsBucket}
	}
	return nil
}

func writeAttemptResumedataJSON(content, layout, style, outputDir string, attemptNum int, fs filesystem.FileSystem, config *serviceConfig) error {
	// Step 5: Write the validated content to the filesystem in a way the resume projects json server can read it, plus locally for posterity.
	// Assuming the file path is up and outside of the project directory
	// Example: /home/user/output/validated_content.json
	updatedContent, err := insertLayout(content, layout, style)
	if err != nil {
		log.Printf("Error inserting layout info: %v\n", err)
		return err
	}

	//TODO !!!!!!!! dont do this!!!!!! not like this!!!!
	if config.fsType == "local" {
		// this is/was just a cheesy way to get the attempted resume updated json available to the react project via a local json server service.
		outputFilePath := filepath.Join("../ResumeData/resumedata/", fmt.Sprintf("attempt%d.json", attemptNum))
		err = writeValidatedContent(updatedContent, outputFilePath)
		if err != nil {
			log.Printf("Error writing content to file: %v\n", err)
			return err
		}
		log.Println("Content successfully written to:", outputFilePath)
	} else if config.fsType == "gcs" {
		outputFilePath := fmt.Sprintf("%s/attempt%d.json", outputDir, attemptNum)
		log.Printf("writeAttemptResumedataJSON to GCS bucket, path: %s", outputFilePath)
		fs.WriteFile(outputFilePath, []byte(content))
		log.Printf("writeAttemptResumedataJSON thinks it got past that.")
	}

	// Example: /home/user/output/validated_content.json
	localOutfilePath := filepath.Join(outputDir, fmt.Sprintf("attempt%d.json", attemptNum))
	err = writeValidatedContent(updatedContent, localOutfilePath)
	if err != nil {
		log.Printf("Error writing content to file: %v\n", err)
		return err
	}
	log.Println("Content successfully written to:", localOutfilePath)
	return nil
}

func insertLayout(content string, layout string, style string) (string, error) {
	// Step 1: Deserialize the JSON content into a map
	var data map[string]interface{}
	err := json.Unmarshal([]byte(content), &data)
	if err != nil {
		return "", err
	}

	// Step 2: Insert the layout into the map
	data["layout"] = layout

	if style != "" {
		data["style"] = style
	}

	// Step 3: Reserialize the map back into a JSON string
	updatedContent, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// Step 4: Return the updated JSON string
	return string(updatedContent), nil
}

func getBaselineJSON(baseline string, config *serviceConfig) (string, error) {
	// get JSON of the current complete resume including all the hidden stuff, this hits an express server that imports the reactresume resumedata.mjs and outputs it as json.
	jsonRequestURL := fmt.Sprintf("%s?baseline=%s", config.jsonServerURL, baseline)
	resp, err := http.Get(jsonRequestURL)
	if err != nil {
		log.Fatalf("Failed to make the HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the response body: %v", err)
	}
	log.Printf("got %d bytes of json from the json-server via %s\n", len(body), jsonRequestURL)
	return string(body), nil
}

func getLayoutFromBaselineJSON(baselineJSON string) (string, string, error) {
	//if i want anything else beyond layout and style i should return a struct because this is ugly.

	//log.Println("dbg baselinejson", baselineJSON)
	var decoded map[string]interface{}
	err := json.Unmarshal([]byte(baselineJSON), &decoded)
	if err != nil {
		return "", "", err
	}

	// Check if the "layout" key exists and is a string
	layout, ok := decoded["layout"].(string)
	if !ok {
		return "", "", errors.New("layout is missing or not a string")
	}
	// Check if the "style" key exists and is a string (its ok if its not there but if it is we should keep it)
	style, _ := decoded["style"].(string)

	return layout, style, nil
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

func takeNotesOnJD(input *Input, outputDir string) (string, error) {
	//JDResponseFormat, err := os.ReadFile(filepath.Join("response_templates", "jdinfo.json"))
	jDResponseSchemaRaw, err := os.ReadFile(filepath.Join("response_templates", "jdinfo-schema.json"))
	if err != nil {
		log.Fatalf("failed to read expect_response.json: %v", err)
	}
	// Validate the JSON content
	jDResponseSchema, err := decodeJSON(string(jDResponseSchemaRaw))
	if err != nil {
		log.Fatalf("failed to decode JSON: %v", err)
	}
	//if err := validateJSON(string(jDResponseSchemaRaw)); err != nil {
	//	return err
	//}
	prompt := strings.Join([]string{
		"Extract information from the following Job Description. Take note of the name of the company, the job title, and most importantly the list of key words that a candidate will have in their CV in order to get through initial screening. Additionally, extract any location, remote-ok status, salary info and hiring process notes which can be succinctly captured.",
		"\n--- start job description ---\n",
		input.JD,
		"\n--- end job description ---\n",
	}, "")

	apirequest := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "You are a Job Description info extractor assistant.",
			},
			//{
			//	"role":    "system",
			//	"content": "You are a Job Description info extractor assistant. The response should include only the fields of the provided JSON example, in well-formed JSON, without any triple quoting, such that your responses can be ingested directly into an information system.",
			//},
			//{
			//	"role":    "user",
			//	"content": "Show me an example input for the Job Description information system to ingest",
			//},
			//{
			//	"role":    "assistant",
			//	"content": JDResponseFormat,
			//},
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
		//"max_tokens":  100,
		"temperature": 0.7,
	}
	api_request_pretty, err := serializeToJSON(apirequest)
	writeToFile(api_request_pretty, 0, "jd_info_request_pretty", outputDir)
	if err != nil {
		log.Fatalf("Failed to marshal final JSON: %v", err)
	}

	exists, output, err := checkForPreexistingAPIOutput(outputDir, "jd_info_response_raw", 0)
	if err != nil {
		log.Fatalf("Error checking for pre-existing API output: %v", err)
	}
	if !exists {
		output, err = makeAPIRequest(apirequest, input.APIKey, 0, "jd_info_response_raw", outputDir)
		if err != nil {
			log.Fatalf("Error making API request: %v", err)
		}
	}

	//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
	var apiResponse APIResponse
	err = json.Unmarshal([]byte(output), &apiResponse)
	if err != nil {
		log.Fatalf("Error deserializing API response: %v\n", err)
	}

	//Extract the message content
	if len(apiResponse.Choices) == 0 {
		log.Fatalf("No choices found in the API response")
	}

	content := apiResponse.Choices[0].Message.Content

	err = validateJSON(content)
	if err != nil {
		log.Fatalf("Error validating JSON content: %v\n", err)
	}
	log.Printf("Got %d bytes of JSON content about the JD (at least well formed enough to be decodable) out of that last response\n", len(content))

	outputFilePath := filepath.Join(outputDir, "jdinfo-out.json")
	err = writeValidatedContent(content, outputFilePath)
	if err != nil {
		log.Fatalf("Error writing content to file: %v\n", err)
	}
	log.Println("JD Info Content successfully written to:", outputFilePath)
	return content, nil
}

func getInputPrompt(directory string) (string, error) {
	// Construct the filename
	filepath := filepath.Join(directory, "prompt.txt")

	// Check if the file exists
	_, err := os.Stat(filepath)
	if err == nil {
		// File exists, read its contents
		data, err := os.ReadFile(filepath)
		if err != nil {
			return "", fmt.Errorf("prompt file existed but failed to read it from file system??%v", err)
		}
		return string(data), nil
	}
	return "", nil
}

func getDefaultPrompt(layout string) (string, error) {
	log.Printf("No (or empty) prompt from the file system so will use a default one.")

	//prompt := "Provide feedback on the following JSON. Is it well formed? What do you think the purpose is? Tell me about things marked as hide and what that might mean. Finally, how long in terms of page count do you think the final document this feeds into is?\n\nJSON: "

	//prompt_parts := []string{
	//	"This guy needs a job ASAP. You need to make his resume look PERFECT for the job. Fake it until you make it right? Fix it up, do what it takes. Aim for 3-5 companies, each with 2-4 projects. Make them relate to the Job Description as best as possible, including possibly switching up industries and industry terms. ",
	//	"Feel free to dig into those hidden companies and projects for inspiration, include whatever you think could be relevant. ",
	//	"Do not claim to have worked at the target company from the Job Description unless the input resume data structure JSON from the candidate actually claims to have already worked there before. ",
	//	"The target Job Description for which this candidate should appear to perfectly match is below. Pay special attention to any magic tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability (magic tokens should be placed somewhere that will get rendered such as a project description or a job title):\n",
	//	"\n--- start job description ---\n",
	//	input.JD,
	//	"\n--- end job description ---\n",
	//	"The following JSON represents the current generic 1 page resume for the candidate. Much of the information in the data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description above. ",
	//	"The input resume data structure JSON is below:\n",
	//	string(body),
	//}

	//prompt_parts := []string{
	//	"The hiring industry is broken. We are not taking this seriously. Reformat the resume and fill it up with goofy nonsense that makes fun of recruiting and hiring in general as well as the whole hiring industry. Fill it with metrics about job losses, bad candidates, jobs that dont exist and wasted hours on take home assignments. ",
	//	"Ostensibly, the job that's being targeted is described in the Job Description below, but the details aren't as important as the overall cynicism and irony:\n",
	//	"\n--- start job description ---\n",
	//	input.JD,
	//	"\n--- end job description ---\n",
	//	"The following JSON represents the current generic 1 page resume for the candidate. Much of the information in the data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more cynically suitable for the Job Description above. ",
	//	"The input resume data structure JSON is below:\n",
	//	string(body),
	//}

	//common := //todo if having to change things that are the same perhaps

	//because the different layouts are going to be structured different i just want to refer to the sections by what they are, so slight nuance depending on the layout. annoying yes, and maybe not neccessary idk. perhaps a more unified way of talking about the work history could be done
	m := map[string]string{
		"chrono": strings.Join([]string{
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
		"functional": strings.Join([]string{
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
	}
	value, ok := m[layout]
	if !ok {
		return "", errors.New("layout prompt not found: " + layout)
	}
	return value, nil
}

// Struct to hold the inspection results
type inspectResult struct {
	NumberOfPages        int
	LastPageContentRatio float64
}

// inspectPNGFiles counts all the PNG files in the output directory and calculates the content ratio of the last page
func inspectPNGFiles(outputDir string, attempt int) (inspectResult, error) {
	result := inspectResult{}

	// Read the files in the output directory
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return result, fmt.Errorf("failed to read output directory: %v", err)
	}

	// Filter and collect PNG files
	var pngFiles []string
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), fmt.Sprintf("out%d-", attempt)) {
			continue
		}
		if strings.HasSuffix(file.Name(), ".png") {
			pngFiles = append(pngFiles, filepath.Join(outputDir, file.Name()))
		}
	}

	// Sort the PNG files alphanumerically
	sort.Strings(pngFiles)
	log.Printf("will be treating the last of these files in this list as the last page to look at: %v\n", pngFiles)

	// If no PNG files were found, return the result with zero values
	if len(pngFiles) == 0 {
		return result, nil
	}

	// Update the number of pages in the result
	result.NumberOfPages = len(pngFiles)

	// Calculate the content ratio of the last PNG file
	lastPage := pngFiles[len(pngFiles)-1]

	img, err := imaging.Open(lastPage)
	if err != nil {
		log.Fatalf("Failed to open image: %v", err)
	}

	result.LastPageContentRatio = contentRatio(img)

	return result, nil
}

func dumpPDFToPNG(attempt int, outputDir string, config *serviceConfig) error {
	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting current directory: %v\n", err)
	}

	// Construct the output directory path
	outputDirFullpath := filepath.Join(currentDir, outputDir)

	//could maybe check the pdf for not containing error stuff like "Uncaught runtime errors" before proceeding.
	// MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd)/output:/workspace minidocks/ghostscript:latest gs -sDEVICE=pngalpha -o /workspace/out-%03d.png -r144 /workspace/attempt.pdf
	var cmd *exec.Cmd
	if config.useSystemGs {
		cmd = exec.Command(
			"gs",
			"-sDEVICE=txtwrite",
			"-o", filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"),
			filepath.Join(outputDirFullpath, fmt.Sprintf("attempt%d.pdf", attempt)),
		)
	} else {
		cmd = exec.Command("docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/workspace", outputDirFullpath),
			"minidocks/ghostscript:latest",
			"gs",
			"-sDEVICE=txtwrite",
			"-o", "/workspace/pdf-txtwrite.txt",
			fmt.Sprintf("/workspace/attempt%d.pdf", attempt),
		)
	}
	log.Printf("dump pdf to txt with gs command: %s", strings.Join(cmd.Args, " "))
	log.Println("About to check the pdf text to confirm no errors")
	// Run the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Error running docker command: %v\n", err)
	}

	data, err := os.ReadFile(filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"))
	if err != nil {
		return fmt.Errorf("error reading pdf txt output %v", err)
	}
	if strings.Contains(string(data), "Uncaught runtime errors") {
		return fmt.Errorf("'Uncaught runtime errors' string detected in PDF contents.")
	}

	if config.useSystemGs {
		cmd = exec.Command(
			"gs",
			"-sDEVICE=pngalpha",
			"-o", filepath.Join(outputDirFullpath, fmt.Sprintf("out%d-%%03d.png", attempt)),
			"-r144",
			filepath.Join(outputDirFullpath, fmt.Sprintf("attempt%d.pdf", attempt)),
		)
	} else {
		// dump pdf to png files, one per page, 144ppi
		cmd = exec.Command("docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/workspace", outputDirFullpath),
			"minidocks/ghostscript:latest",
			"gs",
			"-sDEVICE=pngalpha",
			"-o", fmt.Sprintf("/workspace/out%d-%%03d.png", attempt),
			"-r144",
			fmt.Sprintf("/workspace/attempt%d.pdf", attempt),
		)
	}
	log.Printf("dump pdf to png with gs command: %s", strings.Join(cmd.Args, " "))

	// Capture the output into a byte buffer
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &outBuffer

	// Run the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Error running docker command: %v\n", err)
	}

	// Grab some fun stuff from the logging (or throw an error if fun stuff not found)
	// Convert the output to a string
	output := outBuffer.String()

	// Check for specific strings in the output
	if strings.Contains(output, "Processing pages") {
		// Extract the range of pages being processed
		lines := strings.Split(output, "\n")
		var processingLine string
		var pageLines []string
		for _, line := range lines {
			if strings.Contains(line, "Processing pages") {
				processingLine = line
			} else if strings.HasPrefix(line, "Page ") {
				pageLines = append(pageLines, strings.TrimPrefix(line, "Page "))
			}
		}
		if processingLine != "" && len(pageLines) > 0 {
			// Log the relevant information
			log.Println("Rendered pages", strings.Join(pageLines, ", "), "as PNG files")
		}
	} else {
		return fmt.Errorf("No page processing detected in command output.")
	}
	return nil
}

func makePDFRequestAndSave(attempt int, layout, outputDir string, config *serviceConfig, job *Job) error {
	// Step 1: Create a new buffer and a multipart writer
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Step 2: Add the "url" field to the multipart form
	urlField, err := writer.CreateFormField("url")
	if err != nil {
		return fmt.Errorf("failed to create form field: %v", err)
	}

	var urlToRender string
	if config.fsType == "gcs" {
		//for server mode with gcs data we need to make sure we pass jsonserver with a hostname value (no https:// prefix), and make sure that the uuid and a slash (uri escaped) get prepended to the attemptN value.
		jsonServerHostname, err := extractHostname(config.jsonServerURL)
		if err != nil {
			return err
		}
		jsonPathFragment := url.PathEscape(fmt.Sprintf("%s/attempt%d", job.Id.String(), attempt))
		fmt.Sprintf("%s/attempt%d", job.Id.String(), attempt)
		urlToRender = fmt.Sprintf("%s/?jsonserver=%s&resumedata=%s&layout=%s", config.reactAppURL, jsonServerHostname, jsonPathFragment, layout)
	} else {
		//legacy way, presumably json server is on local host or smth.
		urlToRender = fmt.Sprintf("%s/?resumedata=attempt%d&layout=%s", config.reactAppURL, attempt, layout)
	}
	_, err = io.WriteString(urlField, urlToRender)
	if err != nil {
		return fmt.Errorf("failed to write to form field: %v", err)
	}

	// Step 3: Close the multipart writer to finalize the form data
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Step 4: Create a new POST request with the multipart form data
	gotenbergRequestURL := fmt.Sprintf("%s/forms/chromium/convert/url", config.gotenbergURL)
	req, err := http.NewRequest("POST", gotenbergRequestURL, &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Step 5: Set the Content-Type header
	req.Header.Set("Content-Type", writer.FormDataContentType())

	log.Printf("Will ask gotenberg at %s to render page at %s", gotenbergRequestURL, urlToRender)
	// Step 6: Send the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Step 7: Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Step 8: Write the response body (PDF) to the output file
	outputFilePath := filepath.Join(outputDir, fmt.Sprintf("attempt%d.pdf", attempt))

	// Create the output directory if it doesn't exist
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create the output file
	file, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Copy the response body to the output file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write PDF to file: %v", err)
	}

	log.Printf("PDF saved to %s\n", outputFilePath)

	return nil
}

func extractHostname(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Extract the hostname without protocol and trailing slash
	hostname := strings.TrimSuffix(parsedURL.Host, "/")
	return hostname, nil
}

func checkForPreexistingAPIOutput(directory, filenameFragment string, counter int) (bool, string, error) {
	// Construct the filename
	filename := fmt.Sprintf("%s_%d.txt", filenameFragment, counter)
	filepath := filepath.Join(directory, filename)

	// Check if the file exists
	if _, err := os.Stat(filepath); err == nil {
		// File exists, read its contents
		data, err := os.ReadFile(filepath)
		if err != nil {
			return true, "", fmt.Errorf("failed to read existing API output: %v", err)
		}
		log.Printf("Read prior response for api request attempt number %d from file system.\n", counter)
		return true, string(data), nil
	} else if os.IsNotExist(err) {
		// File does not exist
		log.Printf("No prior response found for api request attempt number %d in file system.\n", counter)
		return false, "", nil
	} else {
		// Some other error occurred
		log.Println("Error while checking file system for prior api response info.")
		return false, "", fmt.Errorf("error checking file existence: %v", err)
	}
}

func makeAPIRequest(apiBody interface{}, apiKey string, counter int, name, outputDir string) (string, error) {
	//panic("slow down there son, you really want to hit the paid api at this time?")
	log.Printf("Make request to OpenAI ...")
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
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

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
	log.Printf("Got response from OpenAI API ... (and should have wrote it to the file system)")

	// Return the response string
	return responseString, nil
}

// writeToFile writes data to a file in the output directory with a filename based on the counter and fragment
func writeToFile(data string, counter int, filenameFragment, outputDir string) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Construct the filename
	filename := fmt.Sprintf("%s_%d.txt", filenameFragment, counter)
	filepath := filepath.Join(outputDir, filename)

	// Write the data to the file
	if err := os.WriteFile(filepath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write to file: %v", err)
	}

	return nil
}

// writeValidatedContent writes the validated content to a specific file path
func writeValidatedContent(content, filePath string) error {
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write content to file: %v", err)
	}
	return nil
}

// serializeToJSON takes an interface, serializes it to pretty-printed JSON, and returns it as a string
func serializeToJSON(v interface{}) (string, error) {
	// Marshal the interface to pretty-printed JSON
	jsonData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize to JSON: %v", err)
	}
	return string(jsonData), nil
}

// APIResponse Struct to represent (parts of) the API response (that we care about r/n)
type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Input struct to hold the contents of jd.txt and expect_response.json
type Input struct {
	InputDir             string
	JD                   string
	ExpectResponseSchema interface{}
	APIKey               string
}

// ReadInput reads the input files from the "input" directory and returns an Input struct
func ReadInput(dir string) (*Input, error) {
	// Define file paths
	jdFilePath := filepath.Join(dir, "jd.txt")
	apiKeyFilePath := filepath.Join(dir, "api_key.txt")

	// Read jd.txt
	jdContent, err := os.ReadFile(jdFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read jd.txt: %v", err)
	}

	// Retrieve the API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// If the environment variable is not set, try to read it from api_key.txt
		apiKeyContent, err := os.ReadFile(apiKeyFilePath)
		if err != nil {
			return nil, fmt.Errorf("API key not found in environment variable or api_key.txt: %v", err)
		}
		apiKey = string(apiKeyContent)
		log.Println("Got API Key from input text file")
	} else {
		log.Println("Got API Key from env var")
	}

	// Return the populated Input struct
	return &Input{
		InputDir: dir,
		JD:       string(jdContent),
		//ExpectResponse: string(expectResponseContent),
		//ExpectResponseSchema: expectResponseSchema,
		APIKey: apiKey,
	}, nil
}

func getExpectedResponseJsonSchema(layout string) (interface{}, error) {
	expectResponseFilePath := filepath.Join("response_templates", fmt.Sprintf("%s-schema.json", layout))
	// Read expect_response.json
	expectResponseContent, err := os.ReadFile(expectResponseFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read expect_response.json: %v", err)
	}
	// Validate the JSON content
	expectResponseSchema, err := decodeJSON(string(expectResponseContent))
	if err != nil {
		return nil, err
	}
	return expectResponseSchema, nil
}

// validateJSON checks if a string contains valid JSON
func validateJSON(data string) error {
	var js json.RawMessage //voodoo -- apparently even though its []byte .... thats ok? we can even re-unmarshal it to an actual type later? this was suggested to me for simple json decode verification, and it works. so *shrugs*
	if err := json.Unmarshal([]byte(data), &js); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}
	return nil
}

// decodeJSON takes a JSON string and returns a deserialized object as an interface{}.
func decodeJSON(data string) (interface{}, error) {
	var js json.RawMessage

	// Unmarshal the JSON string into json.RawMessage to verify its validity
	if err := json.Unmarshal([]byte(data), &js); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}

	// If the JSON is valid, you can return it as an interface{}
	var result interface{}
	if err := json.Unmarshal(js, &result); err != nil {
		return nil, fmt.Errorf("error decoding JSON into interface{}: %v", err)
	}

	// Return the deserialized object
	return result, nil
}

// cleanAndValidateJSON takes MJS file contents as a string, strips non-JSON content
// on the first line, removes double-slash comment lines, and validates the resulting JSON.
func cleanAndValidateJSON(mjsContent string) (string, error) {
	lines := strings.Split(mjsContent, "\n")

	// Step 1: Remove lines that contain double-slash comments
	var cleanedLines []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if len(trimmedLine) == 0 {
			continue
		}
		if strings.HasPrefix(trimmedLine, "//") {
			continue
		} else if commentIndex := findCommentIndex(line); commentIndex != -1 {
			// Take the part of the line before the comment and add it to cleanedLines
			cleanedLines = append(cleanedLines, line[:commentIndex])
		} else {
			cleanedLines = append(cleanedLines, line)
		}
	}

	// Step 2: Process the first line to strip non-JSON content
	if len(lines) > 0 {
		firstLine := cleanedLines[0]
		// Use strings.Index to find the first occurrence of '{' and remove everything before it
		if index := strings.Index(firstLine, "{"); index != -1 {
			cleanedLines[0] = firstLine[index:]
		} else {
			return "", fmt.Errorf("no JSON object found on the first line (line looks like: '%s')", firstLine)
		}
	}

	// Step 3: Join the cleaned lines back into a single string
	cleanedJSON := strings.Join(cleanedLines, "\n")
	fmt.Printf("working with :\n\n'%s'", cleanedJSON)
	//panic("stop wut")

	// Step 4: Validate the resulting string as JSON
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedJSON), &js); err != nil {
		return "", fmt.Errorf("invalid JSON: %v", err)
	}

	// Step 5: Return the cleaned and validated JSON string
	return cleanedJSON, nil
}

// findCommentIndex finds the index of "//" that is not within a string
func findCommentIndex(line string) int {
	inString := false
	for i := 0; i < len(line)-1; i++ {
		if line[i] == '"' {
			inString = !inString
		}
		if !inString && line[i] == '/' && line[i+1] == '/' {
			return i
		}
	}
	return -1
}

func contentRatio(img image.Image) float64 {
	// Get image dimensions
	bounds := img.Bounds()
	//totalPixels := bounds.Dx() * bounds.Dy()

	lastColoredPixelRow := 0
	log.Printf("believe image dimensions are: %d x %d\n", bounds.Max.X, bounds.Max.Y)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		//fmt.Printf("checking row %d\n", y)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {

			r, g, b, a := img.At(x, y).RGBA()
			//fmt.Printf("x: %d, y: %d, colorcode: %d %d %d (alpha: %d)\n", x, y, r, g, b, a)
			if isColored(r, g, b, a) {
				lastColoredPixelRow = y + 1
				//fmt.Printf("Found a nonwhite/nontransparent pixel on row %d in column %d\n", y, x)
				break //no need to check any further along this line.
			}
		}
	}
	log.Printf("lastrow found a pixel on: %v, total rows was %v\n", lastColoredPixelRow, bounds.Max.Y)
	lastContentAt := float64(lastColoredPixelRow) / float64(bounds.Max.Y)
	log.Printf("last content found at %.5f of the document.", lastContentAt)
	return lastContentAt
}

func whitePct(img image.Image) float64 {
	// Get image dimensions
	bounds := img.Bounds()
	totalPixels := bounds.Dx() * bounds.Dy()

	// Count white pixels
	whitePixels := 0
	pixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			pixels++
			if isWhite(r, g, b, a) {
				whitePixels++
			}
		}
	}

	// Calculate percentage of white space
	whiteSpacePercentage := (float64(whitePixels) / float64(totalPixels)) * 100

	fmt.Printf("White space percentage: %.2f%%\n", whiteSpacePercentage)
	fmt.Printf("Checked pixels: : %d\n", pixels)

	return whiteSpacePercentage
}

func isColored(r, g, b, a uint32) bool {
	const color_threshold = 0.9 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
	const alpha_threshold = 0.1 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
	//also .... transparent with no color == white when on screen as a pdf so ....
	//return a > 0 && float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
	//if r == 0 && g == 0 && b == 0 && a == 0 {
	//	return false
	//}
	return float64(a) > alpha_threshold && float64(r) <= color_threshold && float64(g) <= color_threshold && float64(b) <= color_threshold
}

func isWhite(r, g, b, a uint32) bool {
	const threshold = 0.9 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
	//also .... transparent with no color == white when on screen as a pdf so ....
	//return a > 0 && float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
	return a == 0 || float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
}
