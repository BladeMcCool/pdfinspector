package tuner

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/filesystem"
	"pdfinspector/pkg/job"
	"regexp"
	"strings"
)

var spaceStripRe = regexp.MustCompile(`\s+`)

// validateJSON checks if a string contains valid JSON
func validateJSON(data string) error {
	var js json.RawMessage //voodoo -- apparently even though its []byte .... thats ok? we can even re-unmarshal it to an actual type later? this was suggested to me for simple json decode verification, and it works. so *shrugs*
	if err := json.Unmarshal([]byte(data), &js); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}
	return nil
}

// DecodeJSON takes a JSON string and returns a deserialized object as an interface{}.
func DecodeJSON(data string) (interface{}, error) {
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

// writeToFile writes data to a file in the output directory with a filename based on the counter and fragment
func writeToFile(data string, counter int, filenameFragment, outputDir string) error {
	// Create the output directory if it doesn't exist
	log.Trace().Msgf("try to mkdirall for: %s", outputDir)
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

// WriteValidatedContent writes the validated content to a specific file path
func WriteValidatedContent(content, filePath string) error {
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
		log.Info().Msgf("Read prior response for api request attempt number %d from file system.", counter)
		return true, string(data), nil
	} else if os.IsNotExist(err) {
		// File does not exist
		log.Info().Msgf("No prior response found for api request attempt number %d in file system.", counter)
		return false, "", nil
	} else {
		// Some other error occurred
		log.Error().Msg("Error while checking file system for prior api response info.")
		return false, "", fmt.Errorf("error checking file existence: %v", err)
	}
}

func WriteAttemptResumedataJSON(content string, job *job.Job, attemptNum int, fs filesystem.FileSystem, config *config.ServiceConfig) error {
	// Step 5: Write the validated content to the filesystem in a way the resume projects json server can read it, plus locally for posterity.
	// Assuming the file path is up and outside of the project directory
	// Example: /home/user/output/validated_content.json
	updatedContent, err := insertLayout(content, job.Layout, job.StyleOverride)
	if err != nil {
		job.Log().Error().Msgf("Error inserting layout info: %v", err)
		return err
	}

	//TODO !!!!!!!! dont do this!!!!!! not like this!!!!
	if config.FsType == "local" {
		// this is/was just a cheesy way to get the attempted resume updated json available to the react project via a local json server service.
		outputFilePath := filepath.Join("../ResumeData/resumedata/", fmt.Sprintf("attempt%d.json", attemptNum))
		err = WriteValidatedContent(updatedContent, outputFilePath)
		if err != nil {
			job.Log().Error().Msgf("Error writing content to file: %v", err)
			return err
		}
		job.Log().Info().Msgf("Content successfully written to: %s", outputFilePath)
	} else if config.FsType == "gcs" {
		outputFilePath := fmt.Sprintf("%s/attempt%d.json", job.OutputDir, attemptNum)
		job.Log().Info().Msgf("writeAttemptResumedataJSON to GCS bucket, path: %s", outputFilePath)
		err = fs.WriteFile(outputFilePath, []byte(updatedContent))
		if err != nil {
			job.Log().Error().Msgf("Error writing content to file: %v", err)
			return err
		}
		job.Log().Trace().Msg("writeAttemptResumedataJSON thinks it got past that.")
	}

	// Example: /home/user/output/validated_content.json
	localOutfilePath := filepath.Join(job.OutputDir, fmt.Sprintf("attempt%d.json", attemptNum))
	err = WriteValidatedContent(updatedContent, localOutfilePath)
	if err != nil {
		job.Log().Error().Msgf("Error writing content to file: %v", err)
		return err
	}
	job.Log().Info().Msgf("Content successfully written to: %s", localOutfilePath)
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

func ExtractText(data interface{}) string {
	var texts []string
	var extract func(interface{})
	extract = func(d interface{}) {
		switch v := d.(type) {
		case string:
			// Append the string value to the texts slice
			texts = append(texts, v)
		case []interface{}:
			// Recursively process each item in the array
			for _, item := range v {
				extract(item)
			}
		case map[string]interface{}:
			// Recursively process each value in the object
			for _, value := range v {
				extract(value)
			}
			// Optional: Handle numbers, booleans, and nulls if needed
		}
	}
	extract(data)
	// Join all collected strings with a space separator
	return strings.Join(texts, " ")
}

func stripStringOfWhiteSpace(in string) string {
	//all instances of whitespace (spaces, tabs etc, should we use a strings.Replace or a regexp?
	return spaceStripRe.ReplaceAllString(in, "")
}

func resumeComposePrompt(job *job.Job, keywords []string) string {
	kwPrompt := ""
	if len(keywords) > 0 {
		kwPrompt = "The adjusted resume data should contain as many of the following keywords as is reasonable/possible: " + strings.Join(keywords, ", ") + "\n"
	}
	prompt_parts := []string{
		job.MainPrompt,
		"\n--- start job description ---\n",
		job.JobDescription,
		"\n--- end job description ---\n",
		kwPrompt,
		"The following JSON resume data represents the work history, skills, competencies and education for the candidate:\n",
		job.BaselineJSON,
	}

	//perhaps the resumedata should be at the start and the instructions of what to do with it should come after? need to a/b test this stuff somehow.
	//prompt_parts := []string{
	//	"The following JSON resume data represents the work history, skills, competencies and education for the candidate:\n",
	//	baselineJSON,
	//	"\n--- start job description ---\n",
	//	input.JD,
	//	"\n--- end job description ---\n",
	//	kwPrompt,
	//	mainPrompt,
	//}

	return strings.Join(prompt_parts, "")
}

func coverletterComposePrompt(job *job.Job, keywords []string) string {

	// cover letter prompt is a little different and can also refer to resumedata supplement.

	log.Trace().Msgf("coverletterComposePrompt: job %s", spew.Sprint(job))

	var supplement string
	if job.SupplementData != nil {
		//ok so this will be the raw json from a template. not sure i want to just jam that right into a prompt lol! ... so for now, i will extract the 'resumedata'
		var decoded interface{}
		err := json.Unmarshal(job.SupplementData, &decoded)
		if err == nil {
			if resumedata, ok := decoded.(map[string]interface{})["resumedata"]; ok {
				reEncoded, err := json.Marshal(resumedata)
				if err == nil {
					supplement = string(reEncoded)
					log.Info().Msgf("reencoded just the resumedata to: %s", supplement)
				}
			}
		}
	}

	prompt_parts := []string{}
	if supplement != "" {
		prompt_parts = append(prompt_parts, []string{
			"Candidate info to inform and supplement the contents of the cover letter:\n",
			"\n--- start candidate info ---\n",
			supplement,
			"\n--- end candidate info ---\n",
		}...)
	}

	prompt_parts = append(prompt_parts, []string{
		job.MainPrompt,
		"\n--- start job description ---\n",
		job.JobDescription,
		"\n--- end job description ---\n",
	}...)

	if len(keywords) > 0 {
		prompt_parts = append(prompt_parts, "The adjusted cover letter data should contain as many of the following keywords as is reasonable/possible: "+strings.Join(keywords, ", ")+"\n")
	}

	//for now lets include a baseline cover letter but i'd like to be able to control whether we do this or not somehow, because if a supplementary info is included then i'm not sure some predefined cover letter adds any value or might just confuse the gpt context
	prompt_parts = append(prompt_parts, []string{
		"The following JSON cover letter data represents a draft version supplied by the candidate. We should improve upon it or completely rewrite it if needed.\n",
		job.BaselineJSON,
	}...)

	return strings.Join(prompt_parts, "")
}
