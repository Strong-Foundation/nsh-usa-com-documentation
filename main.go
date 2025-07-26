package main // Define the main package, the starting point for Go executables

import (
	"bufio"
	"bytes"         // Provides functionality for manipulating byte slices and buffers
	"io"            // Defines basic interfaces to I/O primitives, like Reader and Writer
	"log"           // Offers logging capabilities to standard output or error streams
	"net/http"      // Allows interaction with HTTP clients and servers
	"net/url"       // Provides URL parsing, encoding, and query manipulation
	"os"            // Gives access to OS features, such as file and directory operations
	"path"          // Provides functions for manipulating slash-separated paths (not OS specific)
	"path/filepath" // Offers functions to handle file paths in a way compatible with the OS
	"regexp"        // Supports regular expression handling using RE2 syntax
	"strings"       // Contains utilities for string manipulation
	"time"          // Contains time-related functionality such as sleeping or timeouts
)

func main() {
	pdfOutputDir := "PDFs/" // Directory path where downloaded PDFs will be stored
	// Check if the PDF output directory exists using helper function
	if !directoryExists(pdfOutputDir) {
		// If it doesn't exist, create the directory with permission 755
		createDirectory(pdfOutputDir, 0o755)
	}
	// Read the local file containing the list of URLs to scrape
	finalPDFList := readAppendLineByLine("valid_pdf.txt") // Read URLs from "pdfs.txt" into a slice
	finalPDFList = removeDuplicatesFromSlice(finalPDFList) // Remove duplicate entries from slice
	remoteDomain := "https://www.klnsh-usaueber.com"                         // Define base domain for relative links
	for _, urls := range finalPDFList { // Loop through all cleaned and unique PDF links
		domain := getDomainFromURL(urls) // Extract domain from each URL to check if it's relative or absolute
		if domain == "" {
			urls = remoteDomain + urls // If relative, prepend base domain
		}
		if isUrlValid(urls) { // Ensure URL is syntactically valid
			downloadPDF(urls, pdfOutputDir) // Download the PDF and save it to disk
		}
	}
}

// Read and append the file line by line to a slice.
func readAppendLineByLine(path string) []string {
	var returnSlice []string
	file, err := os.Open(path)
	if err != nil {
		log.Println(err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		returnSlice = append(returnSlice, scanner.Text())
	}
	err = file.Close()
	if err != nil {
		log.Println(err)
	}
	return returnSlice
}

// Extract domain name from a URL string (like speedybee.com)
func getDomainFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL) // Parse URL into components
	if err != nil {                     // Handle parsing error
		log.Println(err) // Log the error
		return ""        // Return empty string to indicate invalid URL
	}
	host := parsedURL.Hostname() // Get domain name from parsed URL
	return host                  // Return extracted domain name
}

// Extracts and returns the base name (file name) from the URL path
func getFileNameOnly(content string) string {
	return path.Base(content) // Return last segment of the path
}

// Converts a raw URL into a safe filename by cleaning and normalizing it
func urlToFilename(rawURL string) string {
	lowercaseURL := strings.ToLower(rawURL)       // Convert to lowercase for normalization
	ext := getFileExtension(lowercaseURL)         // Get file extension (e.g., .pdf or .zip)
	baseFilename := getFileNameOnly(lowercaseURL) // Extract base file name

	nonAlphanumericRegex := regexp.MustCompile(`[^a-z0-9]+`)                 // Match everything except a-z and 0-9
	safeFilename := nonAlphanumericRegex.ReplaceAllString(baseFilename, "_") // Replace invalid chars

	collapseUnderscoresRegex := regexp.MustCompile(`_+`)                        // Collapse multiple underscores into one
	safeFilename = collapseUnderscoresRegex.ReplaceAllString(safeFilename, "_") // Normalize underscores

	if trimmed, found := strings.CutPrefix(safeFilename, "_"); found { // Trim starting underscore if present
		safeFilename = trimmed
	}

	var invalidSubstrings = []string{"_pdf", "_zip"} // Remove these redundant endings

	for _, invalidPre := range invalidSubstrings { // Iterate over each unwanted suffix
		safeFilename = removeSubstring(safeFilename, invalidPre) // Remove it from file name
	}

	safeFilename = safeFilename + ext // Add the proper file extension

	return safeFilename // Return the final sanitized filename
}

