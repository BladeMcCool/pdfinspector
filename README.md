# pdfinspector

## Overview

`pdfinspector` is a project designed to tune baseline resume JSON data into a perfect Job-Description-Matching single page PDF. After submitting a job description and base resume data to the service, it gathers metadata from OpenAI, then generates and iterates on suggested resume content based on user-provided or default prompts. OpenAI helps refine the resume to match the job description, aiming for a single-page result. Once the resume fits the requirements, the final PDF is returned to the user.

## Background

I made a resume auto tuner that is designed to try to hit a perfect 1 page resume using as little or as much of actual job experience/work history/education as desired, and to be able to match it to a Job Description including being sure to insert identified keywords. It uses gpt-4o-mini via OpenAI API under the hood. I don't know if it's really good for anything other than entertainment but I wanted to share heh.

There are currently 2 layouts, a functional and a chronological, and they both have a "fluffy" style option where there is more whitespace.

Some example PDFs output, these are just for fun using fake career info:

Bubblez Huntsworth - Experienced party planner
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/5b3f3fe4-7a12-4437-aa8d-8910f7730d3f/Resume.pdf?inline=1

Pantone McStandandStare - Dedicated paint observer (fluffy)
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/351d2e84-455b-4603-95d6-77371e5730d4/Resume.pdf?inline=1

Chad Spurgington - International Arms Dealer
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/fa268485-bc2e-4085-9520-8e960edc3169/Resume.pdf?inline=1

Spice Weasel - Interstellar Spice Miner (fluffy)
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/b8c626d9-91d4-414a-9795-ae59bf0125c6/Resume.pdf?inline=1

Brock Henderson - Landscaper
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/130b51c7-8a3a-4250-b8fd-0257a91a5492/Resume.pdf?inline=1

You can use real job history and personal info in the nested baseline resume data JSON to have it just tune it up.

## Pieces of the Puzzle

### OpenAI API with structured output
We leverage the native JSON output capability of the API with some prompt engineering to iteratively steer the output as desired. Prior to resume adjustment, some metadata about the given job description is garnered via OpenAI. 

### JSON Server
The JSON server is a backend service responsible for providing the resume update attempts as JSON data. It works by retrieving the resume content updates from Google Cloud Storage (GCS), making them available to the React frontend. The frontend fetches this data to present the iterations of the resume to the user, enabling further refinement based on feedback and OpenAI suggestions.

### Ghostscript
Ghostscript is used to convert the generated PDF into images for accurate and easy inspection of the document's length. By rendering each page of the PDF as an image, pdfinspector can visually assess whether the resume meets the single-page requirement and adjust accordingly before finalizing the document. This ensures the content remains within the necessary limits without sacrificing readability or formatting.

### Gotenberg
Gotenberg is an API-driven document conversion service that converts HTML, Markdown, and URLs to PDFs using Chromium. It’s leveraged in `pdfinspector` for PDF generation from web sources.

### PdfInspector
Go based project to bring all the pieces together.

### Resume Application
The Resume Application is used for rendering a personal resume into a PDF, hosted locally on a container.

- **Running the Resume Container**:
    ```bash
    docker run --rm -p 3001:3000 -d --name resume --network my_network ghcr.io/blademccool/resume:master
    ```

## Other Notes

### Running Gotenberg Locally
To test Gotenberg locally, you can use the following command. This will run Gotenberg on port 80, disable LibreOffice routes, and enable debug-level logging.

- **Container Setup**:
    ```bash
    docker run --rm -p 80:80 -d --network my_network gotenberg/gotenberg:8 gotenberg --api-port=80 --api-timeout=10s --libreoffice-disable-routes --log-level=debug
    ```

- **PDF Rendering Example**:
    ```bash
    curl -v --request POST http://localhost:80/forms/chromium/convert/url --form url=http://host.docker.internal:3000 -o ../pdfinspector/test2.pdf
    ```

