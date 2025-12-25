// Package everything3 provides a Go binding for Everything SDK 3.
// Everything SDK 3 is a C library for communicating with Everything 1.5
// via Windows Named Pipes IPC.
package everything3

import (
	"time"
)

// Error codes returned by Everything SDK 3
const (
	ErrOK               = 0
	ErrOutOfMemory      = 0xE0000001
	ErrIPCPipeNotFound  = 0xE0000002
	ErrDisconnected     = 0xE0000003
	ErrInvalidParameter = 0xE0000004
	ErrBadRequest       = 0xE0000005
	ErrPropertyNotFound = 0xE0000007
)

// Property IDs for search results
const (
	PropertyIDName          = 0  // File name
	PropertyIDPath          = 1  // Folder path
	PropertyIDSize          = 2  // File size (uint64)
	PropertyIDExtension     = 3  // File extension
	PropertyIDType          = 4  // File type description
	PropertyIDDateModified  = 5  // Modified time (FILETIME)
	PropertyIDDateCreated   = 6  // Created time (FILETIME)
	PropertyIDDateAccessed  = 7  // Accessed time (FILETIME)
	PropertyIDAttributes    = 8  // File attributes (uint32)
	PropertyIDRunCount      = 10 // Run count (uint32)
	PropertyIDDateRun       = 11 // Last run time (FILETIME)
	PropertyIDFullPathName  = 12 // Full path and filename
	PropertyIDHighlightName = 13 // Highlighted name
	PropertyIDHighlightPath = 14 // Highlighted path
)

// Image property IDs
const (
	PropertyIDImageWidth      = 100 // Image width
	PropertyIDImageHeight     = 101 // Image height
	PropertyIDImageBitDepth   = 102 // Image bit depth
	PropertyIDImageDimensions = 103 // Image dimensions
)

// Audio property IDs
const (
	PropertyIDAudioDuration   = 200 // Audio duration
	PropertyIDAudioBitRate    = 201 // Audio bit rate
	PropertyIDAudioSampleRate = 202 // Audio sample rate
	PropertyIDAudioChannels   = 203 // Audio channels
	PropertyIDAudioArtist     = 210 // Artist
	PropertyIDAudioAlbum      = 211 // Album
	PropertyIDAudioTitle      = 212 // Title
	PropertyIDAudioGenre      = 213 // Genre
	PropertyIDAudioYear       = 214 // Year
)

// Video property IDs
const (
	PropertyIDVideoDuration = 300 // Video duration
	PropertyIDVideoWidth    = 301 // Video width
	PropertyIDVideoHeight   = 302 // Video height
	PropertyIDVideoFrameRate = 303 // Video frame rate
)

// FoldersFirst specifies how folders are sorted relative to files
type FoldersFirst int

const (
	FoldersFirstDefault FoldersFirst = 0 // Use default setting
	FoldersFirstYes     FoldersFirst = 1 // Folders before files
	FoldersFirstNo      FoldersFirst = 2 // Mixed order
	FoldersFirstFilesFirst FoldersFirst = 3 // Files before folders
)

// SortOrder specifies the sort direction
type SortOrder int

const (
	SortAscending  SortOrder = 0
	SortDescending SortOrder = 1
)

// Sort represents a sort configuration
type Sort struct {
	PropertyID uint32
	Ascending  bool
}

// SearchOptions contains options for a search query
type SearchOptions struct {
	// Search text
	Text string

	// Match options
	MatchCase        bool // Case-sensitive search
	MatchWholeWords  bool // Match whole words only
	MatchPath        bool // Match against full path
	MatchPrefix      bool // Match prefix only
	MatchSuffix      bool // Match suffix only
	MatchDiacritics  bool // Match diacritical marks
	IgnorePunctuation bool // Ignore punctuation
	UseRegex         bool // Use regular expressions

	// Pagination
	Offset uint32 // Starting offset in results
	Count  uint32 // Number of results to return (0 = default)

	// Sorting
	Sorts        []Sort       // Sort configuration
	FoldersFirst FoldersFirst // Folder sorting preference

	// Properties to request
	RequestedProperties []uint32 // List of property IDs to retrieve
}

// DefaultSearchOptions returns sensible default search options
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		Count: 100,
		RequestedProperties: []uint32{
			PropertyIDName,
			PropertyIDPath,
			PropertyIDSize,
			PropertyIDDateModified,
			PropertyIDAttributes,
		},
	}
}

// Result represents a single search result
type Result struct {
	// Basic properties (always available)
	Name     string // File or folder name
	Path     string // Parent folder path
	FullPath string // Complete path including filename

	// File attributes
	Size         uint64    // File size in bytes
	DateModified time.Time // Last modified time
	DateCreated  time.Time // Creation time
	DateAccessed time.Time // Last accessed time
	Attributes   uint32    // Windows file attributes
	Extension    string    // File extension
	Type         string    // File type description

	// Flags
	IsFolder bool // True if this is a directory

	// Extended properties (populated based on request)
	Properties map[uint32]interface{}
}

// ResultList contains search results
type ResultList struct {
	// Counts
	TotalCount    uint64 // Total number of matching results
	FolderCount   uint64 // Number of matching folders
	FileCount     uint64 // Number of matching files
	ViewportCount uint64 // Number of results in this response

	// Results in the current viewport
	Results []Result

	// Search state (for updates)
	searchID uint64
}

// FileInfo contains file attribute information (similar to WIN32_FIND_DATA)
type FileInfo struct {
	Name           string
	Attributes     uint32
	Size           uint64
	CreationTime   time.Time
	LastAccessTime time.Time
	LastWriteTime  time.Time
}

// IsDirectory returns true if the file is a directory
func (fi *FileInfo) IsDirectory() bool {
	return fi.Attributes&0x10 != 0 // FILE_ATTRIBUTE_DIRECTORY
}

// IsReadOnly returns true if the file is read-only
func (fi *FileInfo) IsReadOnly() bool {
	return fi.Attributes&0x01 != 0 // FILE_ATTRIBUTE_READONLY
}

// IsHidden returns true if the file is hidden
func (fi *FileInfo) IsHidden() bool {
	return fi.Attributes&0x02 != 0 // FILE_ATTRIBUTE_HIDDEN
}

// IsSystem returns true if the file is a system file
func (fi *FileInfo) IsSystem() bool {
	return fi.Attributes&0x04 != 0 // FILE_ATTRIBUTE_SYSTEM
}

// JournalInfo contains information about an Everything index journal
type JournalInfo struct {
	VolumeSerial uint32
	NextUSN      uint64
	MaxUSN       uint64
}

// JournalEntry represents a change entry from the journal
type JournalEntry struct {
	Action    JournalAction
	IsFolder  bool
	Path      string
	Name      string
	Timestamp time.Time
}

// JournalAction represents the type of change in a journal entry
type JournalAction int

const (
	JournalActionAdd    JournalAction = 1
	JournalActionRemove JournalAction = 2
	JournalActionModify JournalAction = 3
	JournalActionRename JournalAction = 4
)