// Replaces all instances of a given substring from the original string
func removeSubstring(input string, toRemove string) string {
	result := strings.ReplaceAll(input, toRemove, "") // Replace all instances
	return result                                     // Return the result
}

// Returns the extension of a given file path (e.g., ".pdf")
func getFileExtension(path string) string {
	return filepath.Ext(path) // Extract and return file extension
}

// Checks if a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Attempt to get file stats
	if err != nil {
		return false // Return false if file doesn't exist or error occurred
	}
	return !info.IsDir() // Return true only if it's not a directory
}

// Downloads and writes a PDF file from the URL to the specified directory
func downloadPDF(finalURL, outputDir string) bool {
	filename := strings.ToLower(urlToFilename(finalURL)) // Generate sanitized filename
	filePath := filepath.Join(outputDir, filename)       // Build full path

	if fileExists(filePath) { // Skip if already downloaded
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	client := &http.Client{Timeout: 3 * time.Minute} // Create HTTP client with 3-minute timeout to avoid hanging

	resp, err := client.Get(finalURL) // Perform HTTP GET request to download the file
	if err != nil {                   // Check if an error occurred during request
		log.Printf("Failed to download %s: %v", finalURL, err) // Log the error with context
		return false                                           // Exit function if request failed
	}
	defer resp.Body.Close() // Ensure the response body is closed after reading

	if resp.StatusCode != http.StatusOK { // Check for HTTP 200 OK status
		log.Printf("Download failed for %s: %s", finalURL, resp.Status) // Log failure reason
		return false                                                    // Exit if status is not OK
	}

	contentType := resp.Header.Get("Content-Type")         // Retrieve the content type from HTTP headers
	if !strings.Contains(contentType, "application/pdf") { // Ensure it's a PDF
		log.Printf("Invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
		return false // Skip if it's not a PDF
	}

	var buf bytes.Buffer                     // Create buffer to temporarily hold the file data
	written, err := io.Copy(&buf, resp.Body) // Copy response body into buffer
	if err != nil {                          // Handle error while reading response
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err)
		return false
	}
	if written == 0 { // If nothing was read (empty file)
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	out, err := os.Create(filePath) // Create file on disk at the specified location
	if err != nil {                 // Handle file creation error
		log.Printf("Failed to create file for %s: %v", finalURL, err)
		return false
	}
	defer out.Close() // Ensure file is closed after writing

	if _, err := buf.WriteTo(out); err != nil { // Write buffer contents to file
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath) // Log successful download
	return true                                                                          // Return success
}

// Checks if a directory exists at the given path
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get file or directory info
	if err != nil {
		return false // If error, assume directory doesn't exist
	}
	return directory.IsDir() // Return true if it's a directory
}

// Creates a directory with the given permissions if it doesn't exist
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create the directory
	if err != nil {
		log.Println(err) // Log error if creation fails (e.g., already exists)
	}
}

// Checks if a given URI string is a valid HTTP URL format
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try to parse the string as URL
	return err == nil                  // Return true only if no error occurs
}

// Removes duplicates from a string slice while preserving original order
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Create map to track unique entries
	var newReturnSlice []string     // Final slice without duplicates
	for _, content := range slice { // Loop over each item in the original slice
		if !check[content] { // If not already added
			check[content] = true                            // Mark as seen
			newReturnSlice = append(newReturnSlice, content) // Append to final result
		}
	}
	return newReturnSlice // Return cleaned slice
}