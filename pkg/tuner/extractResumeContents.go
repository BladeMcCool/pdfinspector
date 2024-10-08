package tuner

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"path/filepath"
	"pdfinspector/pkg/job"
	"strings"
)

type ResumeExtractResult struct {
	ResumeJSONRaw  string
	ExpectedSchema interface{}
}

// func (t *Tuner) TuneResumeContents(input *job.Input, mainPrompt, baselineJSON, layout, style, outputDir string, fs filesystem.FileSystem, config *config.ServiceConfig, job *job.Job, updates chan job.JobStatus) error {
func (t *Tuner) ExtractResumeContents(fileContent []byte, layout string, UseSystemGs bool, updates chan job.JobStatus) (*ResumeExtractResult, error) {
	//job.Log().Info().Str("user_key", job.UserKey).Msgf("starting TuneResumeContents")
	SendJobUpdate(updates, "getting idk")
	expectResponseSchema, err := t.GetExpectedResponseJsonSchema(layout)
	if err != nil {
		return nil, fmt.Errorf("Error getting schema: %v\n", err)
	}
	_ = expectResponseSchema

	//////

	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Error getting current directory: %v\n", err)
	}

	// Construct the output directory path
	extractionId := uuid.New().String()
	outputDirFullpath := filepath.Join(currentDir, "extraction", extractionId)
	err = os.MkdirAll(outputDirFullpath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	//write the file data to a temp file so we can use gs to extract it.
	// to a file called input.pdf in the directory of outputDirFullpath
	inputFilePath := filepath.Join(outputDirFullpath, "input.pdf")

	// Write the file data to the new file (input.pdf)
	err = os.WriteFile(inputFilePath, fileContent, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("couldnt write pdf to filesystem")
	}

	//could maybe check the pdf for not containing error stuff like "Uncaught runtime errors" before proceeding.
	// MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd)/output:/workspace minidocks/ghostscript:latest gs -sDEVICE=pngalpha -o /workspace/out-%03d.png -r144 /workspace/attempt.pdf
	var cmd *exec.Cmd
	if UseSystemGs {
		cmd = exec.Command(
			"gs",
			"-sDEVICE=txtwrite",
			"-o", filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"),
			filepath.Join(outputDirFullpath, "input.pdf"),
		)
	} else {
		cmd = exec.Command("docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/workspace", outputDirFullpath),
			"minidocks/ghostscript:latest",
			"gs",
			"-sDEVICE=txtwrite",
			"-o", "/workspace/pdf-txtwrite.txt",
			"/workspace/input.pdf",
		)
	}
	log.Debug().Msgf("dump pdf to txt with gs command: %s", strings.Join(cmd.Args, " "))
	log.Info().Msg("About to check the pdf text to confirm no errors")
	// Run the command
	err = cmd.Run()
	log.Trace().Msg("Here just after run")
	if err != nil {
		return nil, fmt.Errorf("Error running docker command: %v\n", err)
	}
	log.Trace().Msg("Here before readfile")
	data, err := os.ReadFile(filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"))
	if err != nil {
		return nil, fmt.Errorf("error reading pdf txt output %v", err)
	}
	log.Trace().Msg("Here before checking for strings")
	if strings.Contains(string(data), "Uncaught runtime errors") {
		return nil, fmt.Errorf("'Uncaught runtime errors' string detected in PDF contents.")
	}
	if strings.Contains(string(data), "Error loading data: Failed to fetch") {
		return nil, fmt.Errorf("'Error loading data: Failed to fetch' string detected in PDF contents.")
	}
	log.Trace().Msg("read in the text")

	outputFilePath := filepath.Join(outputDirFullpath, "pdf-txtwrite.txt")

	// Read the contents of the file into a []byte
	fileBytes, err := os.ReadFile(outputFilePath)
	if err != nil {
		return nil, fmt.Errorf("couldnt read the output file from the pdf text extract")
	}
	log.Trace().Msgf("read this from text file: %s", string(fileBytes))

	resumeExtractionToLayoutRawJSONText, err := t.openAIResumeExtraction(fileBytes, layout, outputDirFullpath)
	_ = resumeExtractionToLayoutRawJSONText

	return &ResumeExtractResult{
		ResumeJSONRaw:  resumeExtractionToLayoutRawJSONText,
		ExpectedSchema: expectResponseSchema,
	}, nil
}
func (t *Tuner) openAIResumeExtraction(fileContent []byte, layout string, outputDir string) (string, error) {
	expectResponseSchema, err := t.GetExpectedResponseJsonSchema(layout)
	if err != nil {
		return "", err
	}

	prompt_parts := []string{
		"Inspect the following resume text and extract all relevant details pertaining to all fields of the included json schema. Format your response to include as much of the input resume text as possible, under appropriate output fields.",
		"\n--- start resume text data ---\n",
		string(fileContent),
		"\n--- end resume text data ---\n",
	}
	prompt := strings.Join(prompt_parts, "")

	data := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": fmt.Sprintf("You are a helpful resume data extraction assistant. The response should pay careful attention to mapping input data to sensible output fields and formats"),
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "candidate_resume",
				"strict": true,
				"schema": expectResponseSchema,
			},
		},
		//"max_tokens":  2000, //idk i had legit response go over 2000 because it was wordy. not sure that bug where it generated full stream of garbage happened again after putting on 'strict' tho. keep an eye on things.
		"temperature": 0.7,
	}
	//messages := data["messages"].([]map[string]interface{}) //preserve orig

	api_request_pretty, err := serializeToJSON(data)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal final JSON: %v", err)
	}
	err = writeToFile(api_request_pretty, 0, "api_request_pretty", outputDir)
	if err != nil {
		return "", fmt.Errorf("Failed to log api request locally: %v", err)
	}

	exists, output, err := checkForPreexistingAPIOutput(outputDir, "api_response_raw", 0)
	if err != nil {
		return "", fmt.Errorf("Error checking for pre-existing API output: %v", err)
	}
	if !exists {
		//SendJobUpdate(updates, fmt.Sprintf("asking for an attempt %d", i))
		//log.Info().Msg("making api request to openai ...")
		output, err = t.makeAPIRequest(data, 0, "api_response_raw", outputDir)
		if err != nil {
			log.Error().Msgf("openai request had error: %s", err.Error())
			return "", err
		}
	}

	//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
	var apiResponse APIResponse
	err = json.Unmarshal([]byte(output), &apiResponse)
	if err != nil {
		log.Error().Msgf("Error deserializing API response: %v", err)
		return "", err
	}

	//Extract the message content
	if len(apiResponse.Choices) == 0 {
		return "", errors.New("no choices found in the API response")
	}

	content := apiResponse.Choices[0].Message.Content
	//write the response here so that if there is an error with it we can see what it was ??

	err = validateJSON(content)
	if err != nil {
		log.Error().Msgf("Error validating JSON content: %v", err)
		return "", err
	}
	log.Info().Msgf("Got %d bytes of JSON content (at least well formed enough to be decodable) out of that last response", len(content))
	//SendJobUpdate(updates, fmt.Sprintf("got JSON for attempt %d, will request PDF", i))

	//this is just a string atm dunno if thats good enough lol
	return content, nil
}

// GuessCandidateName extracts the candidate's full name from the resumeData interface
func (t *Tuner) GuessCandidateName(resumeData interface{}) (string, error) {
	// Type assert resumeData as a map
	dataMap, ok := resumeData.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("resumeData is not a map")
	}

	// Extract "personal_info" field and ensure it's a map
	personalInfo, ok := dataMap["personal_info"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("personal_info not found or is not a map")
	}

	// Extract the "name" field from personal_info
	name, ok := personalInfo["name"].(string)
	if !ok {
		return "", fmt.Errorf("name not found or is not a string")
	}

	// Return the extracted name
	return name, nil
}
