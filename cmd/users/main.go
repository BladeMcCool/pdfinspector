package main

import (
	"bufio"
	"cloud.google.com/go/storage"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	// Define your CLI flags
	var usersdir string
	var bucketname string
	var count int
	var makekeys int
	flag.StringVar(&usersdir, "usersdir", "users", "path to users dir")
	flag.StringVar(&bucketname, "bucketname", "my-stinky-bucket", "gcs bucket where stuff lives")
	flag.IntVar(&count, "count", 100, "count of credits to issue new users")
	flag.IntVar(&makekeys, "makekeys", 0, "make some keys, print them and exit")

	// Parse the flags
	flag.Parse()

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

			//// Create an empty object to represent the directory (GCS is flat, so the "/" is just part of the object name)
			//obj := bucket.Object(userDir)
			//w := obj.NewWriter(ctx)
			//if err := w.Close(); err != nil {
			//	return fmt.Errorf("failed to create directory for user %s: %v", userID, err)
			//}

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
