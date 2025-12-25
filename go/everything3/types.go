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
	PropertyIDVideoDuration  = 300 // Video duration
	PropertyIDVideoWidth     = 301 // Video width
	PropertyIDVideoHeight    = 302 // Video height
	PropertyIDVideoFrameRate = 303 // Video frame rate
)

// Hash/Checksum property IDs
const (
	PropertyIDMD5    = 37  // MD5 hash (16 bytes)
	PropertyIDSHA1   = 38  // SHA1 hash (20 bytes)
	PropertyIDSHA256 = 39  // SHA256 hash (32 bytes)
	PropertyIDCRC32  = 40  // CRC32 checksum (4 bytes)
	PropertyIDSHA512 = 242 // SHA512 hash (64 bytes)
	PropertyIDSHA384 = 243 // SHA384 hash (48 bytes)
	PropertyIDCRC64  = 244 // CRC64 checksum (8 bytes)
)

// Folder hash property IDs (hash of folder contents)
const (
	PropertyIDFolderDataCRC32           = 361 // CRC32 of folder data
	PropertyIDFolderDataCRC64           = 362 // CRC64 of folder data
	PropertyIDFolderDataMD5             = 363 // MD5 of folder data
	PropertyIDFolderDataSHA1            = 364 // SHA1 of folder data
	PropertyIDFolderDataSHA256          = 365 // SHA256 of folder data
	PropertyIDFolderDataSHA512          = 366 // SHA512 of folder data
	PropertyIDFolderDataAndNamesCRC32   = 367 // CRC32 of folder data and names
	PropertyIDFolderDataAndNamesCRC64   = 368 // CRC64 of folder data and names
	PropertyIDFolderDataAndNamesMD5     = 369 // MD5 of folder data and names
	PropertyIDFolderDataAndNamesSHA1    = 370 // SHA1 of folder data and names
	PropertyIDFolderDataAndNamesSHA256  = 371 // SHA256 of folder data and names
	PropertyIDFolderDataAndNamesSHA512  = 372 // SHA512 of folder data and names
	PropertyIDFolderNamesCRC32          = 373 // CRC32 of folder names
	PropertyIDFolderNamesCRC64          = 374 // CRC64 of folder names
	PropertyIDFolderNamesMD5            = 375 // MD5 of folder names
	PropertyIDFolderNamesSHA1           = 376 // SHA1 of folder names
	PropertyIDFolderNamesSHA256         = 377 // SHA256 of folder names
	PropertyIDFolderNamesSHA512         = 378 // SHA512 of folder names
)

// SFV/Checksum file verification property IDs
const (
	PropertyIDSFVCRC32       = 302 // CRC32 from .sfv file
	PropertyIDMD5SumMD5      = 303 // MD5 from .md5sum file
	PropertyIDSHA1SumSHA1    = 304 // SHA1 from .sha1sum file
	PropertyIDSHA256SumSHA256 = 305 // SHA256 from .sha256sum file
	PropertyIDSHA512SumSHA512 = 320 // SHA512 from .sha512sum file
	PropertyIDMD5SumPass     = 307 // MD5sum verification pass/fail
	PropertyIDSHA1SumPass    = 308 // SHA1sum verification pass/fail
	PropertyIDSHA256SumPass  = 309 // SHA256sum verification pass/fail
	PropertyIDSHA512SumPass  = 321 // SHA512sum verification pass/fail
)

// Extended file property IDs
const (
	PropertyIDSizeOnDisk       = 41  // Size on disk (compressed size)
	PropertyIDDescription      = 42  // File description
	PropertyIDVersion          = 43  // File version
	PropertyIDProductName      = 44  // Product name
	PropertyIDProductVersion   = 45  // Product version
	PropertyIDCompany          = 46  // Company name
	PropertyIDKind             = 47  // File kind
	PropertyIDHardLinkCount    = 164 // Number of hard links
	PropertyIDCompressedSize   = 171 // Compressed size
	PropertyIDCompressionRatio = 176 // Compression ratio
	PropertyIDReparseTag       = 177 // Reparse point tag
	PropertyIDFileID           = 190 // NTFS File ID (128-bit)
	PropertyIDVolumeSerial     = 189 // Volume serial number
)

