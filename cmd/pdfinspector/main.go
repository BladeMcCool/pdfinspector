package main

import (
	"fmt"
	_ "image/png" // Importing PNG support
	"log"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	jobPackage "pdfinspector/pkg/job"
	"pdfinspector/pkg/server"
	"pdfinspector/pkg/tuner"
)

// Global variables for job management
//var (
//	jobQueue = make(chan Job, 100) // Queue to hold jobs
//	//jobIDCounter = 1
//	//mu           sync.Mutex
//	//fs filesystem.FileSystem // Filesystem interface to handle storage
//
//	//this stuff that should probably be cli overridable at least, todo.
//)

// main function: Either runs the web server or executes the main functionality.
func main() {
	// Get the service configuration
	config := config.GetServiceConfig()

	//mode := parseFlags()
	//log.Printf("mode: %s, fsType: %s", mode, fsType)
	//panic("stop for now")

	//gotenbergURL := getEnv("GOTENBERG_URL", "http://localhost:3000")
	//jsonServerURL := getEnv("JSON_SERVER_URL", "http://localhost:4000")
	//reactAppURL := getEnv("REACT_APP_URL", "http://localhost:5000")
	//gcsBucket := getEnv("GCS_BUCKET", "my-stinky-bucket")

	// Configure filesystem
	//localPath := flag.Lookup("local-path").Value.(flag.Getter).Get().(string)
	//_ = localPath
	//gcsBucket := flag.Lookup("gcs-bucket").Value.(flag.Getter).Get().(string)
	//fs = configureFilesystem(config)

	if config.Mode == "cli" {
		// CLI execution mode
		fmt.Println("Running main functionality from CLI...")
		cliRunJob(config)
		//cliRunJob(fs, config)
		// Replace the following with your main functionality
		fmt.Println("Finished Executing main functionality via CLI")
		return
	}

	// Start worker
	//go worker(config)

	// Web server mode
	server := server.NewPdfInspectorServer(config)
	server.RunServer()

	//http.HandleFunc("/submitjob", handleJobSubmission)
	//http.HandleFunc("/checkjob/", handleJobStatus)
	//http.HandleFunc("/runjob", handJobRun(config))
	//http.HandleFunc("/streamjob", handleStreamJob(config))
	//http.HandleFunc("/joboutput/", handleJobOutput(config))
	//
	//fmt.Println("Starting server on port 8080...")
	//if err := http.ListenAndServe(":8080", nil); err != nil {
	//	log.Fatalf("Server failed: %v", err)
	//}
}

