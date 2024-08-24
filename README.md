```
if want to run the resume in docker do like:
    docker run --rm -p 3001:3000 -d --name resume --network my_network ghcr.io/blademccool/resume:master

new logix todo:
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
    cause a ghostscript png render of the pdf
        (that env stuff maybe not needed if we use go program to execute it - but we'll have to fill the pwd too)
        also srsly ask yourself why you are using one from vulhub..... DO BETTER (TODO!!!!vulhub!1!!)
        MSYS_NO_PATHCONV=1 docker run --rm -v /$(pwd):/workspace vulhub/ghostscript:9.56.1 gs -sDEVICE=pngalpha -o /workspace/out2-%03d.png -r144 /workspace/test2.pdf
            update: using minidocks/ghostscript:latest in the code now. phew. that was close.
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



----
this whole deal might work better by breaking it down and adjusting the resume in chunks based on the jd, eg series of promptos to adjust the sections for verbiage, mention its ok to take hidden experiences and use info from them to craft adjustments.
```

i noted also that the profile field in the respones was getting filled with something interesting but we dont use it.

lets get the attemptx.json files also saved in the local output dir so that all the stuff for a given output is preserved in one place
clean up the logging so far
try to generate 1 complete cynical bullshit resume for another actual job, dunno if i should apply.
try to generate 1 real kind of resume for one actual job, apply for it, note it, be careful and correct.
