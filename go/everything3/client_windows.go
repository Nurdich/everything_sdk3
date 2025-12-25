//go:build windows

package everything3

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Client represents a connection to the Everything IPC server
type Client struct {
	pipe     windows.Handle
	mu       sync.Mutex
	instance string
}

// IPC message types
const (
	ipcMsgSearch          = 0x0001
	ipcMsgGetResults      = 0x0002
	ipcMsgGetFileAttr     = 0x0003
	ipcMsgGetFileAttrEx   = 0x0004
	ipcMsgFindFirstFile   = 0x0005
	ipcMsgFindNextFile    = 0x0006
	ipcMsgFindClose       = 0x0007
	ipcMsgGetJournalInfo  = 0x0008
	ipcMsgReadJournal     = 0x0009
)

// Connect creates a new connection to the Everything IPC server.
// instanceName can be empty for the default instance, or "1.5a" for the alpha version.
func Connect(instanceName string) (*Client, error) {
	pipeName := `\\.\PIPE\Everything IPC`
	if instanceName != "" {
		pipeName = fmt.Sprintf(`\\.\PIPE\Everything IPC (%s)`, instanceName)
	}

	pipeNamePtr, err := syscall.UTF16PtrFromString(pipeName)
	if err != nil {
		return nil, fmt.Errorf("invalid pipe name: %w", err)
	}

	// Try to connect to the pipe
	var pipe windows.Handle
	for retries := 0; retries < 10; retries++ {
		pipe, err = windows.CreateFile(
			pipeNamePtr,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED,
			0,
		)
		if err == nil {
			break
		}
		if errors.Is(err, windows.ERROR_PIPE_BUSY) {
			// Wait for the pipe to become available
			windows.WaitNamedPipe(pipeNamePtr, 1000)
			continue
		}
		return nil, fmt.Errorf("failed to connect to Everything IPC: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Everything IPC after retries: %w", err)
	}

	// Set pipe to message mode
	mode := uint32(windows.PIPE_READMODE_MESSAGE)
	if err := windows.SetNamedPipeHandleState(pipe, &mode, nil, nil); err != nil {
		windows.CloseHandle(pipe)
		return nil, fmt.Errorf("failed to set pipe mode: %w", err)
	}

	return &Client{
		pipe:     pipe,
		instance: instanceName,
	}, nil
}

// ConnectDefault connects to the default Everything instance.
// It first tries the default instance, then falls back to "1.5a".
func ConnectDefault() (*Client, error) {
	client, err := Connect("")
	if err == nil {
		return client, nil
	}
	return Connect("1.5a")
}

// Close closes the connection to the Everything server
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe != windows.InvalidHandle {
		err := windows.CloseHandle(c.pipe)
		c.pipe = windows.InvalidHandle
		return err
	}
	return nil
}

// sendMessage sends a message to the IPC server and receives the response
func (c *Client) sendMessage(msgType uint32, data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe == windows.InvalidHandle {
		return nil, errors.New("client is closed")
	}

	// Create message header
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], msgType)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(data)))

	// Combine header and data
	message := append(header, data...)

	// Create overlapped structure for async I/O
	var overlapped windows.Overlapped
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	defer windows.CloseHandle(event)
	overlapped.HEvent = event

	// Write message
	var written uint32
	err = windows.WriteFile(c.pipe, message, &written, &overlapped)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil, fmt.Errorf("failed to write to pipe: %w", err)
	}
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		_, err = windows.WaitForSingleObject(event, 30000) // 30 second timeout
		if err != nil {
			return nil, fmt.Errorf("write timeout: %w", err)
		}
		windows.GetOverlappedResult(c.pipe, &overlapped, &written, false)
	}

	// Reset event for read
	windows.ResetEvent(event)

	// Read response header
	responseHeader := make([]byte, 8)
	var read uint32
	err = windows.ReadFile(c.pipe, responseHeader, &read, &overlapped)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil, fmt.Errorf("failed to read response header: %w", err)
	}
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		_, err = windows.WaitForSingleObject(event, 30000)
		if err != nil {
			return nil, fmt.Errorf("read timeout: %w", err)
		}
		windows.GetOverlappedResult(c.pipe, &overlapped, &read, false)
	}

	if read < 8 {
		return nil, errors.New("incomplete response header")
	}

	respType := binary.LittleEndian.Uint32(responseHeader[0:4])
	respLen := binary.LittleEndian.Uint32(responseHeader[4:8])

	if respType == 0xFFFFFFFF {
		// Error response
		errCode := respLen
		return nil, fmt.Errorf("server error: 0x%08X", errCode)
	}

	// Read response body
	if respLen == 0 {
		return nil, nil
	}

	windows.ResetEvent(event)
	response := make([]byte, respLen)
	err = windows.ReadFile(c.pipe, response, &read, &overlapped)
	if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		_, err = windows.WaitForSingleObject(event, 30000)
		if err != nil {
			return nil, fmt.Errorf("read timeout: %w", err)
		}
		windows.GetOverlappedResult(c.pipe, &overlapped, &read, false)
	}

	return response[:read], nil
}

