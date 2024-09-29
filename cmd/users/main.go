package main

import (
	"bufio"
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"flag"
	"fmt"
	"google.golang.org/api/iterator"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	var mode string
	var usersdir string
	var bucketname string
	var count int
	var makekeys int
	var doDelete bool

	flag.StringVar(&mode, "mode", "users", "mode: users, clean, ...")

	flag.StringVar(&usersdir, "usersdir", "users", "path to users dir")
	flag.StringVar(&bucketname, "bucketname", "my-stinky-bucket", "gcs bucket where stuff lives")
	flag.IntVar(&count, "count", 100, "count of credits to issue new users")
	flag.IntVar(&makekeys, "makekeys", 0, "make some keys, print them and exit")

	flag.BoolVar(&doDelete, "delete", false, "perform delete of old objects or just report what should go")

	// Parse the flags
	flag.Parse()

	if mode == "users" {
		makeUsers(usersdir, bucketname, count, makekeys)
	} else if mode == "clean" {
		cleanOutputs("outputs", bucketname, 1*24*time.Hour, doDelete)
	}
}

var preserveUuids = map[string]bool{
	"5b3f3fe4-7a12-4437-aa8d-8910f7730d3f": true,
	"351d2e84-455b-4603-95d6-77371e5730d4": true,
	"fa268485-bc2e-4085-9520-8e960edc3169": true,
	"b8c626d9-91d4-414a-9795-ae59bf0125c6": true,
	"130b51c7-8a3a-4250-b8fd-0257a91a5492": true,
}

func cleanOutputs(outputsDir, bucketName string, ttl time.Duration, doDelete bool) {
	panic("hey careful we might have customer data now lol. only remove if you're sure!")
	log.Printf("setting up gcs client ...")
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}

	bucket := client.Bucket(bucketName)
	query := &storage.Query{
		Prefix: outputsDir + "/",
	}

	it := bucket.Objects(ctx, query)

	//we have to collect the prefixes first because they don't have any actual date information attached
	// (guessing b/c not the actual objects?) and so we have to inspect something inside the "folder" to get a date.
	checkedOutputs := map[string]time.Time{}
	for {
		objAttrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		// Extract the UUID from the object name
		// Assuming object names are in the format "outputs/{uuid}/..."
		name := objAttrs.Name
		// Remove the outputsDir prefix
		relativePath := strings.TrimPrefix(name, outputsDir+"/")
		// Split the relative path to get the UUID
		parts := strings.SplitN(relativePath, "/", 2)
		if len(parts) < 1 || parts[0] == "" {
			// Skip if we can't get a UUID
			log.Printf("dont know what to do with %s", name)
			continue
		}
		uuid := parts[0]

		// Check if we've already processed this UUID
		if _, exists := checkedOutputs[uuid]; exists {
			continue
		}
		// Mark UUID as processed
		checkedOutputs[uuid] = objAttrs.Created
	}

	deleteIfOlderThan := time.Now().Add(-ttl)
	deletedItemsTotal := 0
	deletedJobs := 0
	for uuid, created := range checkedOutputs {
		if preserveUuids[uuid] {
			log.Printf("preserving output %s indefinitely", uuid)
			continue
		}
		if !created.Before(deleteIfOlderThan) {
			continue
		}
		if !doDelete {
			log.Printf("UUID '%s' is older than allowed; (should delete)", uuid)
			continue
		}
		log.Printf("Deleting UUID '%s' ... (Dated: %s)", uuid, created.Format("2006-01-02T15:04:05Z"))

		// Delete all objects under this UUID directory
		delPrefix := fmt.Sprintf("%s/%s/", outputsDir, uuid)
		delQuery := &storage.Query{
			Prefix: delPrefix,
		}
		delIt := bucket.Objects(ctx, delQuery)
		for {
			delAttrs, err := delIt.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				log.Printf("error iterating over objects for deletion in prefix %s: %v", delPrefix, err)
				return
			}
			// Delete the object
			objHandle := bucket.Object(delAttrs.Name)
			if err := objHandle.Delete(ctx); err != nil {
				log.Printf("error deleting object %s: %v", delAttrs.Name, err)
			}
			deletedItemsTotal++
		}
		deletedJobs++
	}
	log.Printf("Deleted %d jobs (%d total objects)", deletedJobs, deletedItemsTotal)
	//todo maybe report back or smth instead of just the log output.

}

