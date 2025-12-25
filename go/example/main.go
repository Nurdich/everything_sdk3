// Everything SDK 3 - Go Example
// This example demonstrates how to use the Everything SDK 3 Go binding
// to search for files and retrieve file information.
//
// Requirements:
//   - Windows operating system
//   - Everything 1.5 must be running with IPC enabled
//
// Usage:
//
//	go run main.go [search query]
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/voidtools/everything-sdk3-go/everything3"
)

func main() {
	fmt.Println("Everything SDK 3 - Go Example")
	fmt.Println("==============================")
	fmt.Println()

	// Get search query from command line or use default
	searchQuery := "*.go"
	if len(os.Args) > 1 {
		searchQuery = strings.Join(os.Args[1:], " ")
	}

	// Run examples
	if err := simpleExample(searchQuery); err != nil {
		fmt.Printf("Simple example error: %v\n", err)
	}

	fmt.Println()

	if err := paginationExample(searchQuery); err != nil {
		fmt.Printf("Pagination example error: %v\n", err)
	}

	fmt.Println()

	if err := sortingExample(searchQuery); err != nil {
		fmt.Printf("Sorting example error: %v\n", err)
	}

	fmt.Println()

	if err := fileInfoExample(); err != nil {
		fmt.Printf("File info example error: %v\n", err)
	}

	fmt.Println()

	if err := findFilesExample(); err != nil {
		fmt.Printf("Find files example error: %v\n", err)
	}

	fmt.Println()

	if err := hashExample(); err != nil {
		fmt.Printf("Hash example error: %v\n", err)
	}

	fmt.Println()

	if err := searchWithHashesExample(); err != nil {
		fmt.Printf("Search with hashes example error: %v\n", err)
	}
}