// Search performs a search and returns the results
func (c *Client) Search(opts *SearchOptions) (*ResultList, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	// Build search request
	data := c.buildSearchRequest(opts)

	// Send search request
	response, err := c.sendMessage(ipcMsgSearch, data)
	if err != nil {
		return nil, err
	}

	// Parse response
	return c.parseSearchResponse(response)
}

// buildSearchRequest creates the binary request data for a search
func (c *Client) buildSearchRequest(opts *SearchOptions) []byte {
	// Encode search text as UTF-16LE
	searchTextUTF16 := utf16.Encode([]rune(opts.Text))
	searchTextBytes := make([]byte, (len(searchTextUTF16)+1)*2)
	for i, r := range searchTextUTF16 {
		binary.LittleEndian.PutUint16(searchTextBytes[i*2:], r)
	}

	// Build flags
	var flags uint32
	if opts.MatchCase {
		flags |= 0x0001
	}
	if opts.MatchWholeWords {
		flags |= 0x0002
	}
	if opts.MatchPath {
		flags |= 0x0004
	}
	if opts.UseRegex {
		flags |= 0x0008
	}
	if opts.MatchDiacritics {
		flags |= 0x0010
	}
	if opts.IgnorePunctuation {
		flags |= 0x0020
	}
	if opts.MatchPrefix {
		flags |= 0x0040
	}
	if opts.MatchSuffix {
		flags |= 0x0080
	}

	// Calculate buffer size
	bufSize := 4 + // flags
		4 + len(searchTextBytes) + // text length + text
		4 + // offset
		4 + // count
		4 + // folders first
		4 + // sort count
		len(opts.Sorts)*8 + // sorts (property ID + direction)
		4 + // property count
		len(opts.RequestedProperties)*4 // properties

	data := make([]byte, bufSize)
	offset := 0

	// Write flags
	binary.LittleEndian.PutUint32(data[offset:], flags)
	offset += 4

	// Write search text
	binary.LittleEndian.PutUint32(data[offset:], uint32(len(searchTextBytes)))
	offset += 4
	copy(data[offset:], searchTextBytes)
	offset += len(searchTextBytes)

	// Write pagination
	binary.LittleEndian.PutUint32(data[offset:], opts.Offset)
	offset += 4
	count := opts.Count
	if count == 0 {
		count = 100
	}
	binary.LittleEndian.PutUint32(data[offset:], count)
	offset += 4

	// Write folders first
	binary.LittleEndian.PutUint32(data[offset:], uint32(opts.FoldersFirst))
	offset += 4

	// Write sorts
	binary.LittleEndian.PutUint32(data[offset:], uint32(len(opts.Sorts)))
	offset += 4
	for _, sort := range opts.Sorts {
		binary.LittleEndian.PutUint32(data[offset:], sort.PropertyID)
		offset += 4
		if sort.Ascending {
			binary.LittleEndian.PutUint32(data[offset:], 1)
		} else {
			binary.LittleEndian.PutUint32(data[offset:], 0)
		}
		offset += 4
	}

	// Write requested properties
	binary.LittleEndian.PutUint32(data[offset:], uint32(len(opts.RequestedProperties)))
	offset += 4
	for _, prop := range opts.RequestedProperties {
		binary.LittleEndian.PutUint32(data[offset:], prop)
		offset += 4
	}

	return data[:offset]
}

