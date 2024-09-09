next:
    - find a layout template in a directory of them prepared already, it really should usually just change with the layout i think.
    - get some metadata about the JD in a structured way and save it with the outputs. (company name, job title, keywords list, process notes (esp if a takehome assignment mentioned), remote ok? )
        - how about shoving into the CV "github portfolio available in liu of take-home assessments. pick an existing project and lets talk about it." because i'm fucking done with this shit.
    - use output_dir in all places where needed
    - camelcase the vars names??? or whatever is go standard just be fuckin consistent omg.
    - if prompt.txt is present, use the contents, otherwise use our standard one (note, the tune-up prompts might not be generic enough to work properly in all layouts atm!)
    - find a way to test these pdfs with "workday" to see if it can extract the infos b/c if not then that may indicate potential problems.
        - Indeed, Dice too i guess
    - consider the possiblity of targeting a page length other than 1 page.
    - are we being sure that the content actually is covering:  positions, responsibilities, and key skills? (i think so)

```
if want to run the resume in docker do like:
    docker run --rm -p 3001:3000 -d --name resume --network my_network ghcr.io/blademccool/resume:master

new logix todo (update, this is all basically done at the moment -- still havent had it make the thing 3 pages so i havent dealt with more than a little bit of overflow onto a second page):
---------------
prepare initial prompto
    like currently, with system, initial user and sample response message, followed by main prompt.
    main prompt instructs basically: take this jd and this resume data and spit out data that makes the candidate perfect for the job.
    probably need to work a LOT on this prompt setup but i had some positive-ish initial result from gpt-4o-mini.
        then i burned 27 cents in one request on gpt-4 oops.
input should probably not only include the jd related params, but the sample output format as its own little json file (do a fmt.println to see it as we have been using it).
make a directory for the output with a uuid for now, maybe a iso datetime in front.
loop up to five times to get to a better resume under 1 page
    get a result of the current prompt.
    save the output as current attempt num in the dir
    plug the json into the resume via new means, do not overwrite the default data!
        so figure that out! thats the first "next thing to do i think"
        perhaps ..... some url param can tell it the path of an alternate json to source as actual just json
            we could, in our go program, just write to those files in the other project dir. (yeah gross but we'll get something better later)
    cause a gotenberg printout of the modified resume
        (something like this, need to tell it how to get the new json still, oh and is that host right? thats for a docker container not the local run)
        docker run --rm -p 80:80 -d --network my_network gotenberg/gotenberg:8 gotenberg --api-port=80 --api-timeout=10s --libreoffice-disable-routes --log-level=debug
        curl -v --request POST http://localhost:80/forms/chromium/convert/url --form url=http://host.docker.internal:3000 -o ../pdfinspector/test2.pdf
        curl -v --request POST http://localhost:80/forms/chromium/convert/url --form url="http://host.docker.internal:3000?layout=functional" -o ../pdfinspector/functional-test.pdf
        curl -v --request POST http://localhost:80/forms/chromium/convert/url --form url="http://host.docker.internal:3000?layout=functional&resumedata=articulate" -o ./"Chris Hagglund Resume.pdf"
        
        curl -v --request POST https://gotenberg-1025621488749.us-central1.run.app/forms/chromium/convert/url --form url="https://react-app-1025621488749.us-central1.run.app/?jsonserver=json-server-1025621488749.us-central1.run.app&resumedata=46564153-d10c-48d5-a6fa-8df216f798b0%2Fattempt0&layout=functional" -o ./"deployed_test.pdf"

    cause a ghostscript png render of the pdf
        (that env stuff maybe not needed if we use go program to execute it - but we'll have to fill the pwd too)
        MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd):/workspace minidocks/ghostscript:latest gs -sDEVICE=pngalpha -o /workspace/out2-%03d.png -r144 /workspace/test2.pdf
        MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd):/workspace minidocks/ghostscript:latest gs -sDEVICE=txtwrite -o /workspace/out.txt /workspace/test2.pdf
    think about how long the output is ...
    if its more than one page, how much more?
        save the output and panic when this happens b/c we havent had to deal with it but ....
        if its only one page too many ....
            how deep into the new page are we?
            if its just a little, eg (and i should change the code to find nonwhite pixels better instead of what its doing now)
                if it is no text at all on the new page:
                    we just went slightly too long and somehow caused a page break, can we remove 10-15 (or smth) words somewhere?
                if its some percent of the page:
                    a little bit too long, remove a paragraphs worth/find a way to combine two projects into one
                    ... and so on ... this part could get tricky/weird, we should find and work within the llms limitations.
        otherwise its way too long!
            just say this is x number of pages over limit please make much shorter.
    if its less than one page, how much less? we have an acceptable range probably somewhere about 90% filled
        if its in that range, lets say we've got an acceptable output, break the loop!
        otherwise,
           we should ask it to make it however much longer (eg we are only at xx percent of the page so we need to make it 1/.xx times longer)
                i wonder if that kind of logic could work for the over length scenario?
    if we did not have an acceptable output on this iter (before loopsing again)
        include the assistant message that did not give us satisfaction in the next api request, followed by a new prompto like:
            the output produced a rendering that was (xx). please change it in yy way and produce a new output that will adjust the length accordingly while still making the candidate perfect for the jd.


    we should always save something in the output dir about the length detection results of this iteration. (like a pretty-printed json file maybe? idk.)

    i think as part of auditing and attempt to understand what is going on that after we're done our attempts regardless of if successful length was achieved, we should take last version of prompt which might be as long as initial plus like five assistant respondo and intervening do-better requests.
        after the last respondo, ask it to describe how it adjusted things in the JSON. (adjust the system message and api call to make sense in the context of asking for commentary on how the final json relfects changes that make the candidate look ideal for this job, what changes etc)
        make note of what it said :)

i noted also that the profile field in the respones was getting filled with something interesting but we dont use it.

lets get the attemptx.json files also saved in the local output dir so that all the stuff for a given output is preserved in one place
clean up the logging so far
try to generate 1 complete cynical bullshit resume for another actual job, dunno if i should apply.
try to generate 1 real kind of resume for one actual job, apply for it, note it, be careful and correct.


this whole deal might work better by breaking it down and adjusting the resume in chunks based on the jd, eg series of promptos to adjust the sections for verbiage, mention its ok to take hidden experiences and use info from them to craft adjustments.

- make a functional layout that can be toggled. see https://www.reddit.com/r/recruitinghell/comments/1eo69xv/the_resume_format_that_landed_me_interviews_for/

```
strategy by Rahul
https://www.linkedin.com/posts/rahulraj90_jobs-activity-7233370876136542209-8FqV?utm_source=share&utm_medium=member_desktop