// Document property IDs
const (
	PropertyIDTitle         = 25  // Document title
	PropertyIDArtist        = 26  // Artist/Author
	PropertyIDAlbum         = 27  // Album
	PropertyIDYear          = 28  // Year
	PropertyIDComment       = 29  // Comment
	PropertyIDTrack         = 30  // Track number
	PropertyIDGenre         = 31  // Genre
	PropertyIDRating        = 35  // Rating (0-5)
	PropertyIDTags          = 36  // Tags
	PropertyIDSubject       = 50  // Subject
	PropertyIDAuthors       = 51  // Authors
	PropertyIDCopyright     = 55  // Copyright
	PropertyIDPageCount     = 127 // Page count
	PropertyIDWordCount     = 128 // Word count
	PropertyIDCharacterCount = 129 // Character count
	PropertyIDLineCount     = 130 // Line count
)

// Camera/EXIF property IDs
const (
	PropertyIDDateTaken       = 52  // Date photo was taken
	PropertyIDCameraMaker     = 63  // Camera manufacturer
	PropertyIDCameraModel     = 64  // Camera model
	PropertyIDFStop           = 65  // F-stop value
	PropertyIDExposureTime    = 66  // Exposure time
	PropertyIDISOSpeed        = 67  // ISO speed
	PropertyIDFocalLength     = 69  // Focal length
	PropertyIDFlashMode       = 73  // Flash mode
	PropertyIDLatitude        = 91  // GPS latitude
	PropertyIDLongitude       = 92  // GPS longitude
	PropertyIDAltitude        = 93  // GPS altitude
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

// SearchOptionsWithHashes returns search options that include hash properties
func SearchOptionsWithHashes() *SearchOptions {
	return &SearchOptions{
		Count: 100,
		RequestedProperties: []uint32{
			PropertyIDName,
			PropertyIDPath,
			PropertyIDSize,
			PropertyIDDateModified,
			PropertyIDAttributes,
			PropertyIDCRC32,
			PropertyIDMD5,
			PropertyIDSHA1,
			PropertyIDSHA256,
		},
	}
}

// SearchOptionsWithAllHashes returns search options that include all available hashes
func SearchOptionsWithAllHashes() *SearchOptions {
	return &SearchOptions{
		Count: 100,
		RequestedProperties: []uint32{
			PropertyIDName,
			PropertyIDPath,
			PropertyIDSize,
			PropertyIDDateModified,
			PropertyIDAttributes,
			PropertyIDCRC32,
			PropertyIDCRC64,
			PropertyIDMD5,
			PropertyIDSHA1,
			PropertyIDSHA256,
			PropertyIDSHA384,
			PropertyIDSHA512,
		},
	}
}

// Hash represents a file hash/checksum value
type Hash struct {
	Valid bool   // True if the hash is available
	Value []byte // Raw hash bytes
}

// String returns the hex string representation of the hash
func (h Hash) String() string {
	if !h.Valid || len(h.Value) == 0 {
		return ""
	}
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(h.Value)*2)
	for i, b := range h.Value {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}

// CRC32 returns the CRC32 value as uint32 (0 if not valid)
func (h Hash) CRC32() uint32 {
	if !h.Valid || len(h.Value) < 4 {
		return 0
	}
	return uint32(h.Value[0]) | uint32(h.Value[1])<<8 |
		uint32(h.Value[2])<<16 | uint32(h.Value[3])<<24
}

// CRC64 returns the CRC64 value as uint64 (0 if not valid)
func (h Hash) CRC64() uint64 {
	if !h.Valid || len(h.Value) < 8 {
		return 0
	}
	return uint64(h.Value[0]) | uint64(h.Value[1])<<8 |
		uint64(h.Value[2])<<16 | uint64(h.Value[3])<<24 |
		uint64(h.Value[4])<<32 | uint64(h.Value[5])<<40 |
		uint64(h.Value[6])<<48 | uint64(h.Value[7])<<56
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

	// Hash/Checksum values (populated if requested)
	CRC32  Hash // CRC32 checksum
	CRC64  Hash // CRC64 checksum
	MD5    Hash // MD5 hash
	SHA1   Hash // SHA1 hash
	SHA256 Hash // SHA256 hash
	SHA384 Hash // SHA384 hash
	SHA512 Hash // SHA512 hash

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