// parseSearchResponse parses the binary response from a search
func (c *Client) parseSearchResponse(data []byte) (*ResultList, error) {
	if len(data) < 32 {
		return nil, errors.New("response too short")
	}

	result := &ResultList{
		Results: make([]Result, 0),
	}

	offset := 0

	// Read counts
	result.TotalCount = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	result.FolderCount = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	result.FileCount = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	result.ViewportCount = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	// Read results
	for i := uint64(0); i < result.ViewportCount && offset < len(data); i++ {
		r, bytesRead, err := c.parseResult(data[offset:])
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		result.Results = append(result.Results, r)
		offset += bytesRead
	}

	return result, nil
}

// parseResult parses a single result from the response
func (c *Client) parseResult(data []byte) (Result, int, error) {
	if len(data) < 4 {
		return Result{}, 0, io.EOF
	}

	r := Result{
		Properties: make(map[uint32]interface{}),
	}
	offset := 0

	// Read flags
	flags := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	r.IsFolder = flags&0x01 != 0

	// Read name
	name, n := readUTF16String(data[offset:])
	r.Name = name
	offset += n

	// Read path
	path, n := readUTF16String(data[offset:])
	r.Path = path
	offset += n

	// Build full path
	r.FullPath = filepath.Join(r.Path, r.Name)

	// Read size
	if offset+8 <= len(data) {
		r.Size = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
	}

	// Read dates (FILETIME format)
	if offset+8 <= len(data) {
		r.DateModified = filetimeToTime(binary.LittleEndian.Uint64(data[offset:]))
		offset += 8
	}
	if offset+8 <= len(data) {
		r.DateCreated = filetimeToTime(binary.LittleEndian.Uint64(data[offset:]))
		offset += 8
	}
	if offset+8 <= len(data) {
		r.DateAccessed = filetimeToTime(binary.LittleEndian.Uint64(data[offset:]))
		offset += 8
	}

	// Read attributes
	if offset+4 <= len(data) {
		r.Attributes = binary.LittleEndian.Uint32(data[offset:])
		offset += 4
	}

	return r, offset, nil
}

// readUTF16String reads a null-terminated UTF-16LE string
func readUTF16String(data []byte) (string, int) {
	if len(data) < 4 {
		return "", 0
	}

	// Read length
	strLen := binary.LittleEndian.Uint32(data[0:4])
	if strLen == 0 {
		return "", 4
	}

	byteLen := int(strLen * 2)
	if len(data) < 4+byteLen {
		return "", 4
	}

	// Decode UTF-16LE
	utf16Chars := make([]uint16, strLen)
	for i := uint32(0); i < strLen; i++ {
		utf16Chars[i] = binary.LittleEndian.Uint16(data[4+i*2:])
	}

	return string(utf16.Decode(utf16Chars)), 4 + byteLen
}

// filetimeToTime converts a Windows FILETIME to Go time.Time
func filetimeToTime(ft uint64) time.Time {
	if ft == 0 {
		return time.Time{}
	}
	// FILETIME is 100-nanosecond intervals since January 1, 1601
	// Go time.Time uses nanoseconds since January 1, 1970
	const epochDiff = 116444736000000000 // 100-ns intervals between 1601 and 1970
	ns := (int64(ft) - epochDiff) * 100
	return time.Unix(0, ns)
}

// GetFileAttributes returns the file attributes for the specified file
func (c *Client) GetFileAttributes(path string) (uint32, error) {
	// Encode path as UTF-16LE
	pathUTF16 := utf16.Encode([]rune(path))
	pathBytes := make([]byte, (len(pathUTF16)+1)*2)
	for i, r := range pathUTF16 {
		binary.LittleEndian.PutUint16(pathBytes[i*2:], r)
	}

	response, err := c.sendMessage(ipcMsgGetFileAttr, pathBytes)
	if err != nil {
		return 0, err
	}

	if len(response) < 4 {
		return 0, errors.New("invalid response")
	}

	return binary.LittleEndian.Uint32(response), nil
}

