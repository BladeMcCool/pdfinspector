package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	_ "image/png" // Importing PNG support
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

//func main() {
//	//
//	// Open the PNG file
//	img, err := imaging.Open("output/out-001.png")
//	if err != nil {
//		log.Fatalf("Failed to open image: %v", err)
//	}
//
//	//_ = whitePct(img)
//	_ = contentEnds(img)
//}

func main() {

	// Example call to ReadInput
	inputDir := "input" // The directory where jd.txt and expect_response.json are located

	//this stuff that should probably be cli overridable at least, todo.
	acceptable_ratio := 0.88
	max_attempts := 7
	layout := "functional"
	//layout := "default"
	outputDir := "output"

	input, err := ReadInput(inputDir, layout)
	if err != nil {
		log.Fatalf("Error reading input files: %v", err)
	}
	_ = input

	err = takeNotesOnJD(input, outputDir)
	if err != nil {
		log.Println("error taking notes on JD: ", err)
		return
	}
	//panic("does it look right - before proceeding")

	// get JSON of the current complete resume including all the hidden stuff, this hits an express server that imports the reactresume resumedata.mjs and outputs it as json.
	resp, err := http.Get(fmt.Sprintf("http://localhost:3002?layout=%s", layout))
	if err != nil {
		log.Fatalf("Failed to make the HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the response body: %v", err)
	}
	log.Printf("got %d bytes of json from the json-server\n", len(body))

	prompt_parts := []string{
		"\n--- start job description ---\n",
		input.JD,
		"\n--- end job description ---\n",
		"The following JSON represents the current generic 1 page resume for the candidate. Much of the information in the data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description above. ",
		"The input resume data structure JSON is below:\n",
		string(body),
	}
	inputPrompt, err := getInputPrompt(inputDir)
	if err != nil {
		log.Println("error from reading input prompt: ", err)
		return
	}
	prompt_parts = append([]string{inputPrompt}, prompt_parts...)

	prompt := strings.Join(prompt_parts, "")
	//log.Printf("prompto:\n\n%s", prompt)

	//// Serialize the map to JSON
	//jsonData, err := json.MarshalIndent(data, "", "  ")
	//if err != nil {
	//	log.Fatalf("Failed to marshal JSON: %v", err)
	//}

	// Create a map to represent the API request structure
	data := map[string]interface{}{
		"model": "gpt-4o-mini",
		"messages": []map[string]interface{}{
			{
				"role": "system",
				//"content": fmt.Sprintf("You are a helpful resume tuning person (not a bot or an AI). The response should include only the fields expected to be rendered by the application, in well-formed JSON, without any triple quoting, such that the final resume fills one page to between %d%% and 95%%, leaving only a small margin at the bottom.", int(acceptable_ratio*100)),
				//"content": fmt.Sprintf("You are a helpful resume tuning assistant. The response should include resume content such that the final resume fills one page to between %d%% and 95%%, leaving only a small margin at the bottom. The output must respect the supplied JSON schema including having some value for fields identified as required in the schema", int(acceptable_ratio*100)),
				"content": fmt.Sprintf("You are a helpful resume tuning assistant. The response should include resume content such that the final resume fills one page to between %d%% and 95%%, leaving only a small margin at the bottom.", int(acceptable_ratio*100)),
			},
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

	for i := range max_attempts {
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
			output, err = makeAPIRequest(data, input.APIKey, i, "api_response_raw", outputDir)
		}

		//openai api should have responded to our request with a json text that can be used as resumedata input. extract it.
		var apiResponse APIResponse
		err = json.Unmarshal([]byte(output), &apiResponse)
		if err != nil {
			fmt.Printf("Error deserializing API response: %v\n", err)
			return
		}

		//Extract the message content
		if len(apiResponse.Choices) == 0 {
			fmt.Println("No choices found in the API response")
			return
		}

		content := apiResponse.Choices[0].Message.Content

		err = validateJSON(content)
		if err != nil {
			fmt.Printf("Error validating JSON content: %v\n", err)
			return
		}
		log.Printf("Got %d bytes of JSON content (at least well formed enough to be decodable) out of that last response\n", len(content))

		// Step 5: Write the validated content to the filesystem in a way the resume projects json server can read it, plus locally for posterity.
		// Assuming the file path is up and outside of the project directory
		// Example: /home/user/output/validated_content.json
		outputFilePath := filepath.Join("../ReactResume/resumedata/", fmt.Sprintf("attempt%d.json", i))
		err = writeValidatedContent(content, outputFilePath)
		if err != nil {
			log.Printf("Error writing content to file: %v\n", err)
			return
		}
		log.Println("Content successfully written to:", outputFilePath)

		// Example: /home/user/output/validated_content.json
		localOutfilePath := filepath.Join(outputDir, fmt.Sprintf("attempt%d.json", i))
		err = writeValidatedContent(content, localOutfilePath)
		if err != nil {
			log.Printf("Error writing content to file: %v\n", err)
			return
		}
		log.Println("Content successfully written to:", localOutfilePath)

		//we should be able to render that updated content proposal now via gotenberg + ghostscript
		err = makePDFRequestAndSave(i, layout, outputDir)
		if err != nil {
			log.Printf("Error: %v\n", err)
		}

		//and the ghostscript dump to pngs ... one thing i need to do first tho is make my own ghostscript image i think b/c using a 'vuln' one out of laziness is probably horribly bad.
		// MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd)/output:/workspace minidocks/ghostscript:latest gs -sDEVICE=pngalpha -o /workspace/out-%03d.png -r144 /workspace/attempt.pdf
		err = dumpPDFToPNG(i, outputDir)
		if err != nil {
			log.Printf("Error during pdf to image dump: %v\n", err)
			break
		}

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
				tryPrompt = fmt.Sprintf("Too long, reduce by %d%%, by making minimal edits to the prior output as possible. Sometimes going overboard on skills makes it too long. Remember to make the candidate still look great in relation to the Job Description supplied earlier!", reduceByPct)
			}
		} else if result.NumberOfPages == 1 && result.LastPageContentRatio < acceptable_ratio {
			log.Println("make it longer ...")
			tryNewPrompt = true
			//tryPrompt = fmt.Sprintf("Not long enough when rendered, was only %d%% of the page. Fill it up to between %d%% and 95%%. You can bulk up the content of existing project descriptions, add new projects within companies or by beefing up the skills list. Remember to make the candidate look even greater in relation to the Job Description supplied earlier!", int(result.LastPageContentRatio*100), int(acceptable_ratio*100))
			increaseByPct := int((95.0 - result.LastPageContentRatio*100) / 2) //wat? idk smthin like this anyway.
			tryPrompt = fmt.Sprintf("Not long enough, increase by %d%%, you can bulk up the content of existing project descriptions, add new projects within companies or by beefing up the skills list. Remember to make the candidate look even greater in relation to the Job Description supplied earlier!", increaseByPct)

			//try to make it longer!!! - include the assistants last message in the new prompt so it can see what it did
		} else if result.NumberOfPages == 1 && result.LastPageContentRatio >= acceptable_ratio {
			log.Printf("over %d%% and still on one page? nice. we should stop (determined complete after attempt index %d).\n", int(acceptable_ratio*100), i)
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
		//messages = append(messages, )
		////and prompt for a length adjustment as required. this is going to be experimental for a bit i think.
		//messages = append(messages, map[string]interface{})
		//data["messages"] = messages

		//fmt.Println(content)
		//break
		_ = i
		_ = output
	}

	// Step 6: Marshal the request body into JSON
	finalJSON, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Failed to marshal final JSON: %v", err)
	}

	// Print the final JSON request body
	fmt.Println(string(finalJSON))

	//fmt.Println("example possible output:")
	//fmt.Println(input.ExpectResponse)

	//test1 := "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Software Engineer specializing in blockchain technology and data management\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Expert in Python and SQL for data-intensive environments; adept at data extraction and organization from blockchain sources.\",\n        \"Strong software engineering principles with a focus on code readability, testing, and API development.\",\n        \"Proficient in backend development with Go, Python, and Rust, particularly in cryptocurrency applications.\",\n        \"Experience with Docker, Kubernetes, and CI/CD pipelines to ensure efficient deployment and maintenance.\",\n        \"Familiar with blockchain technology and eager to explore decentralized solutions.\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed and maintained a crucial API for aggregating blockchain transaction data, ensuring data integrity and adherence to SLAs while enhancing performance.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Go, Docker, APIs\"\n                },\n                {\n                    \"desc\": \"Created a custom Terraform provider for syncing resources with blockchain services, facilitating critical operations for institutional clients.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Implemented a comprehensive data extraction process for public blockchain sources, enabling efficient analysis and reporting.\",\n                    \"sortdate\": \"2023-01-01\",\n                    \"tech\": \"Go, Docker, SQL\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Led the development of decentralized applications utilizing blockchain technology, including a Bitcoin Lightning Network donation system and a hosted Bitcoin wallet.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Go, Python, JavaScript, Docker, IPFS\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Extensive experience with Python and SQL in data-heavy environments, particularly for blockchain data.\",\n        \"Strong problem-solving abilities with an emphasis on data integrity and performance optimization.\",\n        \"Self-motivated and organized, with a passion for decentralized technology and innovation.\"\n    ]\n}"

	//test2 and 3 below were defective probably due to using the prior output as input in the resume! oops (it was eating its own tail)
	//test2 := "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Dedicated Software Engineer specializing in blockchain technology and data management, with a passion for decentralized solutions and a commitment to enhancing data integrity across platforms.\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Expert in Python and SQL for data-intensive environments; adept at data extraction and organization from blockchain sources, ensuring data accuracy and reliability.\",\n        \"Strong software engineering principles with a focus on code readability, testing, and API development to support scalable solutions.\",\n        \"Proficient in backend development with Go, Python, and Rust, particularly in cryptocurrency applications that optimize transaction flow and data management.\",\n        \"Experience with Docker, Kubernetes, and CI/CD pipelines to ensure efficient deployment and maintenance of applications, enhancing operational workflows.\",\n        \"Familiar with blockchain technology, eager to explore decentralized solutions and contribute to innovative projects within the crypto space.\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed and maintained a crucial API for aggregating blockchain transaction data, ensuring data integrity and adherence to SLAs while enhancing performance for institutional clients.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Go, Docker, APIs\"\n                },\n                {\n                    \"desc\": \"Created a custom Terraform provider for syncing resources with blockchain services, facilitating critical operations for institutional clients and boosting transactional efficiency.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Implemented a comprehensive data extraction process for public blockchain sources, enabling efficient analysis and reporting, and significantly improving data accessibility for downstream applications.\",\n                    \"sortdate\": \"2023-01-01\",\n                    \"tech\": \"Go, Docker, SQL\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Led the development of decentralized applications utilizing blockchain technology, including a Bitcoin Lightning Network donation system and a hosted Bitcoin wallet, contributing to significant advancements in peer-to-peer transactions.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Go, Python, JavaScript, Docker, IPFS\"\n                },\n                {\n                    \"desc\": \"Designed and implemented innovative solutions for data management and extraction in various projects, enhancing data flows and user experiences across applications.\",\n                    \"sortdate\": \"2015-01-01\",\n                    \"tech\": \"Python, SQL, AWS\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors, demonstrating a strong foundation in software development principles and practices.\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Extensive experience with Python and SQL in data-heavy environments, particularly for blockchain data management and analysis.\",\n        \"Strong problem-solving abilities with an emphasis on data integrity and performance optimization in software applications.\",\n        \"Self-motivated and organized, with a passion for decentralized technology and innovation, committed to driving advancements in the cryptocurrency industry.\"\n    ]\n}"
	//test3 := "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Dedicated Software Engineer specializing in blockchain technology and data management, passionate about decentralized solutions and committed to enhancing data integrity across platforms.\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Expert in Python and SQL for data-intensive environments; adept at data extraction and organization from blockchain sources, ensuring data accuracy and reliability.\",\n        \"Strong software engineering principles with a focus on code readability, testing, and API development to support scalable solutions.\",\n        \"Proficient in backend development with Go, Python, and Rust, particularly in cryptocurrency applications that optimize transaction flow and data management.\",\n        \"Experience with Docker, Kubernetes, and CI/CD pipelines to ensure efficient deployment and maintenance of applications, enhancing operational workflows.\",\n        \"Familiar with blockchain technology, eager to explore decentralized solutions and contribute to innovative projects within the crypto space.\",\n        \"Strong problem-solving skills with a proven ability to track down public source code and identify necessary data sources for analysis.\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed and maintained a crucial API for aggregating blockchain transaction data, ensuring data integrity and adherence to SLAs while enhancing performance. This involved collaborating with cross-functional teams to refine data collection processes and improve overall service delivery for institutional clients.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Go, Docker, APIs\"\n                },\n                {\n                    \"desc\": \"Created a custom Terraform provider for syncing resources with blockchain services, facilitating critical operations for institutional clients and boosting transactional efficiency. This project significantly streamlined resource management and improved compliance with internal policies.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Implemented a comprehensive data extraction process for public blockchain sources, enabling efficient analysis and reporting, and significantly improving data accessibility for downstream applications. This included documentation of extraction methodologies for future reference and training.\",\n                    \"sortdate\": \"2023-01-01\",\n                    \"tech\": \"Go, Docker, SQL\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Led the development of decentralized applications utilizing blockchain technology, including a Bitcoin Lightning Network donation system and a hosted Bitcoin wallet. These projects not only showcased my technical expertise but also contributed to significant advancements in peer-to-peer transactions, emphasizing security and user experience.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Go, Python, JavaScript, Docker, IPFS\"\n                },\n                {\n                    \"desc\": \"Designed and implemented innovative solutions for data management and extraction in various projects, enhancing data flows and user experiences across applications. My work involved optimizing SQL queries for performance and ensuring that data pipelines were both robust and scalable.\",\n                    \"sortdate\": \"2015-01-01\",\n                    \"tech\": \"Python, SQL, AWS\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors, demonstrating a strong foundation in software development principles and practices.\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Extensive experience with Python and SQL in data-heavy environments, particularly for blockchain data management and analysis.\",\n        \"Strong problem-solving abilities with an emphasis on data integrity and performance optimization in software applications.\",\n        \"Self-motivated and organized, with a passion for decentralized technology and innovation, committed to driving advancements in the cryptocurrency industry.\",\n        \"Effective communicator with the ability to collaborate with cross-functional teams, ensuring project alignment with organizational goals.\",\n        \"Adept at tracking blockchain data trends and capable of identifying necessary data sources for comprehensive analysis.\"\n    ]\n}"

	//that was a better result with an adjusted prompt that said make sure to have 3-5 companies etc
	//test4 := "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Software Engineer with a passion for blockchain technology\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Proficient in Python and SQL in data-intensive environments\",\n        \"Strong software engineering principles including code readability and testing\",\n        \"Expertise in aggregating and analyzing data from multiple sources including blockchains\",\n        \"Familiar with Docker, Kubernetes, and modern tech stacks including Golang, Airflow, and SQLAlchemy\",\n        \"Experienced in developing and maintaining APIs for data management and integration\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed and maintained a custom Terraform provider to sync resources with a third-party accounting service via web API, facilitating seamless data flow for key business segments.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Oversaw enhancements to a crucial internal tool in Go that interacts with Nomad and GitLab APIs, significantly improving deployment accuracy and integrating monitoring metrics.\",\n                    \"sortdate\": \"2022-06-01\",\n                    \"tech\": \"Go, Docker, Nomad, Prometheus\"\n                },\n                {\n                    \"desc\": \"Unified testing frameworks across multiple repositories, creating a consistent CI pipeline with Docker Compose to enhance test reliability and efficiency.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Bash scripting, Docker, CI pipelines\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed innovative blockchain-based solutions, including a decentralized social network and Bitcoin Lightning Network integrations, demonstrating a deep understanding of cryptocurrency technologies.\",\n                    \"sortdate\": \"2019-04-27\",\n                    \"tech\": \"Go, Nginx, JavaScript, HTML5, LND\"\n                },\n                {\n                    \"desc\": \"Created a proof-of-concept for a censorship-resistant social media platform utilizing IPFS and blockchain data structures, showcasing expertise in decentralized technologies.\",\n                    \"sortdate\": \"2021-03-15\",\n                    \"tech\": \"Go, IPFS, Docker\"\n                }\n            ]\n        },\n        {\n            \"company\": \"PerfectServe/Telmediq\",\n            \"tag\": \"tmq\",\n            \"companydesc\": \"Clinical Communication and Collaboration\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Software Engineer\",\n            \"daterange\": \"Oct 2019 - Nov 2021\",\n            \"sortdate\": \"2021-08-15\",\n            \"projects\": [\n                {\n                    \"desc\": \"Engineered a robust interactive SMS-based survey microservice using Django/Python and Twilio, significantly improving patient follow-up processes and data collection.\",\n                    \"tech\": \"Django/Python, Twilio, Kubernetes\"\n                }\n            ]\n        },\n        {\n            \"company\": \"Go2mobi\",\n            \"tag\": \"go2\",\n            \"companydesc\": \"Mobile Advertising Self-Serve DSP\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"Nov 2012 - Feb 2017\",\n            \"sortdate\": \"2017-02-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Built a high-performance Real Time Bidder in Go, processing over 500,000 queries per second, demonstrating ability to handle large-scale data operations.\",\n                    \"tech\": \"Go, Python, AWS, RabbitMQ\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Data extraction and organization from various blockchains\",\n        \"Developing reliable APIs to ensure data integrity\",\n        \"Strong analytical skills with a focus on data-driven decision making\",\n        \"Self-motivated, organized, and effective communicator\"\n    ]\n}"

	//test5 := "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Software Engineer\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Analyzing and solving complex problems; Proficient in self-directed learning; Adaptable to new languages, tools and frameworks; Experienced in both back-end and front-end development environments\",\n        \"Developing backend solutions, integrations and microservices; With Go, Python, Rust and Node.js\",\n        \"Building and supporting web apps; With HTML5, Javascript/ES6, React, REST, SCSS\",\n        \"Handling data; With Postgres, MySQL, Redis, AMQP and others\",\n        \"Working with a variety of tools; Experienced with Docker, AWS, Kubernetes, JetBrains IDEs, CI systems and more\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed a custom Terraform provider to sync resources with a 3rd party accounting service via their web API, facilitating the launch of key business segments.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Unified isolated testing frameworks in Bash and Rust across 13 repositories encompassing the entire core backend web service, creating a consistent Gitlab CI pipeline and test environment with Docker Compose, improving test reliability and workflow efficiency.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Bash scripting, Docker, Docker Compose, CI pipelines\"\n                },\n                {\n                    \"desc\": \"Implemented an integration of in-house end-to-end and isolated testing tooling to work with the Gmail API (Rust), to facilitate automated email testing using real email, enabling new capabilities and use cases with the existing testing framework for several teams within the organization.\",\n                    \"sortdate\": \"2023-01-01\",\n                    \"tech\": \"Gmail API, Rust, GitLabCI\"\n                }\n            ]\n        },\n        {\n            \"company\": \"PerfectServe/Telmediq\",\n            \"tag\": \"tmq\",\n            \"companydesc\": \"Clinical Communication and Collaboration\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Software Engineer\",\n            \"daterange\": \"Oct 2019 - Nov 2021\",\n            \"sortdate\": \"2021-08-15\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed an interactive SMS-based survey microservice for patient follow-up, utilizing Django/Python and integrating Twilio for efficient SMS communication, as a drop-in replacement for a vendor product, realizing tens of thousands of dollars of monthly operational cost savings.\",\n                    \"tech\": \"Django/Python, Twilio, Postgres, Kubernetes\"\n                },\n                {\n                    \"desc\": \"Engineered robust integrations with third-party scheduling system APIs using Python, ensuring seamless synchronization between physician schedules and the existing client IT infrastructure.\",\n                    \"tech\": \"Django/Python, APIs, Spinnaker, Kubernetes\"\n                }\n            ]\n        },\n        {\n            \"company\": \"Go2mobi\",\n            \"tag\": \"go2\",\n            \"companydesc\": \"Mobile Advertising Self-Serve DSP\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"Nov 2012 - Feb 2017\",\n            \"sortdate\": \"2017-02-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Built and maintained mobile advertising Real Time Bidder in Python, later transitioned to Go. Handled 500,000 queries per second on a cluster of 15 servers.\",\n                    \"tech\": \"Go, Python, AWS, RabbitMQ, Redis, MySQL+Postgres\"\n                },\n                {\n                    \"desc\": \"Developed processes for ETL and reporting the hundreds of millions of records per day generated by the bidder, providing key decision-making data for successful advertising campaign execution.\",\n                    \"tech\": \"Python, RabbitMQ, MySQL, Redshift+RDS\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed a decentralized social network system built on IPFS, a Bitcoin Lightning Network donation invoice system for a website, and a captcha solving game for 4chan users, showcasing my adaptability and proficiency with a variety of technologies and platforms.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Go, Docker, Nginx, Git, Python, JavaScript, Node.js, ES6, Perl, HTML5, React, MySQL, Redis, Asterisk, IPFS, LND, Bitcoind, VSCode, Github Actions\",\n                    \"github\": \"https://github.com/BladeMcCool/ReactResume\",\n                    \"location\": \"Remote\",\n                    \"jobtitle\": \"Open Source Developer\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Solving complex problems involving dynamic systems with many moving parts\",\n        \"Self-learning, adapting to new tech across full-stack development\",\n        \"Back-end development with Go, Python, Rust, Node.js\",\n        \"Front-end web app development using React/JSX/HTML5, ES6/Javascript, REST, SCSS\",\n        \"Data management with Postgres, MySQL, Redis, AMQP\",\n        \"Proficient with Docker, JetBrains IDEs, CI systems\",\n        \"Experienced with AWS, Kubernetes\"\n    ]\n}"
	//test6 := "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Software Engineer specializing in Blockchain and Data Solutions\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Expert in Python and SQL, with extensive experience in data-intensive environments\",\n        \"Proficient in developing robust back-end solutions, integrations, and microservices with Go and Python\",\n        \"Strong understanding of blockchain technology and experience with smart contracts and APIs\",\n        \"Skilled in Docker and Kubernetes for container orchestration and deployment\",\n        \"Exceptional problem-solving skills with a focus on data integrity and process documentation\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed a custom Terraform provider to sync resources with third-party accounting services, enhancing operational efficiency and supporting institutional clients in managing their crypto assets.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Implemented improvements to an internal production deployment tool in Go, integrating blockchain metrics with Prometheus for enhanced monitoring and reliability, ensuring data integrity across deployments.\",\n                    \"sortdate\": \"2022-06-01\",\n                    \"tech\": \"Go, Docker, Nomad, Prometheus, Grafana\"\n                },\n                {\n                    \"desc\": \"Unified isolated testing frameworks across multiple repositories, improving the CI pipeline and ensuring seamless integration of blockchain data processes.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Bash scripting, Docker, CI pipelines\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Blockchain Solutions Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Pioneered a Bitcoin Lightning Network donation system, integrating blockchain payment solutions that facilitate secure transactions for charitable contributions.\",\n                    \"sortdate\": \"2019-04-27\",\n                    \"tech\": \"Go, Nginx, JavaScript, HTML5, LND, Bitcoind\"\n                },\n                {\n                    \"desc\": \"Created a decentralized social network prototype leveraging blockchain data structures, enhancing user privacy and data security.\",\n                    \"sortdate\": \"2021-03-15\",\n                    \"tech\": \"Go, ES6, IPFS, Docker\"\n                },\n                {\n                    \"desc\": \"Designed a hosted Bitcoin wallet system allowing users to interact via touch-tone telephone, showcasing innovation in blockchain accessibility.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Perl, Asterisk, Bitcoind\"\n                }\n            ]\n        },\n        {\n            \"company\": \"PerfectServe/Telmediq\",\n            \"tag\": \"tmq\",\n            \"companydesc\": \"Clinical Communication and Collaboration\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Software Engineer\",\n            \"daterange\": \"Oct 2019 - Nov 2021\",\n            \"sortdate\": \"2021-08-15\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed an innovative SMS-based survey service for patient follow-up, integrating Twilio and ensuring seamless data flow for healthcare professionals.\",\n                    \"tech\": \"Django/Python, Twilio, Kubernetes\"\n                },\n                {\n                    \"desc\": \"Engineered integrations with third-party scheduling systems, enabling efficient data synchronization and improving client IT infrastructure.\",\n                    \"tech\": \"Django/Python, APIs, Kubernetes\"\n                }\n            ]\n        },\n        {\n            \"company\": \"Go2mobi\",\n            \"tag\": \"go2\",\n            \"companydesc\": \"Mobile Advertising Self-Serve DSP\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"Nov 2012 - Feb 2017\",\n            \"sortdate\": \"2017-02-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed a high-performance Real Time Bidder handling extensive data transactions, optimizing query speed and reliability in a competitive environment.\",\n                    \"tech\": \"Go, Python, AWS, RabbitMQ\"\n                },\n                {\n                    \"desc\": \"Implemented ETL processes for analyzing vast datasets, providing actionable insights for advertising strategies.\",\n                    \"tech\": \"Python, MySQL, Redshift\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Expert in Python and SQL for data aggregation and analysis\",\n        \"Proficient in blockchain technology and decentralized applications\",\n        \"Experienced with Docker and Kubernetes for cloud-based solutions\",\n        \"Strong problem-solving skills with a focus on data integrity\",\n        \"Excellent communication and documentation abilities\"\n    ]\n}"
	//fmt.Println("some test result output")
	//fmt.Println(test6)

}

