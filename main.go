package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png" // Importing PNG support
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {

	// Example call to ReadInput
	inputDir := "input" // The directory where jd.txt and expect_response.json are located

	input, err := ReadInput(inputDir)
	if err != nil {
		log.Fatalf("Error reading input files: %v", err)
	}
	_ = input

	// get JSON of the current complete resume including all the hidden stuff, this hits an express server that imports the reactresume resumedata.mjs and outputs it as json.
	resp, err := http.Get("http://localhost:3002")
	if err != nil {
		log.Fatalf("Failed to make the HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the response body: %v", err)
	}
	fmt.Printf("got %d bytes of json from the json-server\n", len(body))

	//some_jd := "Building the Future of Crypto \nOur Krakenites are a world-class team with crypto conviction, united by our desire to discover and unlock the potential of crypto and blockchain technology.\n\nWhat makes us different?\n\nKraken is a mission-focused company rooted in crypto values. As a Krakenite, you’ll join us on our mission to accelerate the global adoption of crypto, so that everyone can achieve financial freedom and inclusion. For over a decade, Kraken’s focus on our mission and crypto ethos has attracted many of the most talented crypto experts in the world.\n\nBefore you apply, please read the Kraken Culture page to learn more about our internal culture, values, and mission. We also expect candidates to familiarize themselves with the Kraken app. Learn how to create a Kraken account here.\n\nAs a fully remote company, we have Krakenites in 70+ countries who speak over 50 languages. Krakenites are industry pioneers who develop premium crypto products for experienced traders, institutions, and newcomers to the space. Kraken is committed to industry-leading security, crypto education, and world-class client support through our products like Kraken Pro, Kraken NFT, and Kraken Futures.\n\nBecome a Krakenite and build the future of crypto!\n\nProof of work\nThe team\nAt Kraken Digital Asset Exchange, the Staked team, now part of Kraken Institutional since Kraken's acquisition in December 2021, specializes in enabling non-custodial staking solutions tailored for institutional clients. Staked offers staking services across 40+ proof-of-stake (PoS) blockchains, managing billions in delegated assets. Trusted by top investors and cryptocurrency exchanges, Staked ensures optimal staking rewards.\n\nAs a member of our team, you will play a crucial role in extracting and organizing data from various blockchains preparing it for downstream analysis. Your work will maintain our high standards for data integrity and support our mission to deliver top-notch staking solutions to institutional clients. Join us to drive innovation in the cryptocurrency industry with a collaborative and curious mindset\n\nThe opportunity\nExtract and organize data from multiple blockchains using RPC, API, or smart contract calls\n\nDevelop and maintain the software stack for collecting and indexing transaction data for efficient consumption\n\nMaintain a reliable API to provide comprehensive account histories and ensure data is auditable and meets SLAs\n\nTrack and monitor public blockchain sources and find necessary data for analysis\n\nDocument data extraction processes and methodologies\n\nCollaborate with internal teams to ensure seamless integration and data flow\n\nSkills you should HODL\n3-5+ years of experience with Python and SQL, in data-intensive environments\n\nStrong experience with software engineering principles, code readability, and testing\n\nProficiency in aggregating and analyzing data\n\nExperience with SQL databases and query performance\n\nFamiliarity with Docker, Kubernetes, and other elements of our tech stack such as Typescript, Golang, Airflow, SQLAlchemy\n\nKnowledge of basic web protocols and experience with API development is a plus\n\nAbility to track down public source code and identify necessary data sources\n\nExperience with blockchain nodes and eagerness to analyze blockchain data\n\nSelf-motivated, organized, and a good communicator with a problem-solving mindset\n\nPassionate about decentralized tech and eager to drive the industry forward\n\nB.S. in Computer Science or a related field, or equivalent experience"
	//_ = some_jd
	//example_possible_output_hardcody := "{\n    \"personal_info\": \"placeholder\",\n    \"key_skills\": [\n        \"Analysing and solving problems; Proficient in self-directed learning; Adaptable to new languages, tools and frameworks; Experienced in both back-end and front-end development environments\",\n        \"Developing backend solutions, integrations and microservices; With Go, Python, Rust and Node.js\",\n        \"Building and supporting web apps; With HTML5, Javascript/ES6, React, REST, SCSS\",\n        \"Handling data; With Postgres, MySQL, Redis, AMQP and others\",\n        \"Working with a variety of tools; Experienced with Docker, LLMs, AWS, Kubernetes, JetBrains IDEs, CI systems and more\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed fun and interesting projects including a decentralized social network system built on IPFS, a Bitcoin Lightning Network donation invoice system for a website, a captcha solving game for 4chan users, a hosted Bitcoin wallet that can be interacted with using touch tone telephone, and this CV in React.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Go, Docker, Nginx, Git, Python, JavaScript, Node.js, ES6, Perl, HTML5, React, MySQL, Redis, Asterisk, IPFS, LND, Bitcoind, VSCode, Github Actions\",\n                    \"github\": \"https://github.com/BladeMcCool/ReactResume\",\n                    \"location\": \"Remote\",\n                    \"jobtitle\": \"Open Source Developer\"\n                }\n            ]\n        },\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"desc\": \"Built custom Terraform provider to maintain state with web service APIs, oversaw needed improvements to custom internal production job deployment audit trail tooling, integrated testing tools with gmail API, consolidated divergent isolated test runner tooling across multiple repos with a standardized testing kit for use in CI pipelines and local execution\",\n            \"tech\": \"Go, Docker, Rust, Nomad, Vault, GitLabCI, Bash scripting, Terraform, JIRA, ChatGPT, APIs\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Oversaw improvements to a crucial internal production deployment tool in Go which interacts with Nomad and GitLab APIs, helping to completely eliminate incidents related to incorrect deployments, integrating metrics with Prometheus for monitoring, boosting test coverage, fixing bugs and implementing requested features.\",\n                    \"sortdate\": \"2022-06-01\",\n                    \"tech\": \"Go, Docker, Nomad, Prometheus, Grafana\"\n                },\n                {\n                    \"desc\": \"Developed a custom Terraform provider to sync resources with a 3rd party accounting service via their web API, facilitating the launch of key business segments.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Unified isolated testing frameworks in Bash and Rust across 13 repositories encompassing the entire core backend web service, creating a consistent Gitlab CI pipeline and test environment with Docker Compose, improving test reliability and workflow efficiency.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Bash scripting, Docker, Docker Compose, CI pipelines\"\n                },\n                {\n                    \"desc\": \"Implemented an integration of in-house end-to-end and isolated testing tooling to work with the Gmail API (Rust), to facilitate automated email testing using real email, enabling new capabilities and use cases with the existing testing framework for several teams within the organization.\",\n                    \"sortdate\": \"2023-01-01\",\n                    \"tech\": \"Gmail API, Rust, GitLabCI\"\n                }\n            ]\n        },\n        {\n            \"company\": \"PerfectServe/Telmediq\",\n            \"tag\": \"tmq\",\n            \"companydesc\": \"Clinical Communication and Collaboration\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Software Engineer\",\n            \"desc\": \"Interactive SMS-based survey microservice for patient followup, web API implemented with Django/Python, making use of Twilio for SMS communication. Integrations with third party scheduling systems to coordinate and sync physician schedules with existing client IT infrastructure. Adding features to and fixing issues in various microservices throughout the organization.\",\n            \"tech\": \"Django/Python, Go, Kubernetes, Spinnaker, Postgres, Twilio, Nginx\",\n            \"daterange\": \"Oct 2019 - Nov 2021\",\n            \"sortdate\": \"2021-08-15\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed an interactive SMS-based survey microservice for patient follow-up, utilizing Django/Python and integrating Twilio for efficient SMS communication, as a drop-in replacement for a vendor product, realizing tens of thousands of dollars of monthly operational cost savings.\",\n                    \"tech\": \"Django/Python, Twilio, Postgres, Kubernetes\"\n                },\n                {\n                    \"desc\": \"Engineered robust integrations with third-party scheduling system APIs using Python, ensuring seamless synchronization between physician schedules and the existing client IT infrastructure.\",\n                    \"tech\": \"Django/Python, APIs, Spinnaker, Kubernetes\"\n                },\n                {\n                    \"desc\": \"Contributed to the enhancement and maintenance of various microservices across the organization in Python and Go, implementing REST endpoints to serve user interface components and improving service logs.\",\n                    \"tech\": \"Django/Python, Go, Kubernetes, Spinnaker, Postgres, Redis, Nginx\"\n                }\n            ]\n        },\n        {\n            \"company\": \"Go2mobi\",\n            \"tag\": \"go2\",\n            \"companydesc\": \"Mobile Advertising Self-Serve DSP\",\n            \"location\": \"Victoria, BC\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"desc\": \"Built and maintained mobile advertising Real Time Bidder in Python, later transitioned to Go. Handled 500,000 queries per second on a cluster of 15 servers. Also developed processes for ETL and reporting the hundreds of millions of records per day generated by the bidder. Re-architected existing click tracking system for better performance.\",\n            \"tech\": \"Go, Python, Node.js, AWS, RabbitMQ, Redis, MySQL+Postgres\",\n            \"daterange\": \"Nov 2012 - Feb 2017\",\n            \"sortdate\": \"2017-02-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Built and maintained mobile advertising Real Time Bidder in Python/Tornado, later transitioned to Go. Handled 500,000 queries per second on a cluster of 15 servers.\",\n                    \"tech\": \"Go, Python, AWS, RabbitMQ, Redis, MySQL+Postgres\"\n                },\n                {\n                    \"desc\": \"Developed processes for ETL and reporting the hundreds of millions of records per day generated by the bidder, providing key decision-making data for successful advertising campaign execution.\",\n                    \"tech\": \"Python, RabbitMQ, MySQL, Redshift+RDS\"\n                },\n                {\n                    \"desc\": \"Re-architected existing click tracking system away from PHP and into async Python/Tornado for better performance, boosting CTRs.\",\n                    \"tech\": \"Python, AWS\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Solving complex problems involving dynamic systems with many moving parts\",\n        \"self-learning, adapting to new tech across full-stack development\",\n        \"back-end development with Go, Python, Rust, Node.js\",\n        \"front-end web app development using React/JSX/HTML5, ES6/Javascript, REST, SCSS\",\n        \"data management with Postgres, MySQL, Redis, AMQP\",\n        \"proficient with Docker, JetBrains IDEs, CI systems\",\n        \"experienced with AWS, Kubernetes\"\n    ]\n}"
	//fmt.Println(example_possible_output_hardcody)
	//_ = example_possible_output

	//content := strings.Join(prompt, " ") + string(body)
	//_ = content

	//prompt := "Provide feedback on the following JSON. Is it well formed? What do you think the purpose is? Tell me about things marked as hide and what that might mean. Finally, how long in terms of page count do you think the final document this feeds into is?\n\nJSON: "
	prompt_parts := []string{
		"The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. ",
		"Your output JSON can simply omit anything which need not be seen in the rendered resume document (If all of the projects within a job are marked as hidden then the whole job will be hidden). ",
		"The work_history contains a list of companies and projects within those companies. ",
		"Feel free to adjust any descriptive text fields at the company or project level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. ",
		"Embellishment of anything remotely factual or possibly tangential is encouraged. ",
		"Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to unhide it. Part of the goal is to keep the length of the final render at one page, while showing the most relevant information which makes the candidate appear a perfect fit for the target job from being hidden. ",
		"Be sure to include between 3 and 5 distinct company sections. Each company section can list separate projects within it, aim for 2-3 projects within each company. Make sure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed. ",
		"The target Job Description for which this candidate should appear to perfectly match is below:\n",
		input.JD,
		"\n--- end job description ---\n",
		"The following JSON represents the current generic 1 page resume for the candidate. Much of the information in the data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description above. ",
		"The input resume data structure JSON is below:\n",
		string(body),
	}
	prompt := strings.Join(prompt_parts, "")

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
				"role":    "system",
				"content": "Response should include only the fields expected to be rendered by the application, in well-formed JSON, such that the final render after content changes remains exactly one page.",
			},
			{
				"role":    "user",
				"content": "Show me an example output for the application to use",
			},
			{
				"role":    "assistant",
				"content": input.ExpectResponse,
			},
			{
				"role":    "user",
				"content": prompt,
			},

			//this was the output, its valid, its tuned ... fun! i mean, its also terrible and results in the output being only half a page.
			//{
			//	"role":    "assistant",
			//	"content": "{\n    \"personal_info\": {\n        \"name\": \"Chris A. Hagglund\",\n        \"email\": \"chris@chws.ca\",\n        \"phone\": \"250-532-9694\",\n        \"linkedin\": \"linkedin.com/in/1337-chris-hagglund\",\n        \"location\": \"Lethbridge AB\",\n        \"profile\": \"Software Engineer specializing in blockchain technology and data management\",\n        \"github\": \"https://github.com/BladeMcCool\"\n    },\n    \"key_skills\": [\n        \"Expert in Python and SQL for data-intensive environments; adept at data extraction and organization from blockchain sources.\",\n        \"Strong software engineering principles with a focus on code readability, testing, and API development.\",\n        \"Proficient in backend development with Go, Python, and Rust, particularly in cryptocurrency applications.\",\n        \"Experience with Docker, Kubernetes, and CI/CD pipelines to ensure efficient deployment and maintenance.\",\n        \"Familiar with blockchain technology and eager to explore decentralized solutions.\"\n    ],\n    \"work_history\": [\n        {\n            \"company\": \"Kraken\",\n            \"tag\": \"krk\",\n            \"companydesc\": \"Digital Asset Exchange\",\n            \"location\": \"Remote\",\n            \"jobtitle\": \"Software Engineer II\",\n            \"daterange\": \"Jan 2022 - Mar 2024\",\n            \"sortdate\": \"2024-03-11\",\n            \"projects\": [\n                {\n                    \"desc\": \"Developed and maintained a crucial API for aggregating blockchain transaction data, ensuring data integrity and adherence to SLAs while enhancing performance.\",\n                    \"sortdate\": \"2023-06-01\",\n                    \"tech\": \"Go, Docker, APIs\"\n                },\n                {\n                    \"desc\": \"Created a custom Terraform provider for syncing resources with blockchain services, facilitating critical operations for institutional clients.\",\n                    \"sortdate\": \"2022-01-24\",\n                    \"tech\": \"Terraform, Go, APIs\"\n                },\n                {\n                    \"desc\": \"Implemented a comprehensive data extraction process for public blockchain sources, enabling efficient analysis and reporting.\",\n                    \"sortdate\": \"2023-01-01\",\n                    \"tech\": \"Go, Docker, SQL\"\n                }\n            ]\n        },\n        {\n            \"company\": \"CHWS\",\n            \"tag\": \"chws\",\n            \"companydesc\": \"Software Development Consultancy\",\n            \"location\": \"Lethbridge, AB\",\n            \"jobtitle\": \"Senior Software Developer\",\n            \"daterange\": \"2010 - Present\",\n            \"sortdate\": \"2011-01-01\",\n            \"projects\": [\n                {\n                    \"desc\": \"Led the development of decentralized applications utilizing blockchain technology, including a Bitcoin Lightning Network donation system and a hosted Bitcoin wallet.\",\n                    \"sortdate\": \"2012-01-01\",\n                    \"tech\": \"Go, Python, JavaScript, Docker, IPFS\"\n                }\n            ]\n        }\n    ],\n    \"education_v2\": [\n        {\n            \"institution\": \"Humber College\",\n            \"location\": \"Toronto, ON\",\n            \"description\": \"3 year Computer Programmer/Analyst Diploma\",\n            \"graduated\": \"May 2002\",\n            \"notes\": [\n                \"Graduated with honors\"\n            ]\n        }\n    ],\n    \"skills\": [\n        \"Extensive experience with Python and SQL in data-heavy environments, particularly for blockchain data.\",\n        \"Strong problem-solving abilities with an emphasis on data integrity and performance optimization.\",\n        \"Self-motivated and organized, with a passion for decentralized technology and innovation.\"\n    ]\n}",
			//},
			//{
			//	"role":    "user",
			//	"content": "The resulting output was correctly formatted but resulted in a final render that occupied 25% of the page. Please make a similar attempt but target a longer overall final render length about quadruple that of the last attempt.",
			//},
		},
		"temperature": 0.7,
	}

	max_attempts := 5
	for i := range max_attempts {
		api_request_pretty, err := serializeToJSON(data)
		writeToFile(api_request_pretty, i, "api_request_pretty")
		if err != nil {
			log.Fatalf("Failed to marshal final JSON: %v", err)
		}
		exists, output, err := checkForPreexistingAPIOutput("output", "api_response_raw", i)
		if err != nil {
			log.Fatalf("Error checking for pre-existing API output: %v", err)
		}
		if !exists {
			output, err = makeAPIRequest(data, input.APIKey, i)
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

		// Step 5: Write the validated content to the filesystem
		// Assuming the file path is up and outside of the project directory
		// Example: /home/user/output/validated_content.json
		outputFilePath := filepath.Join("../ReactResume/resumedata/", "attempt.json")

		err = writeValidatedContent(content, outputFilePath)
		if err != nil {
			log.Printf("Error writing content to file: %v\n", err)
			return
		}

		log.Println("Content successfully written to:", outputFilePath)

		//fmt.Println(content)
		break
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

func makeAPIRequest(apiBody interface{}, apiKey string, counter int) (string, error) {
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
	err = writeToFile(responseString, counter, "api_response_raw")
	if err != nil {
		return "", fmt.Errorf("failed to write response to file: %v", err)
	}

	// Return the response string
	return responseString, nil
}

// writeToFile writes data to a file in the output directory with a filename based on the counter and fragment
func writeToFile(data string, counter int, filenameFragment string) error {
	// Define the output directory (hardcoded)
	outputDir := "output"

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
	JD             string
	ExpectResponse string
	APIKey         string
}

// ReadInput reads the input files from the "input" directory and returns an Input struct
func ReadInput(dir string) (*Input, error) {
	// Define file paths
	jdFilePath := filepath.Join(dir, "jd.txt")
	expectResponseFilePath := filepath.Join(dir, "expect_response.json")
	apiKeyFilePath := filepath.Join(dir, "api_key.txt")

	// Read expect_response.json
	expectResponseContent, err := os.ReadFile(expectResponseFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read expect_response.json: %v", err)
	}
	// Validate the JSON content
	if err := validateJSON(string(expectResponseContent)); err != nil {
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
		JD:             string(jdContent),
		ExpectResponse: string(expectResponseContent),
		APIKey:         apiKey,
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

//func main() {
//  //
//	// Open the PNG file
//	img, err := imaging.Open("out2-002.png")
//	if err != nil {
//		log.Fatalf("Failed to open image: %v", err)
//	}
//
//	_ = whitePct(img)
//	_ = contentEnds(img)
//}

func contentEnds(img image.Image) float64 {
	// Get image dimensions
	bounds := img.Bounds()
	//totalPixels := bounds.Dx() * bounds.Dy()

	lastColoredPixelRow := 0
	fmt.Printf("believe image dimensions are: %d x %d\n", bounds.Max.X, bounds.Max.Y)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {

			r, g, b, a := img.At(x, y).RGBA()
			if !isWhite(r, g, b, a) {
				lastColoredPixelRow = y
				fmt.Printf("Found a nonwhite/nontransparent pixel on row %d in column %d\n", y, x)
				break //no need to check any further along this line.
			}
		}
	}
	fmt.Printf("lastrow found a pixel on: %v, total rows was %v\n", lastColoredPixelRow, bounds.Max.Y)
	lastContentAt := float64(lastColoredPixelRow) / float64(bounds.Max.Y)
	fmt.Printf("last content found at %.5f of the document.", lastContentAt)
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

func isWhite(r, g, b, a uint32) bool {
	const threshold = 0.9 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
	//also .... transparent with no color == white when on screen as a pdf so ....
	//return a > 0 && float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
	return a == 0 || float64(r) >= threshold && float64(g) >= threshold && float64(b) >= threshold
}