// GetFileInfo returns detailed file information for the specified file
func (c *Client) GetFileInfo(path string) (*FileInfo, error) {
	// Encode path as UTF-16LE
	pathUTF16 := utf16.Encode([]rune(path))
	pathBytes := make([]byte, (len(pathUTF16)+1)*2)
	for i, r := range pathUTF16 {
		binary.LittleEndian.PutUint16(pathBytes[i*2:], r)
	}

	response, err := c.sendMessage(ipcMsgGetFileAttrEx, pathBytes)
	if err != nil {
		return nil, err
	}

	if len(response) < 44 {
		return nil, errors.New("invalid response")
	}

	fi := &FileInfo{}
	offset := 0

	// Parse WIN32_FIND_DATA structure
	fi.Attributes = binary.LittleEndian.Uint32(response[offset:])
	offset += 4

	fi.CreationTime = filetimeToTime(binary.LittleEndian.Uint64(response[offset:]))
	offset += 8

	fi.LastAccessTime = filetimeToTime(binary.LittleEndian.Uint64(response[offset:]))
	offset += 8

	fi.LastWriteTime = filetimeToTime(binary.LittleEndian.Uint64(response[offset:]))
	offset += 8

	sizeHigh := binary.LittleEndian.Uint32(response[offset:])
	offset += 4
	sizeLow := binary.LittleEndian.Uint32(response[offset:])
	offset += 4
	fi.Size = uint64(sizeHigh)<<32 | uint64(sizeLow)

	// Read filename
	fi.Name, _ = readUTF16String(response[offset:])

	return fi, nil
}

// FindHandle represents a file search iterator
type FindHandle struct {
	client   *Client
	handleID uint64
}

// FindFirstFile starts a file search for the specified pattern
func (c *Client) FindFirstFile(pattern string) (*FindHandle, *FileInfo, error) {
	// Encode pattern as UTF-16LE
	patternUTF16 := utf16.Encode([]rune(pattern))
	patternBytes := make([]byte, (len(patternUTF16)+1)*2)
	for i, r := range patternUTF16 {
		binary.LittleEndian.PutUint16(patternBytes[i*2:], r)
	}

	response, err := c.sendMessage(ipcMsgFindFirstFile, patternBytes)
	if err != nil {
		return nil, nil, err
	}

	if len(response) < 8 {
		return nil, nil, errors.New("invalid response")
	}

	handleID := binary.LittleEndian.Uint64(response[0:8])
	if handleID == 0 {
		return nil, nil, errors.New("no files found")
	}

	handle := &FindHandle{
		client:   c,
		handleID: handleID,
	}

	fi, err := parseFindData(response[8:])
	if err != nil {
		handle.Close()
		return nil, nil, err
	}

	return handle, fi, nil
}

// Next returns the next file in the search
func (h *FindHandle) Next() (*FileInfo, error) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, h.handleID)

	response, err := h.client.sendMessage(ipcMsgFindNextFile, data)
	if err != nil {
		return nil, err
	}

	if len(response) < 4 {
		return nil, io.EOF
	}

	success := binary.LittleEndian.Uint32(response[0:4])
	if success == 0 {
		return nil, io.EOF
	}

	return parseFindData(response[4:])
}

// Close closes the find handle
func (h *FindHandle) Close() error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, h.handleID)
	_, err := h.client.sendMessage(ipcMsgFindClose, data)
	return err
}

// parseFindData parses file information from response data
func parseFindData(data []byte) (*FileInfo, error) {
	if len(data) < 36 {
		return nil, errors.New("invalid find data")
	}

	fi := &FileInfo{}
	offset := 0

	fi.Attributes = binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	fi.CreationTime = filetimeToTime(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	fi.LastAccessTime = filetimeToTime(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	fi.LastWriteTime = filetimeToTime(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	sizeHigh := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	sizeLow := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	fi.Size = uint64(sizeHigh)<<32 | uint64(sizeLow)

	if offset < len(data) {
		fi.Name, _ = readUTF16String(data[offset:])
	}

	return fi, nil
}

// FormatSize returns a human-readable file size
func FormatSize(size uint64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := uint64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// Ensure Client implements io.Closer
var _ io.Closer = (*Client)(nil)

// Platform check
func init() {
	// This package only works on Windows
	_ = unsafe.Sizeof(windows.Handle(0))
}