func takeNotesOnJD(input *Input, outputDir string) error {
	//JDResponseFormat, err := os.ReadFile(filepath.Join("response_templates", "jdinfo.json"))
	jDResponseSchemaRaw, err := os.ReadFile(filepath.Join("response_templates", "jdinfo-schema.json"))
	if err != nil {
		log.Fatalf("failed to read expect_response.json: %v", err)
	}
	// Validate the JSON content
	jDResponseSchema, err := decodeJSON(string(jDResponseSchemaRaw))
	if err != nil {
		return err
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
	return nil
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
	} else {
		log.Printf("couldnt read a prompt from the file system so will use a default one.")
	}

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
	return strings.Join([]string{
		"The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. ",
		"Your output JSON can simply omit anything which need not be seen in the rendered resume document (If all of the projects within a job are marked as hidden then the whole job will be hidden). ",
		"The work_history contains a list of companies and projects within those companies. ",
		"Feel free to adjust any descriptive text fields at the company or project level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. ",
		"Embellishment of anything remotely factual or possibly tangential is encouraged. ",
		"Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to unhide it. Part of the goal is to keep the length of the final render at one page, while showing the most relevant information which makes the candidate appear a perfect fit for the target job from being hidden. ",
		"Be sure to include between 3 and 5 distinct company sections. Each company section can list separate projects within it, aim for 2-3 projects within each company. Make sure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed. ",
		"The target Job Description for which this candidate should appear to perfectly match is below. Pay special attention to any magic tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability:\n",
	}, ""), nil
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

func dumpPDFToPNG(attempt int, outputDir string) error {
	// Get the current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting current directory: %v\n", err)
	}

	// Construct the output directory path
	outputDirFullpath := filepath.Join(currentDir, outputDir)

	//could maybe check the pdf for not containing error stuff like "Uncaught runtime errors" before proceeding.
	cmd := exec.Command("docker", "run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", outputDirFullpath),
		"minidocks/ghostscript:latest",
		"gs",
		"-sDEVICE=txtwrite",
		"-o", "/workspace/pdf-txtwrite.txt",
		fmt.Sprintf("/workspace/attempt%d.pdf", attempt),
	)
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

