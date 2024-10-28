package tuner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"pdfinspector/pkg/config"
	"pdfinspector/pkg/job"
	"sort"
	"strings"
)

// Struct to hold the inspection results
type inspectResult struct {
	NumberOfPages        int
	LastPageContentRatio float64
}

type GotenbergHTTPError struct {
	HttpResponseCode int
	HttpError        bool
	Message          string
}

func (e *GotenbergHTTPError) Error() string {
	return e.Message
}

func makePDFRequestAndSave(attempt int, config *config.ServiceConfig, job *job.Job) error {
	// Step 1: Create a new buffer and a multipart writer
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	ctx := context.Background()
	// Step 2: Add the "url" field to the multipart form
	urlField, err := writer.CreateFormField("url")
	if err != nil {
		return fmt.Errorf("failed to create url form field: %v", err)
	}

	var urlToRender string
	if config.FsType == "gcs" {
		//for server mode with gcs data we need to make sure we pass jsonserver with a hostname value (no https:// prefix), and make sure that the uuid and a slash (uri escaped) get prepended to the attemptN value.
		jsonServerHostname, err := extractHostname(config.JsonServerURL)
		if err != nil {
			return err
		}

		//note: json server will expect gcp auth token for react server, because react server needs to receive it to load the page, and b/c its headless chrome its going to forward _that_ token to js fetch requests (you can't even override it if you wanted to due to how chrome treats bearer tokens)
		jsonPathFragment := url.PathEscape(fmt.Sprintf("%s/attempt%d", job.Id, attempt))
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

	extraHttpHeaders, err := getExtraHttpHeadersForGotenbergRequest(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to obtain tokens for react server: %v", err)
	}
	err = writer.WriteField("extraHttpHeaders", extraHttpHeaders)
	if err != nil {
		return fmt.Errorf("failed to create extraHttpHeaders form field: %v", err)
	}

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

	log.Info().Msgf("Will ask gotenberg at %s to render page at %s", gotenbergRequestURL, urlToRender)
	// Step 6: Send the HTTP request
	client, err := createAuthenticatedClient(ctx, config.GotenbergURL)
	if err != nil {
		return fmt.Errorf("failed to create authenticated client: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Trace().Msgf("failed to send HTTP request: %v", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusServiceUnavailable {
			return &GotenbergHTTPError{
				HttpResponseCode: resp.StatusCode,
				HttpError:        true,
				Message:          "Gotenberg gave retryable http error code",
			}
		}
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

	log.Info().Msgf("PDF saved to %s", outputFilePath)

	return nil
}

func getExtraHttpHeadersForGotenbergRequest(ctx context.Context, config *config.ServiceConfig) (string, error) {
	//due to GCP internals and not wanting these services wide open to the public internet we have to pass some tokens along.
	//we need to prepare them here because we can't make gotenberg figure this out.

	// Step 1: Obtain the ID token for the React service
	reactIDToken, err := getIDToken(ctx, config.ReactAppURL)
	if err != nil {
		return "", fmt.Errorf("Error obtaining ID token for React service: %v", err)
	}

	// Step 2: Prepare the extra HTTP headers as a JSON string
	extraHeaders := map[string]string{
		"Authorization": "Bearer " + reactIDToken, //note, due to how chrome works, this will also be the token that gets forwarded to json server in js fetch request, there is no way to change it.
	}
	extraHeadersJSON, err := json.Marshal(extraHeaders)
	if err != nil {
		return "", fmt.Errorf("Error marshalling extra headers: %v", err)
	}
	return string(extraHeadersJSON), nil
}

func getIDToken(ctx context.Context, audience string) (string, error) {
	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return "", fmt.Errorf("idtoken.NewTokenSource: %v", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("tokenSource.Token: %v", err)
	}

	return token.AccessToken, nil
}

func createAuthenticatedClient(ctx context.Context, audience string) (*http.Client, error) {
	tokenSource, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return nil, fmt.Errorf("idtoken.NewTokenSource: %v", err)
	}
	client := oauth2.NewClient(ctx, tokenSource)
	return client, nil
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
	log.Debug().Msgf("dump pdf to txt with gs command: %s", strings.Join(cmd.Args, " "))
	log.Info().Msg("About to check the pdf text to confirm no errors")
	// Run the command
	err = cmd.Run()
	log.Trace().Msg("Here just after run")
	if err != nil {
		return fmt.Errorf("Error running docker command: %v\n", err)
	}
	log.Trace().Msg("Here before readfile")
	data, err := os.ReadFile(filepath.Join(outputDirFullpath, "pdf-txtwrite.txt"))
	if err != nil {
		return fmt.Errorf("error reading pdf txt output %v", err)
	}
	log.Trace().Msg("Here before checking for strings")
	if strings.Contains(string(data), "Uncaught runtime errors") {
		return fmt.Errorf("'Uncaught runtime errors' string detected in PDF contents.")
	}
	if strings.Contains(string(data), "Error loading data: Failed to fetch") {
		return fmt.Errorf("'Error loading data: Failed to fetch' string detected in PDF contents.")
	}
	if strings.Contains(string(data), "Unsupported resume layout: ") {
		return fmt.Errorf("'Unsupported resume layout: ' string detected in PDF contents.")
	}

	log.Trace().Msg("Here before proceeding to image dumping")

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
	log.Debug().Msgf("dump pdf to png with gs command: %s", strings.Join(cmd.Args, " "))

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
			log.Info().Msgf("Rendered pages %s as PNG files", strings.Join(pageLines, ", "))
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
	log.Debug().Msgf("will be treating the last of these files in this list as the last page to look at: %v", pngFiles)

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
		return result, fmt.Errorf("Failed to open image: %v", err)
	}

	result.LastPageContentRatio = contentRatio(img)

	return result, nil
}

func contentRatio(img image.Image) float64 {
	// Get image dimensions
	bounds := img.Bounds()
	//totalPixels := bounds.Dx() * bounds.Dy()

	lastColoredPixelRow := 0
	log.Debug().Msgf("believe image dimensions are: %d x %d", bounds.Max.X, bounds.Max.Y)

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
	log.Debug().Msgf("lastrow found a pixel on: %v, total rows was %v", lastColoredPixelRow, bounds.Max.Y)
	lastContentAt := float64(lastColoredPixelRow) / float64(bounds.Max.Y)
	log.Debug().Msgf("last content found at %.5f of the document.", lastContentAt)
	return lastContentAt
}

func isColored(r, g, b, a uint32) bool {
	const color_threshold = 0.9 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
	const alpha_threshold = 0.1 * 0xffff //idk, using a threshold was a robots idea, seemed reasonable -- but also probably not neccesary. using 1.0 (eg not using it) gives a similar result.
	//also .... transparent with no color == white when on screen as a pdf so ....
	return float64(a) > alpha_threshold && float64(r) <= color_threshold && float64(g) <= color_threshold && float64(b) <= color_threshold
}