func cliRunJob(config *config.ServiceConfig) {

	// Example call to ReadInput
	inputDir := "input" // The directory where jd.txt and expect_response.json are located

	//baseline := "functional"
	baseline := "chrono"
	//baseline := "resumeproject"
	//baseline := "retailchrono"
	//outputDir := "outputs"
	styleOverride := "fluffy"
	//styleOverride := ""

	t := tuner.NewTuner(config)
	baselineJSON, err := t.GetBaselineJSON(baseline)
	if err != nil {
		log.Fatalf("error from reading baseline JSON: %v", err)
	}
	layout, style, err := t.GetLayoutFromBaselineJSON(baselineJSON)
	if err != nil {
		log.Fatalf("error from extracting layout from baseline JSON: %v", err)
	}
	if styleOverride != "" {
		style = styleOverride
	}

	input, err := jobPackage.ReadInput(inputDir)
	if err != nil {
		log.Fatalf("Error reading input files: %v", err)
	}
	if input.APIKey != "" {
		config.OpenAiApiKey = input.APIKey
	}

	//input.ExpectResponseSchema, err = t.GetExpectedResponseJsonSchema(layout)
	//if err != nil {
	//	log.Println("error from getExpectedResponseJsonSchema: ", err)
	//	return
	//}

	mainPrompt, err := getInputPrompt(inputDir)
	if err != nil {
		log.Println("error from reading input prompt: ", err)
		return
	}
	if mainPrompt == "" {
		mainPrompt, err = t.GetDefaultPrompt(layout)
		if err != nil {
			log.Println("error from reading input prompt: ", err)
			return
		}
	}

	// this func could possibly leverage tuner.PopulateJob but doing that also might be annoying. tbd.
	job := jobPackage.NewDefaultJob()
	job.JobDescription = input.JD
	job.Layout = layout
	job.Style = style
	job.MainPrompt = mainPrompt
	job.BaselineJSON = baselineJSON
	//job.OutputDir = filepath.Join(config.LocalPath, job.Id.String()) //should not use this for things that end up on gcs from a windows machine b/c it gets a backslash. idk probably should have local and gcs dirs saved separately so local can use local path sep and gcs always use forward slash.
	job.OutputDir = fmt.Sprintf("%s/%s", config.LocalPath, job.Id.String())

	err = t.TuneResumeContents(job, nil)
	if err != nil {
		log.Fatalf("Error from resume tuning: %v", err)
	}
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

// worker simulates a worker that processes jobs.
//func worker(config *serviceConfig) {
//	runner := &jobRunner{
//		config: config,
//	}
//	for job := range jobQueue {
//		runner.RunJob(&job, nil)
//	}
//}

//
//func runJob(job *Job, config *serviceConfig, updates chan JobStatus) {
//	defer close(updates)
//	//mu.Lock()
//	//jobID := jobIDCounter
//	//jobIDCounter++
//	//mu.Unlock()
//	log.Printf("do something with this job: %#v", job)
//
//	baselineJSON := job.BaselineJSON //i think really this is where it should come from
//	var err error
//	if baselineJSON == "" {
//		//but for some testing (and the initial implementation, predating the json server even, where my personal info was baked into the react project ... anyway, that got moved to the json server and some variants got names. but that should all get deprecated i think)
//		baselineJSON, err = getBaselineJSON(job.Baseline, config)
//		if err != nil {
//			log.Fatalf("error from reading baseline JSON: %v", err)
//		}
//	}
//	sendJobUpdate(updates, "got baseline JSON")
//
//	layout, style, err := getLayoutFromBaselineJSON(baselineJSON)
//	if err != nil {
//		log.Fatalf("error from extracting layout from baseline JSON: %v", err)
//	}
//	if job.StyleOverride != "" {
//		style = job.StyleOverride
//	}
//
//	expectResponseSchema, err := getExpectedResponseJsonSchema(layout)
//	//todo refactor this stuff lol
//	inputTemp := &Input{
//		JD:                   job.JobDescription,
//		ExpectResponseSchema: expectResponseSchema,
//		APIKey:               config.openAiApiKey,
//	}
//	if err != nil {
//		log.Fatalf("Error reading input files: %v", err)
//	}
//
//	mainPrompt := job.CustomPrompt
//	if mainPrompt == "" {
//		mainPrompt, err = getDefaultPrompt(layout)
//		if err != nil {
//			log.Println("error from reading input prompt: ", err)
//			return
//		}
//	}
//
//	//todo: fix this calls arguments it should probably just be one struct.
//	err = tuneResumeContents(inputTemp, mainPrompt, baselineJSON, layout, style, job.Id.String(), acceptableRatio, maxAttempts, fs, config, job, updates)
//	if err != nil {
//		log.Fatalf("Error from resume tuning: %v", err)
//	}
//
//	// Simulate job processing
//	//time.Sleep(5 * time.Second)
//	//result := fmt.Sprintf("Processed job with field1: %s, field2: %s, field3: %s", job.Field1, job.Field2, job.Field3)
//
//	//jobResult := JobResult{ID: jobID, Status: "Completed", Result: result}
//
//	// Store job result in filesystem
//	//if err := fs.WriteFile(jobID, jobResult); err != nil {
//	//	log.Printf("Error writing job result: %v", err)
//	//}
//}

//
//// handleJobSubmission handles incoming job submissions via POST.
//func handleJobSubmission(w http.ResponseWriter, r *http.Request) {
//	log.Println("here in handleJobSubmission")
//	if r.Method != http.MethodPost {
//		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
//		return
//	}
//
//	var job Job
//	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
//		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
//		return
//	}
//	log.Printf("here in handleJobSubmission with job like: %#v", job)
//
//	//// Add job to queue
//	job.Id = uuid.New()
//	jobQueue <- job
//
//	// Respond with a job URL
//	jobURL := fmt.Sprintf("/checkjob/%d", 12345)
//	w.WriteHeader(http.StatusAccepted)
//	w.Write([]byte(fmt.Sprintf("Job submitted successfully. Poll job status at: %s", jobURL)))
//}
//
//// handleJobStatus handles checking job status via GET.
//func handleJobStatus(w http.ResponseWriter, r *http.Request) {
//	jobIDStr := r.URL.Path[len("/checkjob/"):]
//	jobID, err := strconv.Atoi(jobIDStr)
//	if err != nil {
//		http.Error(w, "Invalid job ID", http.StatusBadRequest)
//		return
//	}
//
//	_ = jobID
//	log.Printf("NYI: checking of job results via http")
//	jobResult := &JobResult{}
//	//jobResult, err := fs.ReadFile(jobID)
//	//if err != nil {
//	//	http.Error(w, "Job not found", http.StatusNotFound)
//	//	return
//	//}
//
//	w.Header().Set("Content-Type", "application/json")
//	json.NewEncoder(w).Encode(jobResult)
//}
//
//// handJobRun keep the connection going for the life of the job (try stuff in google cloud run)
//func handJobRun(config *serviceConfig) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		log.Println("here in handJobRun")
//		if r.Method != http.MethodPost {
//			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
//			return
//		}
//
//		var job Job
//		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
//			http.Error(w, "Invalid JSON input", http.StatusBadRequest)
//			return
//		}
//		log.Printf("here in handJobRun with job like: %#v", job)
//
//		//// Add job to queue
//		job.Id = uuid.New()
//		runJob(&job, config, nil)
//		w.WriteHeader(http.StatusOK)
//		w.Write([]byte(fmt.Sprintf("Job complete. Maybe I could give pdf data")))
//	}
//}
//
//func handleStreamJob(config *serviceConfig) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		log.Println("here in handleStreamJob")
//		if r.Method != http.MethodPost {
//			http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
//			return
//		}
//
//		var job Job
//		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
//			http.Error(w, "Invalid JSON input", http.StatusBadRequest)
//			return
//		}
//		log.Printf("here in handJobRun with job like: %#v", job)
//
//		// Set headers for streaming response
//		w.Header().Set("Content-Type", "application/json")
//		w.Header().Set("Transfer-Encoding", "chunked")
//		w.WriteHeader(http.StatusOK)
//
//		// Create a channel to communicate job status updates
//		statusChan := make(chan JobStatus)
//
//		//// Add job to queue
//		job.Id = uuid.New()
//		go runJob(&job, config, statusChan)
//		// Goroutine simulating a background job that sends status updates to the channel
//		//go func() {
//		//	for i := 0; i < 500; i++ {
//		//		message := fmt.Sprintf("Processing step %d...", i+1)
//		//		sendJobUpdate(statusChan, message)
//		//		time.Sleep(10 * time.Millisecond) // Simulate work being done
//		//		log.Printf("here in fakey %s", message)
//		//	}
//		//	close(statusChan)
//		//}()
//
//		// Stream status updates to the client
//		for status := range statusChan {
//			// Create a JobStatus struct with the status message
//
//			// Marshal the status update to JSON
//			data, err := json.Marshal(status)
//			if err != nil {
//				http.Error(w, "Error encoding status", http.StatusInternalServerError)
//				return
//			}
//
//			// Write the JSON status update to the response
//			_, err = fmt.Fprintf(w, "%s\n", data)
//			if err != nil {
//				log.Println("Client connection lost.")
//				return
//			}
//
//			// Flush the response writer to ensure the data is sent immediately
//			if f, ok := w.(http.Flusher); ok {
//				log.Println("here flusho1")
//				f.Flush()
//			}
//		}
//
//		// Final result after job completion
//		finalResult := Result{
//			Status:  "Completed",
//			Details: "The job was successfully completed.",
//		}
//
//		// Marshal the final result to JSON
//		finalData, err := json.Marshal(finalResult)
//		if err != nil {
//			http.Error(w, "Error encoding final result", http.StatusInternalServerError)
//			return
//		}
//
//		// Send the final JSON result to the client
//		fmt.Fprintf(w, "%s\n", finalData)
//
//		// Flush final response to the client
//		if f, ok := w.(http.Flusher); ok {
//			log.Println("here flusho2")
//			f.Flush()
//		}
//	}
//}
//
//func handleJobOutput(config *serviceConfig) http.HandlerFunc {
//	return func(w http.ResponseWriter, r *http.Request) {
//		log.Printf("here 1 ...")
//		// Extract the path and split it by '/'
//		pathParts := strings.Split(r.URL.Path, "/")
//		// should become stuff like []string{"","jobresult","somejobid","somepdf"}
//		if len(pathParts) < 4 {
//			http.Error(w, "Invalid path", http.StatusBadRequest)
//			return
//		}
//
//		// Rejoin everything after "/jobresult/"
//		// pathParts[2:] contains everything after "/jobresult/"
//		resultPath := strings.Join(pathParts[2:], "/")
//
//		// Use the rejoined path as needed
//		log.Printf("Result Path: %s\n", resultPath)
//
//		data, err := fs.ReadFile(resultPath)
//		if err != nil {
//			http.Error(w, "Could not read file from GCS", http.StatusBadRequest)
//			return
//		}
//		log.Printf("Read %d bytes of data from GCS", len(data))
//
//		// Infer the Content-Type based on the file extension
//		fileName := pathParts[len(pathParts)-1]
//		log.Printf("Send back with filename: %s\n", fileName)
//		ext := filepath.Ext(fileName)
//		mimeType := mime.TypeByExtension(ext)
//
//		// Fallback to application/octet-stream if we can't determine the MIME type
//		if mimeType == "" {
//			mimeType = "application/octet-stream"
//		}
//
//		// Set the headers
//		w.Header().Set("Content-Type", mimeType)
//		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
//		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
//
//		// Write the PDF content to the response
//		w.WriteHeader(http.StatusOK)
//		_, err = w.Write(data)
//		if err != nil {
//			http.Error(w, "Unable to write file to response", http.StatusInternalServerError)
//			return
//		}
//	}
//}
//
//// parseFlags parses the command line arguments.
//// func parseFlags() (string, string) {
//func parseFlags() string {
//	mode := flag.String("mode", "server", "Mode: either 'server' or 'cli'")
//	//fsType := flag.String("fs", "local", "Filesystem type: 'local' or 'gcs'")
//	//localPath := flag.String("local-path", "./jobs", "Local file path for job storage (only for 'local' filesystem)")
//	//gcsBucket := flag.String("gcs-bucket", "", "GCS bucket name for job storage (only for 'gcs' filesystem)")
//
//	flag.Parse()
//
//	// Validation
//	//if *fsType == "gcs" && *gcsBucket == "" {
//	//	log.Fatal("GCS bucket name must be specified for GCS filesystem")
//	//}
//	//
//	//if *fsType == "local" && *localPath == "" {
//	//	log.Fatal("Local path must be specified for local filesystem")
//	//}
//	//
//	//return *mode, *fsType
//	return *mode
//}
//
//func WriteAttemptResumedataJSON(content, layout, style, outputDir string, attemptNum int, fs filesystem.FileSystem, config *config.ServiceConfig) error {
//	// Step 5: Write the validated content to the filesystem in a way the resume projects json server can read it, plus locally for posterity.
//	// Assuming the file path is up and outside of the project directory
//	// Example: /home/user/output/validated_content.json
//	updatedContent, err := insertLayout(content, layout, style)
//	if err != nil {
//		log.Printf("Error inserting layout info: %v\n", err)
//		return err
//	}
//
//	//TODO !!!!!!!! dont do this!!!!!! not like this!!!!
//	if config.FsType == "local" {
//		// this is/was just a cheesy way to get the attempted resume updated json available to the react project via a local json server service.
//		outputFilePath := filepath.Join("../ResumeData/resumedata/", fmt.Sprintf("attempt%d.json", attemptNum))
//		err = tuner.WriteValidatedContent(updatedContent, outputFilePath)
//		if err != nil {
//			log.Printf("Error writing content to file: %v\n", err)
//			return err
//		}
//		log.Println("Content successfully written to:", outputFilePath)
//	} else if config.FsType == "gcs" {
//		outputFilePath := fmt.Sprintf("%s/attempt%d.json", outputDir, attemptNum)
//		log.Printf("writeAttemptResumedataJSON to GCS bucket, path: %s", outputFilePath)
//		err = fs.WriteFile(outputFilePath, []byte(content))
//		if err != nil {
//			log.Printf("Error writing content to file: %v\n", err)
//			return err
//		}
//		log.Printf("writeAttemptResumedataJSON thinks it got past that.")
//	}
//
//	// Example: /home/user/output/validated_content.json
//	localOutfilePath := filepath.Join(outputDir, fmt.Sprintf("attempt%d.json", attemptNum))
//	err = tuner.WriteValidatedContent(updatedContent, localOutfilePath)
//	if err != nil {
//		log.Printf("Error writing content to file: %v\n", err)
//		return err
//	}
//	log.Println("Content successfully written to:", localOutfilePath)
//	return nil
//}
//
//func insertLayout(content string, layout string, style string) (string, error) {
//	// Step 1: Deserialize the JSON content into a map
//	var data map[string]interface{}
//	err := json.Unmarshal([]byte(content), &data)
//	if err != nil {
//		return "", err
//	}
//
//	// Step 2: Insert the layout into the map
//	data["layout"] = layout
//
//	if style != "" {
//		data["style"] = style
//	}
//
//	// Step 3: Reserialize the map back into a JSON string
//	updatedContent, err := json.Marshal(data)
//	if err != nil {
//		return "", err
//	}
//
//	// Step 4: Return the updated JSON string
//	return string(updatedContent), nil
//}

//func getBaselineJSON(baseline string, config *serviceConfig) (string, error) {
//	// get JSON of the current complete resume including all the hidden stuff, this hits an express server that imports the reactresume resumedata.mjs and outputs it as json.
//	jsonRequestURL := fmt.Sprintf("%s?baseline=%s", config.jsonServerURL, baseline)
//	resp, err := http.Get(jsonRequestURL)
//	if err != nil {
//		log.Fatalf("Failed to make the HTTP request: %v", err)
//	}
//	defer resp.Body.Close()
//
//	body, err := io.ReadAll(resp.Body)
//	if err != nil {
//		log.Fatalf("Failed to read the response body: %v", err)
//	}
//	log.Printf("got %d bytes of json from the json-server via %s\n", len(body), jsonRequestURL)
//	return string(body), nil
//}

//func getLayoutFromBaselineJSON(baselineJSON string) (string, string, error) {
//	//if i want anything else beyond layout and style i should return a struct because this is ugly.
//
//	//log.Println("dbg baselinejson", baselineJSON)
//	var decoded map[string]interface{}
//	err := json.Unmarshal([]byte(baselineJSON), &decoded)
//	if err != nil {
//		return "", "", err
//	}
//
//	// Check if the "layout" key exists and is a string
//	layout, ok := decoded["layout"].(string)
//	if !ok {
//		return "", "", errors.New("layout is missing or not a string")
//	}
//	// Check if the "style" key exists and is a string (its ok if its not there but if it is we should keep it)
//	style, _ := decoded["style"].(string)
//
//	return layout, style, nil
//}
//
//type jdMeta struct {
//	CompanyName string   `json:"company_name" validate:"required"`
//	JobTitle    string   `json:"job_title" validate:"required"`
//	Keywords    []string `json:"keywords" validate:"required"`
//	Location    string   `json:"location" validate:"required"`
//	RemoteOK    *bool    `json:"remote_ok" validate:"required"`
//	SalaryInfo  *string  `json:"salary_info" validate:"required"`
//	Process     *string  `json:"process" validate:"required"`
//}
//
//func takeNotesOnJD(input *Input, outputDir string) (string, error) {
//	//JDResponseFormat, err := os.ReadFile(filepath.Join("response_templates", "jdinfo.json"))
//	jDResponseSchemaRaw, err := os.ReadFile(filepath.Join("response_templates", "jdinfo-schema.json"))
//	if err != nil {
//		log.Fatalf("failed to read expect_response.json: %v", err)
//	}
//	// Validate the JSON content
//	jDResponseSchema, err := decodeJSON(string(jDResponseSchemaRaw))
//	if err != nil {
//		log.Fatalf("failed to decode JSON: %v", err)
//	}
//	//if err := validateJSON(string(jDResponseSchemaRaw)); err != nil {
//	//	return err
//	//}
//	prompt := strings.Join([]string{
//		"Extract information from the following Job Description. Take note of the name of the company, the job title, and most importantly the list of key words that a candidate will have in their CV in order to get through initial screening. Additionally, extract any location, remote-ok status, salary info and hiring process notes which can be succinctly captured.",
//		"\n--- start job description ---\n",
//		input.JD,
//		"\n--- end job description ---\n",
//	}, "")
//
//	apirequest := map[string]interface{}{
//		"model": "gpt-4o-mini",
//		"messages": []map[string]interface{}{
//			{
//				"role":    "system",
//				"content": "You are a Job Description info extractor assistant.",
//			},
//			//{
//			//	"role":    "system",
//			//	"content": "You are a Job Description info extractor assistant. The response should include only the fields of the provided JSON example, in well-formed JSON, without any triple quoting, such that your responses can be ingested directly into an information system.",
//			//},
//			//{
//			//	"role":    "user",
//			//	"content": "Show me an example input for the Job Description information system to ingest",
//			//},
//			//{
//			//	"role":    "assistant",
//			//	"content": JDResponseFormat,
//			//},
//			{
//				"role":    "user",
//				"content": prompt,
//			},
//		},
//		"response_format": map[string]interface{}{
//			"type": "json_schema",
//			"json_schema": map[string]interface{}{
//				"name":   "job_description",
//				"strict": true,
//				"schema": jDResponseSchema,
//			},
//		},
//		//"max_tokens":  100,
//		"temperature": 0.7,
//	}
//	api_request_pretty, err := serializeToJSON(apirequest)
//	writeToFile(api_request_pretty, 0, "jd_info_request_pretty", outputDir)
//	if err != nil {
//		log.Fatalf("Failed to marshal final JSON: %v", err)
//	}
//
//	exists, output, err := checkForPreexistingAPIOutput(outputDir, "jd_info_response_raw", 0)
//	if err != nil {
//		log.Fatalf("Error checking for pre-existing API output: %v", err)
//	}
//	if !exists {
//		output, err = makeAPIRequest(apirequest, input.APIKey, 0, "jd_info_response_raw", outputDir)
//		if err != nil {
//			log.Fatalf("Error making API request: %v", err)
//		}
//	}
//
//	//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
//	var apiResponse APIResponse
//	err = json.Unmarshal([]byte(output), &apiResponse)
//	if err != nil {
//		log.Fatalf("Error deserializing API response: %v\n", err)
//	}
//
//	//Extract the message content
//	if len(apiResponse.Choices) == 0 {
//		log.Fatalf("No choices found in the API response")
//	}
//
//	content := apiResponse.Choices[0].Message.Content
//
//	err = validateJSON(content)
//	if err != nil {
//		log.Fatalf("Error validating JSON content: %v\n", err)
//	}
//	log.Printf("Got %d bytes of JSON content about the JD (at least well formed enough to be decodable) out of that last response\n", len(content))
//
//	outputFilePath := filepath.Join(outputDir, "jdinfo-out.json")
//	err = writeValidatedContent(content, outputFilePath)
//	if err != nil {
//		log.Fatalf("Error writing content to file: %v\n", err)
//	}
//	log.Println("JD Info Content successfully written to:", outputFilePath)
//	return content, nil
//}

//
//func getDefaultPrompt(layout string) (string, error) {
//	log.Printf("No (or empty) prompt from the file system so will use a default one.")
//
//	//prompt := "Provide feedback on the following JSON. Is it well formed? What do you think the purpose is? Tell me about things marked as hide and what that might mean. Finally, how long in terms of page count do you think the final document this feeds into is?\n\nJSON: "
//
//	//prompt_parts := []string{
//	//	"This guy needs a job ASAP. You need to make his resume look PERFECT for the job. Fake it until you make it right? Fix it up, do what it takes. Aim for 3-5 companies, each with 2-4 projects. Make them relate to the Job Description as best as possible, including possibly switching up industries and industry terms. ",
//	//	"Feel free to dig into those hidden companies and projects for inspiration, include whatever you think could be relevant. ",
//	//	"Do not claim to have worked at the target company from the Job Description unless the input resume data structure JSON from the candidate actually claims to have already worked there before. ",
//	//	"The target Job Description for which this candidate should appear to perfectly match is below. Pay special attention to any magic tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability (magic tokens should be placed somewhere that will get rendered such as a project description or a job title):\n",
//	//	"\n--- start job description ---\n",
//	//	input.JD,
//	//	"\n--- end job description ---\n",
//	//	"The following JSON represents the current generic 1 page resume for the candidate. Much of the information in the data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description above. ",
//	//	"The input resume data structure JSON is below:\n",
//	//	string(body),
//	//}
//
//	//prompt_parts := []string{
//	//	"The hiring industry is broken. We are not taking this seriously. Reformat the resume and fill it up with goofy nonsense that makes fun of recruiting and hiring in general as well as the whole hiring industry. Fill it with metrics about job losses, bad candidates, jobs that dont exist and wasted hours on take home assignments. ",
//	//	"Ostensibly, the job that's being targeted is described in the Job Description below, but the details aren't as important as the overall cynicism and irony:\n",
//	//	"\n--- start job description ---\n",
//	//	input.JD,
//	//	"\n--- end job description ---\n",
//	//	"The following JSON represents the current generic 1 page resume for the candidate. Much of the information in the data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more cynically suitable for the Job Description above. ",
//	//	"The input resume data structure JSON is below:\n",
//	//	string(body),
//	//}
//
//	//common := //todo if having to change things that are the same perhaps
//
//	//because the different layouts are going to be structured different i just want to refer to the sections by what they are, so slight nuance depending on the layout. annoying yes, and maybe not neccessary idk. perhaps a more unified way of talking about the work history could be done
//	m := map[string]string{
//		"chrono": strings.Join([]string{
//			"The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. ",
//			//"Your output JSON can simply omit anything which need not be seen in the rendered resume document (If all of the projects within a job are marked as hidden then the whole job will be hidden). ",
//			//"The work_history contains a list of companies and projects within those companies. ",
//			"Some of the information in the candidate resume JSON data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description below. ",
//			"Your output JSON can simply omit anything which need not be seen in the rendered resume document. ",
//			"Feel free to adjust any descriptive text fields at the company or project level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. ",
//			"Embellishment of anything remotely factual or possibly tangential is encouraged. ",
//			"Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to include it. ",
//			"The goal is to keep the length of the final render at one page, while showing the most relevant information to make the candidate appear a perfect fit for the target job. ",
//			"Be sure to include between 3 and 5 distinct company sections. Each company section can list separate projects within it, aim for 2-3 projects within each company. ",
//			"Make sure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed. ",
//			"The target Job Description for which this candidate should appear to perfectly match is below. ",
//			"Pay special attention to any special tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability:",
//		}, ""),
//		"functional": strings.Join([]string{
//			//"This guy needs a job ASAP. You need to make his resume look PERFECT for the job. Fake it until you make it right? Fix it up, do what it takes. Aim for 3-5 Functional Areas, each with 2-4 examples of key contributions. Make them relate to the Job Description as best as possible, including possibly switching up industries and industry terms. ",
//			//"Feel free to dig into those hidden companies and projects for inspiration, include whatever you think could be relevant. ",
//			//"The target Job Description for which this candidate should appear to perfectly match is below. Pay special attention to any magic tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability (magic tokens should be placed somewhere that will get rendered such as a project description or a job title):\n",
//
//			"The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. ",
//			"Some of the information in the candidate resume JSON data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description below. ",
//			"Your output JSON can simply omit anything which need not be seen in the rendered resume document. ",
//			"Feel free to adjust any descriptive text fields at the functional area or key contribution level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. ",
//			"Embellishment of anything remotely factual or possibly tangential is encouraged. ",
//			"Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to include it. ",
//			"The goal is to keep the length of the final render at one page, while showing the most relevant information to make the candidate appear a perfect fit for the target job. ",
//			"Be sure to include between 3 and 5 distinct functional areas. Each functional area can list separate key contributions within it, aim for 2-3 examples within each. ",
//			"Ensure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed, ",
//			"The target Job Description for which this candidate should appear to perfectly match is below. ",
//			"Pay special attention to any special tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability:",
//		}, ""),
//	}
//	value, ok := m[layout]
//	if !ok {
//		return "", errors.New("layout prompt not found: " + layout)
//	}
//	return value, nil
//}
//
//func makeAPIRequest(apiBody interface{}, apiKey string, counter int, name, outputDir string) (string, error) {
//	//panic("slow down there son, you really want to hit the paid api at this time?")
//	log.Printf("Make request to OpenAI ...")
//	// Serialize the interface to pretty-printed JSON
//	jsonData, err := json.Marshal(apiBody)
//	if err != nil {
//		return "", fmt.Errorf("failed to serialize API request body to JSON: %v", err)
//	}
//
//	// Create a new HTTP POST request
//	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer([]byte(jsonData)))
//	if err != nil {
//		return "", fmt.Errorf("failed to create HTTP request: %v", err)
//	}
//
//	// Set the Content-Type and Authorization headers
//	req.Header.Set("Content-Type", "application/json")
//	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
//
//	// Send the request using the default HTTP client
//	client := &http.Client{}
//	resp, err := client.Do(req)
//	if err != nil {
//		return "", fmt.Errorf("failed to send HTTP request: %v", err)
//	}
//	defer resp.Body.Close()
//
//	// Read the response body
//	respBody, err := io.ReadAll(resp.Body)
//	if err != nil {
//		return "", fmt.Errorf("failed to read response body: %v", err)
//	}
//
//	// Convert the response body to a string
//	responseString := string(respBody)
//
//	// Write the response to the filesystem
//	err = writeToFile(responseString, counter, name, outputDir)
//	if err != nil {
//		return "", fmt.Errorf("failed to write response to file: %v", err)
//	}
//	log.Printf("Got response from OpenAI API ... (and should have wrote it to the file system)")
//
//	// Return the response string
//	return responseString, nil
//}
//
//// APIResponse Struct to represent (parts of) the API response (that we care about r/n)
//type APIResponse struct {
//	Choices []struct {
//		Message struct {
//			Content string `json:"content"`
//		} `json:"message"`
//	} `json:"choices"`
//}

//
//func getExpectedResponseJsonSchema(layout string) (interface{}, error) {
//	expectResponseFilePath := filepath.Join("response_templates", fmt.Sprintf("%s-schema.json", layout))
//	// Read expect_response.json
//	expectResponseContent, err := os.ReadFile(expectResponseFilePath)
//	if err != nil {
//		return nil, fmt.Errorf("failed to read expect_response.json: %v", err)
//	}
//	// Validate the JSON content
//	expectResponseSchema, err := decodeJSON(string(expectResponseContent))
//	if err != nil {
//		return nil, err
//	}
//	return expectResponseSchema, nil
//}

//
//// cleanAndValidateJSON takes MJS file contents as a string, strips non-JSON content
//// on the first line, removes double-slash comment lines, and validates the resulting JSON.
//func cleanAndValidateJSON(mjsContent string) (string, error) {
//	lines := strings.Split(mjsContent, "\n")
//
//	// Step 1: Remove lines that contain double-slash comments
//	var cleanedLines []string
//	for _, line := range lines {
//		trimmedLine := strings.TrimSpace(line)
//		if len(trimmedLine) == 0 {
//			continue
//		}
//		if strings.HasPrefix(trimmedLine, "//") {
//			continue
//		} else if commentIndex := findCommentIndex(line); commentIndex != -1 {
//			// Take the part of the line before the comment and add it to cleanedLines
//			cleanedLines = append(cleanedLines, line[:commentIndex])
//		} else {
//			cleanedLines = append(cleanedLines, line)
//		}
//	}
//
//	// Step 2: Process the first line to strip non-JSON content
//	if len(lines) > 0 {
//		firstLine := cleanedLines[0]
//		// Use strings.Index to find the first occurrence of '{' and remove everything before it
//		if index := strings.Index(firstLine, "{"); index != -1 {
//			cleanedLines[0] = firstLine[index:]
//		} else {
//			return "", fmt.Errorf("no JSON object found on the first line (line looks like: '%s')", firstLine)
//		}
//	}
//
//	// Step 3: Join the cleaned lines back into a single string
//	cleanedJSON := strings.Join(cleanedLines, "\n")
//	fmt.Printf("working with :\n\n'%s'", cleanedJSON)
//	//panic("stop wut")
//
//	// Step 4: Validate the resulting string as JSON
//	var js map[string]interface{}
//	if err := json.Unmarshal([]byte(cleanedJSON), &js); err != nil {
//		return "", fmt.Errorf("invalid JSON: %v", err)
//	}
//
//	// Step 5: Return the cleaned and validated JSON string
//	return cleanedJSON, nil
//}
//
//// findCommentIndex finds the index of "//" that is not within a string
//func findCommentIndex(line string) int {
//	inString := false
//	for i := 0; i < len(line)-1; i++ {
//		if line[i] == '"' {
//			inString = !inString
//		}
//		if !inString && line[i] == '/' && line[i+1] == '/' {
//			return i
//		}
//	}
//	return -1
//}

//
//func whitePct(img image.Image) float64 {
//	// Get image dimensions
//	bounds := img.Bounds()
//	totalPixels := bounds.Dx() * bounds.Dy()
//
//	// Count white pixels
//	whitePixels := 0
//	pixels := 0
//	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
//		for x := bounds.Min.X; x < bounds.Max.X; x++ {
//			r, g, b, a := img.At(x, y).RGBA()
//			pixels++
//			if isWhite(r, g, b, a) {
//				whitePixels++
//			}
//		}
//	}
//
//	// Calculate percentage of white space
//	whiteSpacePercentage := (float64(whitePixels) / float64(totalPixels)) * 100
//
//	fmt.Printf("White space percentage: %.2f%%\n", whiteSpacePercentage)
//	fmt.Printf("Checked pixels: : %d\n", pixels)
//
//	return whiteSpacePercentage
//}

//func isWhite(r, g, b, a uint32) bool {
//	const threshold = 0.9 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
//	//also .... transparent with no color == white when on screen as a pdf so ....
//	//return a > 0 && float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
//	return a == 0 || float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
//}
