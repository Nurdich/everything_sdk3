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
	const unit = 1024
	if size < unit {
		return ""
	}
	div, exp := uint64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return ""
}

// IsWindowsAvailable returns whether the current platform is Windows
func IsWindowsAvailable() bool {
	return runtime.GOOS == "windows"
}

func init() {
	// This package only works on Windows
	if runtime.GOOS != "windows" {
		// Log a warning or just continue - errors will be returned at runtime
	}
}