### Using Ghostscript
Ghostscript is used to manipulate PDFs, rendering them to images or extracting text from PDFs. It’s especially useful for post-processing and inspecting PDF content.
> MSYS_NO_PATHCONV is just to combat some malarky that happens to paths under git bash sometimes, probably only pertinent in that environment.

- **Render PDF to PNG**:
    ```bash
    MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd):/workspace minidocks/ghostscript:latest gs -sDEVICE=pngalpha -o /workspace/out2-%03d.png -r144 /workspace/test2.pdf
    ```

- **Extract Text from PDF**:
    ```bash
    MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd):/workspace minidocks/ghostscript:latest gs -sDEVICE=txtwrite -o /workspace/out.txt /workspace/test2.pdf
    ```

### Diagrams

[Data Flow Diagram](https://lucid.app/lucidchart/b1478c0b-9269-4361-8811-48ae522f62d3/edit?viewport_loc=-1244%2C-466%2C4146%2C2100%2C0_0&invitationId=inv_f3d323c3-033a-4dea-afdd-3ce504420352)

[Sequence Diagram](https://lucid.app/lucidchart/a7952815-83cd-4fee-a6b2-3a1d4576eac7/edit?viewport_loc=-1636%2C-1069%2C5593%2C2833%2C0_0&invitationId=inv_067f1ea1-bb1c-4b3b-b18a-576decf789f4)

### Cloud Deployment
Currently running on 4 deployed containers in GCP's Cloud Run serverless platform.

### Free Access
DM for a free API key with 100 credits to mess around with it.

### Curl request examples

Below are some example curl requests that were used to create the PDFs above. I'd be happy to provision some additional tokens on a per-person basis, if you'd like a token post a comment asking and I'll try to get back with a DM in the next little while.

If a prompt is not supplied then something along these lines will be used:
The task is to examine a Job Description and a resume data structure with the goal of adjusting the data structure such that the final rendered resume presents the perfect candidate for the job while still keeping the final render to exactly one page. Some of the information in the candidate resume JSON data structure is currently set to be hidden in the output, but this information is available to inspire any changes which may make the candidate appear more suitable for the Job Description below. Your output JSON can simply omit anything which need not be seen in the rendered resume document. Feel free to adjust any descriptive text fields at the functional area or key contribution level with inspiration from the target Job Description to make the candidate seem more relevant in all possible ways that do not involve overt fabrications or lies. Embellishment of anything remotely factual or possibly tangential is encouraged. Information from older company projects can be applied to current jobs descriptions. If older, currently hidden work history can be made particularly relevant, feel free to include it. The goal is to keep the length of the final render at one page, while showing the most relevant information to make the candidate appear a perfect fit for the target job. Be sure to include between 3 and 5 distinct functional areas. Each functional area can list separate key contributions within it, aim for 2-3 examples within each. Ensure that all descriptive text is highly relevant to the job description in some way but still reflects the original character of the item being changed, The target Job Description for which this candidate should appear to perfectly match is below. Pay special attention to any special tokens that the job wants included in applications, or weird instructions. Be prepared to follow them to the best of your ability:

```bash
curl --location "https://pdfinspector-1025621488749.us-central1.run.app/streamjob" \
--header "Authorization: Bearer {your-bearer-token-here}" \
--header "Content-Type: application/json" \
--no-buffer -sS \
--data-raw '{
"jd":"party planner - must have planned events with catering for at least 10 thousand people over multi-day festivals. must be familiar with event promotion and light security duties.",
"baseline_json":"{\"layout\":\"functional\",\"education\":[{\"description\":\"Education Description\",\"graduated\":\"Graduation Date\",\"institution\":\"Institution Name\",\"location\":\"Institution Location\",\"notes\":[\"Notes about education\"]}],\"employment_history\":[{\"company\":\"Company Name\",\"daterange\":\"Date Range\",\"location\":\"Company Location\",\"title\":\"Job Title\"}],\"functional_areas\":[{\"key_contributions\":[{\"company\":\"Company Name\",\"daterange\":\"Date Range\",\"description\":\"Description of key contributions\",\"lead_in\":0,\"tech\":[\"Technology\/Skills\"]}],\"title\":\"Functional Area Title\"}],\"overview\":\"Overview or Summary Text\",\"personal_info\":{\"email\":\"email@example.com\",\"github\":\"https:\/\/github.com\/placeholder\",\"linkedin\":\"https:\/\/linkedin.com\/in\/placeholder\",\"location\":\"Location Placeholder\",\"name\":\"Full Name\",\"phone\":\"123-456-7890\"}}",
"prompt":"replace the resume data contents with a detailing of fake, but real-sounding work history experiences for candidate named \"Bubblez Huntsworth\" that relate to the Job Description, while working at fake related companies during the time period 2001 to 2024. 4 different functional areas with at least a few key contributions under each. this invented info should make the candidate look like the perfect match for the Job Description. the education should be limited to a single institution: Harvard, with a fake but related Phd. keep the education section short. keep the overview fairly short. keep the email address and linkedin very short. if we need to adjust the length of your proposed content later, focus on tuning the functional areas and key contributions."
}'
```
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/5b3f3fe4-7a12-4437-aa8d-8910f7730d3f/Resume.pdf

***

```bash
curl --location "https://pdfinspector-1025621488749.us-central1.run.app/streamjob" \
--header "Authorization: Bearer {your-bearer-token-here}" \
--header "Content-Type: application/json" \
--no-buffer -sS \
--data-raw '{
"style_override":"fluffy",
"jd":"paint watcher - must be versed in watching paint dry and putting up with bullshit",
"baseline_json":"{\"layout\":\"functional\",\"education\":[{\"description\":\"Education Description\",\"graduated\":\"Graduation Date\",\"institution\":\"Institution Name\",\"location\":\"Institution Location\",\"notes\":[\"Notes about education\"]}],\"employment_history\":[{\"company\":\"Company Name\",\"daterange\":\"Date Range\",\"location\":\"Company Location\",\"title\":\"Job Title\"}],\"functional_areas\":[{\"key_contributions\":[{\"company\":\"Company Name\",\"daterange\":\"Date Range\",\"description\":\"Description of key contributions\",\"lead_in\":0,\"tech\":[\"Technology\/Skills\"]}],\"title\":\"Functional Area Title\"}],\"overview\":\"Overview or Summary Text\",\"personal_info\":{\"email\":\"email@example.com\",\"github\":\"https:\/\/github.com\/placeholder\",\"linkedin\":\"https:\/\/linkedin.com\/in\/placeholder\",\"location\":\"Location Placeholder\",\"name\":\"Full Name\",\"phone\":\"123-456-7890\"}}",
"prompt":"replace the resume data contents with a detailing of fake, but real-sounding work history experiences for candidate named \"Pantone McStandAndStare\" that relate to the Job Description, while working at fake related companies during the time period 2001 to 2024. 4 different functional areas with at least a few key contributions under each. this invented info should make the candidate look like the perfect match for the Job Description. the education should be limited to a single institution: Harvard, with a fake but related Phd. keep the education section short. keep the overview fairly short. keep the email address and linkedin very short. if we need to adjust the length of your proposed content later, focus on tuning the functional areas and key contributions."
}'
```
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/351d2e84-455b-4603-95d6-77371e5730d4/Resume.pdf

***

```bash
curl --location "https://pdfinspector-1025621488749.us-central1.run.app/streamjob" \
--header "Authorization: Bearer {your-bearer-token-here}" \
--header "Content-Type: application/json" \
--no-buffer -sS \
--data-raw '{
"jd":"international arms dealer - must have had success in brokering deals in dangerous war zones. the successful candidate will have multiple experiences in assisting in successful coups.",
"baseline_json":"{\"layout\":\"chrono\",\"personal_info\":{\"name\":\"Full Name\",\"email\":\"email@example.com\",\"phone\":\"123-456-7890\",\"linkedin\":\"https:\/\/linkedin.com\/in\/placeholder\",\"location\":\"Location Placeholder\",\"profile\":\"Job Title\",\"github\":\"https:\/\/github.com\/placeholder\"},\"skills\":[\"Skill 1\",\"Skill 2\",\"Skill 3\"],\"work_history\":[{\"company\":\"Company Name\",\"tag\":\"company-tag\",\"companydesc\":\"Company Description\",\"location\":\"Company Location\",\"jobtitle\":\"Job Title\",\"daterange\":\"Date Range\",\"sortdate\":\"Sort Date\",\"projects\":[{\"desc\":\"Project Description\",\"sortdate\":\"Sort Date\",\"tech\":\"Tech Stack\",\"github\":\"https:\/\/github.com\/placeholder\",\"location\":\"Project Location\",\"jobtitle\":\"Project Job Title\",\"dates\":\"Project Dates\",\"hide\":true,\"printOff\":true,\"pageBreakBefore\":true}]}],\"education_v2\":[{\"institution\":\"Institution Name\",\"location\":\"Institution Location\",\"description\":\"Degree\/Certificate Description\",\"graduated\":\"Graduation Date\",\"notes\":[\"Notes about education\"]}],\"hobbies\":[],\"jobs\":[{\"title\":\"Job Title\",\"tag\":\"job-tag\"}]}",
"prompt":"replace the resume data contents with a detailing of fake, but real-sounding work history experiences for candidate named \"Chad Spurgington\" that relate to the Job Description, while working at fake related companies during the time period 1977 to 2014. 6 companies, with several brief work experiences under each. this invented info should make the candidate look like the perfect match for the Job Description. the education should be limited to a single institution: Harvard, with a fake but related Phd. keep the education section short. if we need to adjust the length of your proposed content later, focus on tuning the work experience and skills"
}'
```
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/fa268485-bc2e-4085-9520-8e960edc3169/Resume.pdf

***

```bash
curl --location "https://pdfinspector-1025621488749.us-central1.run.app/streamjob" \
--header "Authorization: Bearer {your-bearer-token-here}" \
--header "Content-Type: application/json" \
--no-buffer -sS \
--data-raw '{
"style_override":"fluffy",
"jd":"spice miner - must be versed in intergalactic spice-based navigation and be able to carry 300 lbs continuously. bonus points for having a mother and including the word NINJA in your bio.",
"baseline_json":"{\"layout\":\"chrono\",\"personal_info\":{\"name\":\"Full Name\",\"email\":\"email@example.com\",\"phone\":\"123-456-7890\",\"linkedin\":\"https:\/\/linkedin.com\/in\/placeholder\",\"location\":\"Location Placeholder\",\"profile\":\"Job Title\",\"github\":\"https:\/\/github.com\/placeholder\"},\"skills\":[\"Skill 1\",\"Skill 2\",\"Skill 3\"],\"work_history\":[{\"company\":\"Company Name\",\"tag\":\"company-tag\",\"companydesc\":\"Company Description\",\"location\":\"Company Location\",\"jobtitle\":\"Job Title\",\"daterange\":\"Date Range\",\"sortdate\":\"Sort Date\",\"projects\":[{\"desc\":\"Project Description\",\"sortdate\":\"Sort Date\",\"tech\":\"Tech Stack\",\"github\":\"https:\/\/github.com\/placeholder\",\"location\":\"Project Location\",\"jobtitle\":\"Project Job Title\",\"dates\":\"Project Dates\",\"hide\":true,\"printOff\":true,\"pageBreakBefore\":true}]}],\"education_v2\":[{\"institution\":\"Institution Name\",\"location\":\"Institution Location\",\"description\":\"Degree\/Certificate Description\",\"graduated\":\"Graduation Date\",\"notes\":[\"Notes about education\"]}],\"hobbies\":[],\"jobs\":[{\"title\":\"Job Title\",\"tag\":\"job-tag\"}]}",
"prompt":"replace the resume data contents with a detailing of fake, but real-sounding work history experiences for candidate named \"Spice Weasel\" that relate to the Job Description, while working at fake related companies during the time period 1945 to 1977. 6 companies, with several brief work experiences under each. this invented info should make the candidate look like the perfect match for the Job Description. the education should be limited to a single institution: Harvard, with a fake but related Phd. keep the education section extremely short. if we need to adjust the length of your proposed content later, focus on tuning the work experience and skills"
}'
```
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/b8c626d9-91d4-414a-9795-ae59bf0125c6/Resume.pdf

***

```bash
# kind of a "normal" example where we supply some baseline data.
curl --location "https://pdfinspector-1025621488749.us-central1.run.app/streamjob" \
--header "Authorization: Bearer {your-bearer-token-here}" \
--header "Content-Type: application/json" \
--no-buffer -sS \
--data-raw '{
"style_override":"fluffy",
"jd":"landscaper extraordinaire - must have experience with bushes and pine trees. a particular focus on irrigation automation and robotics is required, so have some related expertise.",
"baseline_json":"{\"layout\":\"functional\",\"education\":[{\"description\":\"Diploma in Horticulture\",\"graduated\":\"June 2000\",\"institution\":\"Green Valley Technical Institute\",\"location\":\"Springfield, IL\",\"notes\":[\"Focused on landscape design, irrigation systems, and plant health management.\"]}],\"employment_history\":[{\"company\":\"Evergreen Landscaping Co.\",\"daterange\":\"2015 - Present\",\"location\":\"Springfield, IL\",\"title\":\"Senior Landscape Designer\"},{\"company\":\"Natures Touch Landscaping\",\"daterange\":\"2008 - 2015\",\"location\":\"Springfield, IL\",\"title\":\"Landscape Foreman\"},{\"company\":\"Green Thumb Landscaping Solutions\",\"daterange\":\"2000 - 2008\",\"location\":\"Springfield, IL\",\"title\":\"Junior Landscaper\"}],\"functional_areas\":[{\"key_contributions\":[{\"company\":\"Evergreen Landscaping Co.\",\"daterange\":\"2015 - Present\",\"description\":\"Led the design and installation of over 100 high-end residential and commercial landscape projects, optimizing space and improving client property values. Trained and mentored a team of 15 junior landscapers, ensuring project timelines were met while maintaining high standards of quality.\",\"lead_in\":5,\"tech\":[\"Landscape Design\",\"Hardscaping\",\"Team Management\",\"Client Consultation\",\"Irrigation Systems\"]},{\"company\":\"Natures Touch Landscaping\",\"daterange\":\"2008 - 2015\",\"description\":\"Managed landscape design projects for large residential clients, including creating custom garden layouts and overseeing the installation of patios, walkways, and retaining walls. Improved customer satisfaction by 25% through personalized design consultations.\",\"lead_in\":4,\"tech\":[\"Landscape Design\",\"Client Relations\",\"Garden Layouts\",\"Hardscaping\",\"Custom Designs\"]},{\"company\":\"Green Thumb Landscaping Solutions\",\"daterange\":\"2000 - 2008\",\"description\":\"Contributed to the design and execution of large-scale landscape renovation projects, including plant selection and layout optimization. Implemented innovative designs that improved the aesthetics and functionality of outdoor spaces.\",\"lead_in\":3,\"tech\":[\"Plant Selection\",\"Landscape Design\",\"Layout Optimization\",\"Hardscaping\"]}],\"title\":\"Landscape Design and Team Management\"},{\"key_contributions\":[{\"company\":\"Natures Touch Landscaping\",\"daterange\":\"2008 - 2015\",\"description\":\"Installed and maintained large-scale irrigation systems for commercial properties, ensuring efficient water usage and system longevity. Implemented water conservation strategies that reduced water consumption by 20% while maintaining landscape health.\",\"lead_in\":4,\"tech\":[\"Irrigation System Design\",\"Water Conservation\",\"System Maintenance\",\"Eco-Friendly Solutions\"]},{\"company\":\"Evergreen Landscaping Co.\",\"daterange\":\"2015 - Present\",\"description\":\"Developed customized irrigation systems for high-end residential properties, improving plant health and reducing water usage by up to 15%. Trained a team of junior landscapers on irrigation installation techniques and system maintenance best practices.\",\"lead_in\":5,\"tech\":[\"Irrigation System Customization\",\"Water Efficiency\",\"Team Training\",\"Plant Health Management\"]},{\"company\":\"Green Thumb Landscaping Solutions\",\"daterange\":\"2000 - 2008\",\"description\":\"Assisted in the installation of irrigation systems for residential and commercial clients, focusing on optimizing water distribution and reducing maintenance costs.\",\"lead_in\":3,\"tech\":[\"Irrigation System Installation\",\"Water Distribution\",\"Maintenance Cost Reduction\"]}],\"title\":\"Irrigation Systems and Water Management\"},{\"key_contributions\":[{\"company\":\"Evergreen Landscaping Co.\",\"daterange\":\"2015 - Present\",\"description\":\"Oversaw the logistical planning for multiple concurrent landscaping projects, coordinating the delivery of materials, scheduling equipment usage, and managing a team of landscapers. Reduced project delays by 20% through improved scheduling and resource allocation.\",\"lead_in\":5,\"tech\":[\"Logistics Planning\",\"Resource Allocation\",\"Team Management\",\"Project Scheduling\"]},{\"company\":\"Natures Touch Landscaping\",\"daterange\":\"2008 - 2015\",\"description\":\"Managed the logistics for large-scale commercial landscaping projects, ensuring the timely delivery of materials and equipment. Coordinated with suppliers to reduce delivery times and streamline operations, cutting project completion times by 15%.\",\"lead_in\":4,\"tech\":[\"Logistics Coordination\",\"Supplier Management\",\"Time Management\",\"Project Efficiency\"]},{\"company\":\"Green Thumb Landscaping Solutions\",\"daterange\":\"2000 - 2008\",\"description\":\"Led the logistics team responsible for planning and executing complex landscaping operations, including the coordination of multiple crews and the delivery of heavy equipment. Successfully reduced project downtime by optimizing equipment usage and personnel scheduling.\",\"lead_in\":3,\"tech\":[\"Crew Coordination\",\"Heavy Equipment Management\",\"Project Downtime Reduction\",\"Scheduling Optimization\"]}],\"title\":\"Logistics and Resource Management\"}],\"overview\":\"Brock Henderson is a seasoned landscaper with over 20 years of experience working for some of the top landscaping companies in Springfield, IL. He specializes in designing and managing large-scale landscaping projects, with expertise in sustainable landscaping, irrigation systems, and logistics management. Brock is passionate about transforming outdoor spaces into functional and beautiful environments while adhering to eco-friendly practices.\",\"personal_info\":{\"email\":\"brock.henderson@landscapespringfield.com\",\"github\":\"https:\/\/github.com\/brock-henderson-landscapes\",\"linkedin\":\"https:\/\/linkedin.com\/in\/brock-henderson-landscaper\",\"location\":\"Springfield, IL\",\"name\":\"Brock Henderson\",\"phone\":\"777-555-8484\"}}"
}'
```
> https://pdfinspector-tzoh77a45q-uc.a.run.app/joboutput/130b51c7-8a3a-4250-b8fd-0257a91a5492/Resume.pdf

### License 
GPL License, I guess. No warranty, bla bla. DWTFYW in spirit tho.