Applied to 200+ jobs.
200 rejections. No interviews.
Unfortunately, it's a story I hear often.

It turns out they were only:
-> Searching job boards and submitting them over and over.
-> Sending generic messages to the wrong people (recruiters)
-> Hoping and praying for an interview invitation

That approach simply won't work. Especially in this competitive job market.
It's like going fishing with a line but no hook on the end.
You might get extremely lucky and have fish jump straight into the boat, but more likely than not, you’ll catch nothing.

So, we need a different approach :

👉 Clarify your target role, why you want it, and why you're a fit. For example, don't assume that an SDE 2 role is the same in every organization around the world. Similarly, don't choose backend, frontend, or DevOps roles just because there are more job postings—focus on your strengths instead.

👉 Make sure you're well prepared. I've personally seen candidates who are desperate for jobs, but a good majority of them are not ready for interviews when a referral is offered.

👉 Make sure to clearly articulate your "Why" and framing your skills and experience to align with your target role

👉 Connect with people in your target role (this is key).

👉 Engage with your contacts meaningfully—avoid just saying "Hi/Hello" or using LinkedIn automated messages, as these show little effort or respect for their time. Don’t rush in asking for referrals; instead, take time to appear genuine. Not everyone will respond, but some will, and you'll connect with the best-minded people, making it a win-win!

👉Reach out to hiring managers for jobs you apply to, and remember they’re picky about resumes, so make yours stand out with unique achievements, not just solving 1000 Leetcode problems. Solving a Leetcode problem and tackling a real-world issue are very different. If a hiring manager refers you, consider it a jackpot!

👉 Use specific, personalized messages when reaching out—keep them balanced, clear, and professional. Even if you don't secure a job, you’ll build a valuable connection. If there's no response, wait a day or two before following up, and avoid sending multiple messages.

It sounds like a lot of effort, but if you shift the same energy spent applying to jobs to the above activities, you will see better results - more interviews, and more job offers.

----------
for current job check:
jam in:
* Java (education notes, render them)
* Kubernetes under Tmq stuff 
correct email addr to the gmail one.
cross functional somewhere in kraken stuff

baseline checks:
double-check that Redis is mentioned in baseline chrono and functional!

https://pdfinspector-1025621488749.us-central1.run.app

GOTENBERG_URL=https://gotenberg-1025621488749.us-central1.run.app
JSON_SERVER_URL=https://json-server-1025621488749.us-central1.run.app
REACT_APP_URL=https://react-app-1025621488749.us-central1.run.app

docker run -e GOTENBERG_URL=https://gotenberg-1025621488749.us-central1.run.app \
-e JSON_SERVER_URL=https://json-server-1025621488749.us-central1.run.app \
-e REACT_APP_URL=https://react-app-1025621488749.us-central1.run.app \
my-go-app

gcloud run deploy my-go-app \
--image gcr.io/your-project-id/my-go-app-image \
--platform managed \
--region us-central1 \
--allow-unauthenticated \
--set-env-vars GOTENBERG_URL=https://gotenberg-1025621488749.us-central1.run.app,JSON_SERVER_URL=https://json-server-1025621488749.us-central1.run.app,REACT_APP_URL=https://react-app-1025621488749.us-central1.run.app

curl --location 'http://localhost:8080/streamjob' --header 'Content-Type: application/json' --data '{
"jd":"crane operator",
"baseline":"functional",
"prompt":"replace the resume data contents with a detailing of fake, but real-sounding functional contributions that relate to the Job Description, while working at fake related companies during the time period 1945 to 1977. education fake and related as well."
}' --no-buffer -sS

refactor todo/notes:
no more log.Fatalf in anything the webserver calls!