package tuner

import (
	"bytes"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"pdfinspector/config"
	"pdfinspector/job"
	"sort"
	"strings"
)

// Struct to hold the inspection results
type inspectResult struct {
	NumberOfPages        int
	LastPageContentRatio float64
}

func makePDFRequestAndSave(attempt int, config *config.ServiceConfig, job *job.Job) error {
	// Step 1: Create a new buffer and a multipart writer
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Step 2: Add the "url" field to the multipart form
	urlField, err := writer.CreateFormField("url")
	if err != nil {
		return fmt.Errorf("failed to create form field: %v", err)
	}

	var urlToRender string
	if config.FsType == "gcs" {
		//for server mode with gcs data we need to make sure we pass jsonserver with a hostname value (no https:// prefix), and make sure that the uuid and a slash (uri escaped) get prepended to the attemptN value.
		jsonServerHostname, err := extractHostname(config.JsonServerURL)
		if err != nil {
			return err
		}
		jsonPathFragment := url.PathEscape(fmt.Sprintf("%s/attempt%d", job.Id.String(), attempt))
		fmt.Sprintf("%s/attempt%d", job.Id.String(), attempt)
		urlToRender = fmt.Sprintf("%s/?jsonserver=%s&resumedata=%s&layout=%s", config.ReactAppURL, jsonServerHostname, jsonPathFragment, job.Layout)
	} else {
		//legacy way, presumably json server is on local host or smth.
		urlToRender = fmt.Sprintf("%s/?resumedata=attempt%d&layout=%s", config.ReactAppURL, attempt, job.Layout)
	}
	_, err = io.WriteString(urlField, urlToRender)
	if err != nil {
		return fmt.Errorf("failed to write to form field: %v", err)
	}

	// add a waitForExpression since i got a pdf that just had "Loading..." in it ... bad.
	waitForExpressionField, err := writer.CreateFormField("waitForExpression")
	if err != nil {
		return fmt.Errorf("failed to create form field: %v", err)
	}
	_, err = io.WriteString(waitForExpressionField, "window.contentLoaded === true")
	if err != nil {
		return fmt.Errorf("failed to write to form field: %v", err)
	}

	//// test waitDelay (it seemed to work but the js thing above is way more explicit)
	//waitDelayField, err := writer.CreateFormField("waitDelay")
	//if err != nil {
	//	return fmt.Errorf("failed to create form field: %v", err)
	//}
	//_, err = io.WriteString(waitDelayField, "3s")
	//if err != nil {
	//	return fmt.Errorf("failed to write to form field: %v", err)
	//}

	// Close the multipart writer to finalize the form data
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %v", err)
	}

	// Step 4: Create a new POST request with the multipart form data
	gotenbergRequestURL := fmt.Sprintf("%s/forms/chromium/convert/url", config.GotenbergURL)
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
	outputFilePath := filepath.Join(job.OutputDir, fmt.Sprintf("attempt%d.pdf", attempt))

	// Create the output directory if it doesn't exist
	err = os.MkdirAll(job.OutputDir, 0755)
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

func dumpPDFToPNG(attempt int, outputDir string, config *config.ServiceConfig) error {
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
	if config.UseSystemGs {
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
	log.Println("Here just after run")
	if err != nil {
		return fmt.Errorf("Error running docker command: %v\n", err)
	}
	log.Println("Here before readfile")
	data, err := os.ReadFile(filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"))
	if err != nil {
		return fmt.Errorf("error reading pdf txt output %v", err)
	}
	log.Println("Here before checking for strings")
	if strings.Contains(string(data), "Uncaught runtime errors") {
		return fmt.Errorf("'Uncaught runtime errors' string detected in PDF contents.")
	}
	log.Println("Here before proceeding to image dumping")

	if config.UseSystemGs {
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