func makeUsers(usersdir, bucketname string, count, makekeys int) {
	// Print a greeting
	if makekeys != 0 {
		for i := 0; i < makekeys; i++ {
			fmt.Printf("%s\n", randomString(64))
		}
		os.Exit(0)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("cwd was %s", cwd)

	pattern := "list*.txt"
	// Use filepath.Glob to find all matching files
	files, err := filepath.Glob(filepath.Join(usersdir, pattern))
	if err != nil {
		log.Printf("Error finding files in %s: %v", usersdir, err)
		return
	}

	// If no files match the pattern, log it and return
	if len(files) == 0 {
		log.Printf("No files matching pattern %s found in directory %s", pattern, usersdir)
		return
	}

	userKeys := make(map[string]bool)

	// Iterate over each file
	for _, file := range files {
		// Open the file for reading
		f, err := os.Open(file)
		if err != nil {
			log.Printf("Error opening file %s: %v", file, err)
			continue
		}
		defer f.Close()

		// Use bufio.Scanner to read the file line by line
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			// Each line is an API key, add it to UserKeys
			apiKey := strings.TrimSpace(scanner.Text())
			if apiKey != "" {
				userKeys[apiKey] = true
			}
		}

		// Log any scanning errors (such as malformed input)
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading file %s: %v", file, err)
		}
	}
	log.Printf("identified %d user keys", len(userKeys))

	log.Printf("setting up gcs client ...")
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create GCS client: %v", err)
	}
	_ = client
	issueCreditsToNewUsers(client, userKeys, bucketname, count)
}

func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[rand.Intn(62)]
	}
	return string(b)
}

func issueCreditsToNewUsers(client *storage.Client, users map[string]bool, bucketName string, credits int) error {
	ctx := context.Background()
	bucket := client.Bucket(bucketName)

	// Prefix for the users directory
	userDirPrefix := "users/"

	// List all objects under the "users/" directory in the bucket
	it := bucket.Objects(ctx, &storage.Query{Prefix: userDirPrefix, Delimiter: "/"})
	existingUsers := make(map[string]bool)

	// Iterate over the objects and directories
	for {
		attr, err := it.Next()
		if err != nil {
			log.Printf("done iterating over bucket items.")
			break
		}

		// Get the user ID from the object name (userDirPrefix + userID + "/")
		if attr.Prefix != "" {
			userID := strings.TrimPrefix(attr.Prefix, userDirPrefix)
			userID = strings.TrimSuffix(userID, "/")
			log.Printf("found something about a user %s in gcs", userID)
			existingUsers[userID] = true
		}
	}
	log.Printf("Listed out %d known existing users in GCS users dir", len(existingUsers))
	// Loop over the provided user list and issue credits if they don't exist in the bucket
	for userID := range users {
		if !existingUsers[userID] {
			// User doesn't exist, create directory and issue credit
			userDir := userDirPrefix + userID + "/"

			// Create the "credit" file with the number of credits
			creditFile := bucket.Object(userDir + "credit")
			w := creditFile.NewWriter(ctx)
			// Convert the credits to string without a newline
			_, err := w.Write([]byte(strconv.Itoa(credits)))
			if err != nil {
				return fmt.Errorf("failed to write credit file for user %s: %v", userID, err)
			}
			if err := w.Close(); err != nil {
				return fmt.Errorf("failed to close credit file for user %s: %v", userID, err)
			}

			log.Printf("Issued %d credits to user %s", credits, userID)
		}
	}

	return nil
}
