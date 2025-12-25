//go:build !windows

package everything3

import (
	"errors"
	"runtime"
)

// ErrNotSupported is returned when trying to use the library on non-Windows platforms
var ErrNotSupported = errors.New("everything3: this library only works on Windows")

// Client represents a connection to the Everything IPC server
type Client struct{}

// Connect creates a new connection to the Everything IPC server.
// This function is not supported on non-Windows platforms.
func Connect(instanceName string) (*Client, error) {
	return nil, ErrNotSupported
}

// ConnectDefault connects to the default Everything instance.
// This function is not supported on non-Windows platforms.
func ConnectDefault() (*Client, error) {
	return nil, ErrNotSupported
}

// FindEverythingPath searches for the Everything.exe installation path.
// This function is not supported on non-Windows platforms.
func FindEverythingPath() (string, error) {
	return "", ErrNotSupported
}

// IsEverythingRunning checks if Everything is currently running.
// This function is not supported on non-Windows platforms.
func IsEverythingRunning(instanceName string) bool {
	return false
}

// StartEverything starts the Everything application if it's not already running.
// This function is not supported on non-Windows platforms.
func StartEverything() (string, error) {
	return "", ErrNotSupported
}

// ConnectOrStart connects to the Everything IPC server, starting Everything if it's not running.
// This function is not supported on non-Windows platforms.
func ConnectOrStart(instanceName string) (*Client, error) {
	return nil, ErrNotSupported
}

// ConnectOrStartDefault connects to Everything, starting it if necessary.
// This function is not supported on non-Windows platforms.
func ConnectOrStartDefault() (*Client, error) {
	return nil, ErrNotSupported
}

// Close closes the connection to the Everything server
func (c *Client) Close() error {
	return ErrNotSupported
}

// Search performs a search and returns the results
func (c *Client) Search(opts *SearchOptions) (*ResultList, error) {
	return nil, ErrNotSupported
}

// GetFileAttributes returns the file attributes for the specified file
func (c *Client) GetFileAttributes(path string) (uint32, error) {
	return 0, ErrNotSupported
}

// GetFileInfo returns detailed file information for the specified file
func (c *Client) GetFileInfo(path string) (*FileInfo, error) {
	return nil, ErrNotSupported
}

// FindHandle represents a file search iterator
type FindHandle struct{}

// FindFirstFile starts a file search for the specified pattern
func (c *Client) FindFirstFile(pattern string) (*FindHandle, *FileInfo, error) {
	return nil, nil, ErrNotSupported
}

// Next returns the next file in the search
func (h *FindHandle) Next() (*FileInfo, error) {
	return nil, ErrNotSupported
}

// Close closes the find handle
func (h *FindHandle) Close() error {
	return ErrNotSupported
}

// FormatSize returns a human-readable file size
func FormatSize(size uint64) string {
	return ""
}

// GetFileHash returns the hash value for a file
func (c *Client) GetFileHash(path string, propertyID uint32) (Hash, error) {
	return Hash{}, ErrNotSupported
}

// GetFileCRC32 returns the CRC32 checksum for a file
func (c *Client) GetFileCRC32(path string) (uint32, error) {
	return 0, ErrNotSupported
}

// GetFileCRC64 returns the CRC64 checksum for a file
func (c *Client) GetFileCRC64(path string) (uint64, error) {
	return 0, ErrNotSupported
}

// GetFileMD5 returns the MD5 hash for a file as a hex string
func (c *Client) GetFileMD5(path string) (string, error) {
	return "", ErrNotSupported
}

// GetFileSHA1 returns the SHA1 hash for a file as a hex string
func (c *Client) GetFileSHA1(path string) (string, error) {
	return "", ErrNotSupported
}

// GetFileSHA256 returns the SHA256 hash for a file as a hex string
func (c *Client) GetFileSHA256(path string) (string, error) {
	return "", ErrNotSupported
}

// GetFileSHA512 returns the SHA512 hash for a file as a hex string
func (c *Client) GetFileSHA512(path string) (string, error) {
	return "", ErrNotSupported
}

// GetFileHashes returns multiple hash values for a file
func (c *Client) GetFileHashes(path string) (*FileHashes, error) {
	return nil, ErrNotSupported
}

// FileHashes contains all hash values for a file
type FileHashes struct {
	CRC32  Hash
	CRC64  Hash
	MD5    Hash
	SHA1   Hash
	SHA256 Hash
	SHA384 Hash
	SHA512 Hash
}

// GetPropertyString returns a string property value for a file
func (c *Client) GetPropertyString(path string, propertyID uint32) (string, error) {
	return "", ErrNotSupported
}

// GetPropertyUint64 returns a uint64 property value for a file
func (c *Client) GetPropertyUint64(path string, propertyID uint32) (uint64, error) {
	return 0, ErrNotSupported
}

// GetPropertyUint32 returns a uint32 property value for a file
func (c *Client) GetPropertyUint32(path string, propertyID uint32) (uint32, error) {
	return 0, ErrNotSupported
}

// IsWindowsAvailable returns whether the current platform is Windows
func IsWindowsAvailable() bool {
	return runtime.GOOS == "windows"
}

func init() {
	// This package only works on Windows
}