// simpleExample demonstrates a basic search
func simpleExample(query string) error {
	fmt.Println("=== Simple Search Example ===")
	fmt.Printf("Searching for: %s\n\n", query)

	// Connect to Everything
	// Try default instance first, then "1.5a"
	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Create search options
	opts := everything3.DefaultSearchOptions()
	opts.Text = query
	opts.Count = 10 // Limit to 10 results

	// Perform search
	results, err := client.Search(opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Print results
	fmt.Printf("Found %d total results (showing %d)\n\n",
		results.TotalCount, len(results.Results))

	for i, result := range results.Results {
		typeStr := "File"
		if result.IsFolder {
			typeStr = "Folder"
		}

		fmt.Printf("%d. [%s] %s\n", i+1, typeStr, result.FullPath)
		if !result.IsFolder {
			fmt.Printf("   Size: %s, Modified: %s\n",
				everything3.FormatSize(result.Size),
				result.DateModified.Format("2006-01-02 15:04:05"))
		}
	}

	return nil
}

// paginationExample demonstrates paginated search results
func paginationExample(query string) error {
	fmt.Println("=== Pagination Example ===")
	fmt.Printf("Searching for: %s (page 2, 5 results per page)\n\n", query)

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Create search options with pagination
	opts := everything3.DefaultSearchOptions()
	opts.Text = query
	opts.Offset = 5  // Skip first 5 results (page 2)
	opts.Count = 5   // Show 5 results per page

	results, err := client.Search(opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("Total results: %d | Showing results 6-10\n\n", results.TotalCount)

	for i, result := range results.Results {
		fmt.Printf("%d. %s\n", i+6, result.FullPath)
	}

	return nil
}

// sortingExample demonstrates sorted search results
func sortingExample(query string) error {
	fmt.Println("=== Sorting Example ===")
	fmt.Printf("Searching for: %s (sorted by size, descending)\n\n", query)

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Create search options with sorting
	opts := everything3.DefaultSearchOptions()
	opts.Text = query
	opts.Count = 10
	opts.Sorts = []everything3.Sort{
		{
			PropertyID: everything3.PropertyIDSize,
			Ascending:  false, // Largest files first
		},
	}
	opts.FoldersFirst = everything3.FoldersFirstNo // Mix folders and files

	results, err := client.Search(opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("Top %d largest files:\n\n", len(results.Results))

	for i, result := range results.Results {
		if !result.IsFolder {
			fmt.Printf("%d. %s (%s)\n", i+1, result.Name, everything3.FormatSize(result.Size))
		} else {
			fmt.Printf("%d. [DIR] %s\n", i+1, result.Name)
		}
	}

	return nil
}

// fileInfoExample demonstrates getting file attributes
func fileInfoExample() error {
	fmt.Println("=== File Info Example ===")

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Get info for a common Windows file
	testPath := `C:\Windows\notepad.exe`
	fmt.Printf("Getting info for: %s\n\n", testPath)

	// Get basic attributes
	attrs, err := client.GetFileAttributes(testPath)
	if err != nil {
		fmt.Printf("Note: Could not get attributes (file may not exist): %v\n", err)
		return nil
	}

	fmt.Printf("Attributes: 0x%08X\n", attrs)
	fmt.Printf("  - Is Directory: %v\n", attrs&0x10 != 0)
	fmt.Printf("  - Is Read-Only: %v\n", attrs&0x01 != 0)
	fmt.Printf("  - Is Hidden: %v\n", attrs&0x02 != 0)
	fmt.Printf("  - Is System: %v\n", attrs&0x04 != 0)

	// Get detailed file info
	info, err := client.GetFileInfo(testPath)
	if err != nil {
		fmt.Printf("Note: Could not get detailed info: %v\n", err)
		return nil
	}

	fmt.Printf("\nDetailed Info:\n")
	fmt.Printf("  Name: %s\n", info.Name)
	fmt.Printf("  Size: %s\n", everything3.FormatSize(info.Size))
	fmt.Printf("  Created: %s\n", info.CreationTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Modified: %s\n", info.LastWriteTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Accessed: %s\n", info.LastAccessTime.Format("2006-01-02 15:04:05"))

	return nil
}

// findFilesExample demonstrates the FindFirstFile/FindNextFile pattern
func findFilesExample() error {
	fmt.Println("=== FindFiles Example ===")
	fmt.Println("Listing files in C:\\Windows\\*.exe (first 5)\n")

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Start finding files
	handle, firstFile, err := client.FindFirstFile(`C:\Windows\*.exe`)
	if err != nil {
		fmt.Printf("Note: Could not find files: %v\n", err)
		return nil
	}
	defer handle.Close()

	count := 0

	// Print first file
	fmt.Printf("%d. %s (%s)\n", count+1, firstFile.Name, everything3.FormatSize(firstFile.Size))
	count++

	// Iterate through remaining files
	for count < 5 {
		info, err := handle.Next()
		if err != nil {
			break // No more files
		}
		fmt.Printf("%d. %s (%s)\n", count+1, info.Name, everything3.FormatSize(info.Size))
		count++
	}

	fmt.Printf("\nListed %d files\n", count)

	return nil
}

// hashExample demonstrates getting file hash values (CRC32, MD5, SHA256, etc.)
func hashExample() error {
	fmt.Println("=== File Hash Example ===")

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Get hashes for a common Windows file
	testPath := `C:\Windows\notepad.exe`
	fmt.Printf("Getting hashes for: %s\n\n", testPath)

	// Get individual hashes
	crc32, err := client.GetFileCRC32(testPath)
	if err != nil {
		fmt.Printf("CRC32: (not available: %v)\n", err)
	} else {
		fmt.Printf("CRC32:  %08X\n", crc32)
	}

	md5, err := client.GetFileMD5(testPath)
	if err != nil {
		fmt.Printf("MD5:    (not available: %v)\n", err)
	} else {
		fmt.Printf("MD5:    %s\n", md5)
	}

	sha1, err := client.GetFileSHA1(testPath)
	if err != nil {
		fmt.Printf("SHA1:   (not available: %v)\n", err)
	} else {
		fmt.Printf("SHA1:   %s\n", sha1)
	}

	sha256, err := client.GetFileSHA256(testPath)
	if err != nil {
		fmt.Printf("SHA256: (not available: %v)\n", err)
	} else {
		fmt.Printf("SHA256: %s\n", sha256)
	}

	// Get all hashes at once
	fmt.Println("\n--- All Hashes ---")
	hashes, err := client.GetFileHashes(testPath)
	if err != nil {
		fmt.Printf("Could not get hashes: %v\n", err)
	} else {
		if hashes.CRC32.Valid {
			fmt.Printf("CRC32:  %08X\n", hashes.CRC32.CRC32())
		}
		if hashes.CRC64.Valid {
			fmt.Printf("CRC64:  %016X\n", hashes.CRC64.CRC64())
		}
		if hashes.MD5.Valid {
			fmt.Printf("MD5:    %s\n", hashes.MD5.String())
		}
		if hashes.SHA1.Valid {
			fmt.Printf("SHA1:   %s\n", hashes.SHA1.String())
		}
		if hashes.SHA256.Valid {
			fmt.Printf("SHA256: %s\n", hashes.SHA256.String())
		}
		if hashes.SHA384.Valid {
			fmt.Printf("SHA384: %s\n", hashes.SHA384.String())
		}
		if hashes.SHA512.Valid {
			fmt.Printf("SHA512: %s\n", hashes.SHA512.String())
		}
	}

	return nil
}

// searchWithHashesExample demonstrates searching with hash properties
func searchWithHashesExample() error {
	fmt.Println("=== Search with Hashes Example ===")
	fmt.Println("Searching for *.exe files with CRC32 and SHA256\n")

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Use search options that include hash properties
	opts := everything3.SearchOptionsWithHashes()
	opts.Text = "*.exe"
	opts.Count = 5

	results, err := client.Search(opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("Found %d results (showing %d with hashes)\n\n",
		results.TotalCount, len(results.Results))

	for i, result := range results.Results {
		fmt.Printf("%d. %s\n", i+1, result.Name)
		fmt.Printf("   Path: %s\n", result.Path)
		fmt.Printf("   Size: %s\n", everything3.FormatSize(result.Size))

		// Show hash values if available
		if result.CRC32.Valid {
			fmt.Printf("   CRC32: %08X\n", result.CRC32.CRC32())
		}
		if result.MD5.Valid {
			fmt.Printf("   MD5: %s\n", result.MD5.String())
		}
		if result.SHA256.Valid {
			fmt.Printf("   SHA256: %s\n", result.SHA256.String())
		}
		fmt.Println()
	}

	return nil
}

// advancedSearchExample demonstrates advanced search options
func advancedSearchExample() error {
	fmt.Println("=== Advanced Search Example ===")

	client, err := everything3.ConnectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to Everything: %w", err)
	}
	defer client.Close()

	// Search with multiple options
	opts := &everything3.SearchOptions{
		Text:            "config",           // Search for "config"
		MatchCase:       false,              // Case-insensitive
		MatchWholeWords: false,              // Partial matches OK
		MatchPath:       true,               // Search in paths too
		UseRegex:        false,              // Don't use regex
		Count:           20,                 // Get 20 results
		FoldersFirst:    everything3.FoldersFirstYes,
		Sorts: []everything3.Sort{
			{
				PropertyID: everything3.PropertyIDDateModified,
				Ascending:  false, // Most recently modified first
			},
		},
		RequestedProperties: []uint32{
			everything3.PropertyIDName,
			everything3.PropertyIDPath,
			everything3.PropertyIDSize,
			everything3.PropertyIDDateModified,
			everything3.PropertyIDDateCreated,
			everything3.PropertyIDExtension,
		},
	}

	results, err := client.Search(opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("Found %d results for 'config' (showing %d most recently modified)\n\n",
		results.TotalCount, len(results.Results))

	for i, result := range results.Results {
		typeStr := "File"
		if result.IsFolder {
			typeStr = "Folder"
		}
		fmt.Printf("[%s] %d. %s\n", typeStr, i+1, result.FullPath)
		fmt.Printf("     Modified: %s\n", result.DateModified.Format("2006-01-02 15:04:05"))
	}

	return nil
}