func makePDFRequestAndSave(attempt int, layout, outputDir string) error {
	// Step 1: Create a new buffer and a multipart writer
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Step 2: Add the "url" field to the multipart form
	urlField, err := writer.CreateFormField("url")
	if err != nil {
		return fmt.Errorf("failed to create form field: %v", err)
	}
	_, err = io.WriteString(urlField, fmt.Sprintf("http://host.docker.internal:3000/?resumedata=attempt%d&layout=%s", attempt, layout))
	if err != nil {
		return fmt.Errorf("failed to write to form field: %v", err)
	}

	// Step 3: Close the multipart writer to finalize the form data
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Step 4: Create a new POST request with the multipart form data
	req, err := http.NewRequest("POST", "http://localhost:80/forms/chromium/convert/url", &requestBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	// Step 5: Set the Content-Type header
	req.Header.Set("Content-Type", writer.FormDataContentType())

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
func ReadInput(dir, layout string) (*Input, error) {
	// Define file paths
	jdFilePath := filepath.Join(dir, "jd.txt")
	expectResponseFilePath := filepath.Join("response_templates", fmt.Sprintf("%s-schema.json", layout))
	apiKeyFilePath := filepath.Join(dir, "api_key.txt")

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
		ExpectResponseSchema: expectResponseSchema,
		APIKey:               apiKey,
	}, nil
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
