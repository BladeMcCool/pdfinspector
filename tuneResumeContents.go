package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"pdfinspector/filesystem"
	"strings"
)

func tuneResumeContents(input *Input, mainPrompt, baselineJSON, layout, style, outputDir string, acceptableRatio float64, maxAttempts int, fs filesystem.FileSystem, config *serviceConfig, job *Job, updates chan JobStatus) error {
	sendJobUpdate(updates, "getting any JD meta")
	jDmetaRawJSON, err := takeNotesOnJD(input, outputDir)
	if err != nil {
		log.Println("error taking notes on JD: ", err)
		return err
	}
	jDMetaDecoded := &jdMeta{}
	err = json.Unmarshal([]byte(jDmetaRawJSON), jDMetaDecoded)
	if err != nil {
		log.Println("error extracting notes on JD: ", err)
		return err
	}
	sendJobUpdate(updates, "got any JD meta")

	//panic("does it look right - before proceeding")
	kwPrompt := ""
	if len(jDMetaDecoded.Keywords) > 0 {
		kwPrompt = "The adjusted resume data should contain as many of the following keywords as is reasonable/possible: " + strings.Join(jDMetaDecoded.Keywords, ", ") + "\n"
	}
	prompt_parts := []string{
		mainPrompt,
		"\n--- start job description ---\n",
		input.JD,
		"\n--- end job description ---\n",
		kwPrompt,
		"The following JSON resume data represents the work history, skills, competencies and education for the candidate:\n",
		baselineJSON,
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

	prompt := strings.Join(prompt_parts, "")

	// Create a map to represent the API request structure
	data := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{
			{
				"role": "system",
				//"content": fmt.Sprintf("You are a helpful resume tuning person (not a bot or an AI). The response should include only the fields expected to be rendered by the application, in well-formed JSON, without any triple quoting, such that the final resume fills one page to between %d%% and 95%%, leaving only a small margin at the bottom.", int(acceptable_ratio*100)),
				//"content": fmt.Sprintf("You are a helpful resume tuning assistant. The response should include resume content such that the final resume fills one page to between %d%% and 95%%, leaving only a small margin at the bottom. The output must respect the supplied JSON schema including having some value for fields identified as required in the schema", int(acceptable_ratio*100)),
				"content": fmt.Sprintf("You are a helpful resume tuning assistant. The response should include resume content such that the final resume fills one page to between %d%% and 95%%, leaving only a small margin at the bottom.", int(acceptableRatio*100)),
			},
			// this was the way to do it without using the structured output facilities. tbh i'm still not sure what was producing better results but continuing on with the "right" way (structured output) at present.
			//{
			//	"role":    "user",
			//	"content": "Show me an example input for the resume system to ingest",
			//},
			//{
			//	"role":    "assistant",
			//	"content": input.ExpectResponse,
			//},
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
				"schema": input.ExpectResponseSchema,
			},
		},
		//"max_tokens":  2000, //idk i had legit response go over 2000 because it was wordy. not sure that bug where it generated full stream of garbage happened again after putting on 'strict' tho. keep an eye on things.
		"temperature": 0.7,
	}
	messages := data["messages"].([]map[string]interface{}) //preserve orig

	for i := 0; i < maxAttempts; i++ {
		api_request_pretty, err := serializeToJSON(data)
		writeToFile(api_request_pretty, i, "api_request_pretty", outputDir)
		if err != nil {
			log.Fatalf("Failed to marshal final JSON: %v", err)
		}
		exists, output, err := checkForPreexistingAPIOutput(outputDir, "api_response_raw", i)
		if err != nil {
			log.Fatalf("Error checking for pre-existing API output: %v", err)
		}
		if !exists {
			sendJobUpdate(updates, fmt.Sprintf("asking for an attempt %d", i))
			output, err = makeAPIRequest(data, input.APIKey, i, "api_response_raw", outputDir)
		}

		//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
		var apiResponse APIResponse
		err = json.Unmarshal([]byte(output), &apiResponse)
		if err != nil {
			fmt.Printf("Error deserializing API response: %v\n", err)
			return err
		}

		//Extract the message content
		if len(apiResponse.Choices) == 0 {
			return errors.New("no choices found in the API response")
		}

		content := apiResponse.Choices[0].Message.Content

		err = validateJSON(content)
		if err != nil {
			fmt.Printf("Error validating JSON content: %v\n", err)
			return err
		}
		log.Printf("Got %d bytes of JSON content (at least well formed enough to be decodable) out of that last response\n", len(content))
		sendJobUpdate(updates, fmt.Sprintf("got JSON for attempt %d, will request PDF", i))

		err = writeAttemptResumedataJSON(content, layout, style, outputDir, i, fs, config)

		//we should be able to render that updated content proposal now via gotenberg + ghostscript
		err = makePDFRequestAndSave(i, layout, outputDir, config, job)
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
		sendJobUpdate(updates, fmt.Sprintf("got PDF for attempt %d, will dump to PNG", i))

		//and the ghostscript dump to pngs ...
		err = dumpPDFToPNG(i, outputDir, config)
		if err != nil {
			log.Printf("Error during pdf to image dump: %v\n", err)
			break
		}
		sendJobUpdate(updates, fmt.Sprintf("got PNGs for attempt %d, will check it", i))

		result, err := inspectPNGFiles(outputDir, i)
		if err != nil {
			log.Printf("Error inspecting png files: %v\n", err)
			break
		}

		log.Printf("inspect result: %#v", result)
		if result.NumberOfPages == 0 {
			log.Printf("no pages, idk just stop")
			break
		}
		sendJobUpdate(updates, fmt.Sprintf("png inspection for attempt %d: %#v", i, result))

		tryNewPrompt := false
		var tryPrompt string
		if result.NumberOfPages > 1 {
			if result.NumberOfPages > 2 {
				log.Println("too many pages , this could be interesting but stop for now")
				tryNewPrompt = true
				tryPrompt = fmt.Sprintf("That was way too long, reduce the amount of content to try to get it down to one full page by summarizing or removing some existing project descriptions, removing projects within companies or by shortening up the skills list. Remember to make the candidate still look great in relation to the Job Description supplied earlier!")
			} else {
				reduceByPct := int(((result.LastPageContentRatio / (1 + result.LastPageContentRatio)) * 100) / 2)
				log.Printf("only one extra page .... reduce by %d%%", reduceByPct)
				tryNewPrompt = true
				//tryPrompt = fmt.Sprintf("Too long, reduce by %d%%, by making minimal edits to the prior output as possible. Sometimes going overboard on skills makes it too long. Remember to make the candidate still look great in relation to the Job Description supplied earlier!", reduceByPct)
				tryPrompt = fmt.Sprintf("Too long, reduce the total content length by %d%%, while still keeping the information highly relevant to the Job Description.", reduceByPct)
			}
		} else if result.NumberOfPages == 1 && result.LastPageContentRatio < acceptableRatio {
			log.Println("make it longer ...")
			tryNewPrompt = true
			//tryPrompt = fmt.Sprintf("Not long enough when rendered, was only %d%% of the page. Fill it up to between %d%% and 95%%. You can bulk up the content of existing project descriptions, add new projects within companies or by beefing up the skills list. Remember to make the candidate look even greater in relation to the Job Description supplied earlier!", int(result.LastPageContentRatio*100), int(acceptable_ratio*100))
			increaseByPct := int((95.0 - result.LastPageContentRatio*100) / 2) //wat? idk smthin like this anyway.
			//tryPrompt = fmt.Sprintf("Not long enough, increase by %d%%, you can bulk up the content of existing project descriptions, add new projects within companies or by beefing up the skills list. Remember to make the candidate look even greater in relation to the Job Description supplied earlier!", increaseByPct)
			tryPrompt = fmt.Sprintf("Not long enough, increase the total content length by %d%%, while still keeping the information highly relevant to the Job Description.", increaseByPct)

			//try to make it longer!!! - include the assistants last message in the new prompt so it can see what it did
		} else if result.NumberOfPages == 1 && result.LastPageContentRatio >= acceptableRatio {
			log.Printf("over %d%% and still on one page? nice. we should stop (determined complete after attempt index %d).\n", int(acceptableRatio*100), i)
			break
		}
		log.Printf("will try new prompt: %s", tryPrompt)
		if tryNewPrompt {
			//not sure what the best approach is, to only send the assistants last response and the new prompt,
			data["messages"] = append(messages, []map[string]interface{}{
				{
					"role":    "assistant",
					"content": content,
				}, {
					"role":    "user",
					"content": tryPrompt,
				},
			}...)

			//or to keep stacking them...
			//messages = append(messages, []map[string]interface{}{
			//	{
			//		"role":    "assistant",
			//		"content": content,
			//	}, {
			//		"role":    "user",
			//		"content": tryPrompt,
			//	},
			//}...)
			//data["messages"] = messages
		}
	}

	// Step 6: Marshal the request body into JSON
	finalJSON, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Failed to marshal final JSON: %v", err)
		return nil
	}

	// Print the final JSON request body
	fmt.Println(string(finalJSON))
	return nil
}
