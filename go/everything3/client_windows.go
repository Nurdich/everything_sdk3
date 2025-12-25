//go:build windows

package everything3

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procWaitNamedPipe = kernel32.NewProc("WaitNamedPipeW")
)

func waitNamedPipe(name *uint16, timeout uint32) error {
	r1, _, err := procWaitNamedPipe.Call(uintptr(unsafe.Pointer(name)), uintptr(timeout))
	if r1 == 0 {
		return err
	}
	return nil
}

// Client represents a connection to the Everything IPC server
type Client struct {
	pipe       windows.Handle
	mu         sync.Mutex
	instance   string
	sendEvent  windows.Handle
	recvEvent  windows.Handle
	is64bit    bool // true if running as 64-bit process
}

// IPC command codes (from Everything3.c)
const (
	cmdGetIPCPipeVersion  = 0
	cmdGetMajorVersion    = 1
	cmdGetMinorVersion    = 2
	cmdGetRevision        = 3
	cmdGetBuildNumber     = 4
	cmdGetTargetMachine   = 5
	cmdFindPropertyName   = 6
	cmdSearch             = 7
	cmdIsDBLoaded         = 8
	cmdIsPropertyIndexed  = 9
	cmdIsPropertyFastSort = 10
	cmdGetPropertyName    = 11
	cmdGetPropertyCanonicalName = 12
	cmdGetPropertyType    = 13
	cmdIsResultChange     = 14
	cmdGetRunCount        = 15
	cmdSetRunCount        = 16
	cmdIncRunCount        = 17
	cmdGetFolderSize      = 18
	cmdGetFileAttributes  = 19
	cmdGetFileAttributesEx = 20
	cmdFindFirstFile      = 21
	cmdGetResults         = 22
	cmdSort               = 23
	cmdWaitForResultChange = 24
)

// IPC response codes
const (
	respOKMoreData     = 100
	respOK             = 200
	respBadRequest     = 400
	respCancelled      = 401
	respNotFound       = 404
	respOutOfMemory    = 500
	respInvalidCommand = 501
)

// Search flags
const (
	searchFlagMatchCase            = 0x00000001
	searchFlagMatchWholeWord       = 0x00000002
	searchFlagMatchPath            = 0x00000004
	searchFlagRegex                = 0x00000008
	searchFlagMatchDiacritics      = 0x00000010
	searchFlagMatchPrefix          = 0x00000020
	searchFlagMatchSuffix          = 0x00000040
	searchFlagIgnorePunctuation    = 0x00000080
	searchFlagIgnoreWhitespace     = 0x00000100
	searchFlagFoldersFirstAscending  = 0x00000000
	searchFlagFoldersFirstAlways   = 0x00000200
	searchFlagFoldersFirstNever    = 0x00000400
	searchFlagFoldersFirstDescending = 0x00000600
	searchFlag64Bit                = 0x00004000
)

// Sort flags
const (
	sortFlagDescending = 0x00000001
)

// Result item flags
const (
	resultFlagFolder = 0x01
	resultFlagRoot   = 0x02
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
			waitNamedPipe(pipeNamePtr, 1000)
			continue
		}
		return nil, fmt.Errorf("failed to connect to Everything IPC: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to Everything IPC after retries: %w", err)
	}

	// Create events for overlapped I/O
	sendEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(pipe)
		return nil, fmt.Errorf("failed to create send event: %w", err)
	}

	recvEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		windows.CloseHandle(pipe)
		windows.CloseHandle(sendEvent)
		return nil, fmt.Errorf("failed to create recv event: %w", err)
	}

	return &Client{
		pipe:      pipe,
		instance:  instanceName,
		sendEvent: sendEvent,
		recvEvent: recvEvent,
		is64bit:   unsafe.Sizeof(uintptr(0)) == 8,
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

// FindEverythingPath searches for the Everything.exe installation path.
// It checks the Windows registry first, then common installation directories.
func FindEverythingPath() (string, error) {
	// Try to find from registry (HKLM and HKCU)
	registryPaths := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\voidtools\Everything`},
		{registry.CURRENT_USER, `SOFTWARE\voidtools\Everything`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\voidtools\Everything`},
	}

	for _, rp := range registryPaths {
		key, err := registry.OpenKey(rp.root, rp.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		// Try "InstallPath" or "Install_Dir" value
		for _, valueName := range []string{"InstallPath", "Install_Dir", ""} {
			val, _, err := key.GetStringValue(valueName)
			if err == nil && val != "" {
				exePath := filepath.Join(val, "Everything.exe")
				if fileExists(exePath) {
					key.Close()
					return exePath, nil
				}
				// Maybe the value is the exe path itself
				if fileExists(val) {
					key.Close()
					return val, nil
				}
			}
		}
		key.Close()
	}

	// Try common installation paths
	commonPaths := []string{
		`C:\Program Files\Everything\Everything.exe`,
		`C:\Program Files (x86)\Everything\Everything.exe`,
		`C:\Program Files\Everything 1.5a\Everything.exe`,
		`C:\Program Files (x86)\Everything 1.5a\Everything.exe`,
	}

	// Also check user's local app data
	if appData := getenv("LOCALAPPDATA"); appData != "" {
		commonPaths = append(commonPaths,
			filepath.Join(appData, "Everything", "Everything.exe"),
			filepath.Join(appData, "Programs", "Everything", "Everything.exe"),
		)
	}

	for _, path := range commonPaths {
		if fileExists(path) {
			return path, nil
		}
	}

	return "", errors.New("Everything.exe not found")
}

// IsEverythingRunning checks if Everything is currently running by trying to connect to its IPC pipe.
func IsEverythingRunning(instanceName string) bool {
	pipeName := `\\.\PIPE\Everything IPC`
	if instanceName != "" {
		pipeName = fmt.Sprintf(`\\.\PIPE\Everything IPC (%s)`, instanceName)
	}

	pipeNamePtr, err := syscall.UTF16PtrFromString(pipeName)
	if err != nil {
		return false
	}

	// Try to open the pipe briefly
	pipe, err := windows.CreateFile(
		pipeNamePtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return false
	}
	windows.CloseHandle(pipe)
	return true
}

// StartEverything starts the Everything application if it's not already running.
// Returns the path to Everything.exe that was started, or an error if it couldn't be started.
func StartEverything() (string, error) {
	// Check if already running
	if IsEverythingRunning("") || IsEverythingRunning("1.5a") {
		return "", nil // Already running
	}

	// Find Everything.exe
	exePath, err := FindEverythingPath()
	if err != nil {
		return "", fmt.Errorf("cannot find Everything.exe: %w", err)
	}

	// Start Everything with -startup flag (starts minimized/background)
	cmd := fmt.Sprintf(`"%s" -startup`, exePath)
	cmdPtr, err := syscall.UTF16PtrFromString(cmd)
	if err != nil {
		return "", err
	}

	var si windows.StartupInfo
	var pi windows.ProcessInformation
	si.Cb = uint32(unsafe.Sizeof(si))

	err = windows.CreateProcess(
		nil,
		cmdPtr,
		nil,
		nil,
		false,
		windows.CREATE_NO_WINDOW,
		nil,
		nil,
		&si,
		&pi,
	)
	if err != nil {
		return "", fmt.Errorf("failed to start Everything: %w", err)
	}

	// Close process handles (we don't need to wait for it)
	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return exePath, nil
}

// ConnectOrStart connects to the Everything IPC server, starting Everything if it's not running.
// This function will:
// 1. Try to connect to an existing Everything instance
// 2. If no instance is running, find and start Everything.exe
// 3. Wait for Everything to become available and connect
func ConnectOrStart(instanceName string) (*Client, error) {
	// First, try to connect directly
	client, err := Connect(instanceName)
	if err == nil {
		return client, nil
	}

	// Connection failed, try to start Everything
	exePath, startErr := StartEverything()
	if startErr != nil {
		return nil, fmt.Errorf("cannot connect to Everything and failed to start it: connect error: %v, start error: %w", err, startErr)
	}

	// Wait for Everything to start and become available
	var lastErr error
	for i := 0; i < 50; i++ { // Wait up to 5 seconds
		time.Sleep(100 * time.Millisecond)

		client, lastErr = Connect(instanceName)
		if lastErr == nil {
			return client, nil
		}
	}

	if exePath != "" {
		return nil, fmt.Errorf("started Everything (%s) but failed to connect: %w", exePath, lastErr)
	}
	return nil, fmt.Errorf("failed to connect to Everything: %w", lastErr)
}

// ConnectOrStartDefault connects to Everything, starting it if necessary.
// It tries both the default instance and the "1.5a" instance.
func ConnectOrStartDefault() (*Client, error) {
	// Try default instance first
	client, err := ConnectOrStart("")
	if err == nil {
		return client, nil
	}

	// Try 1.5a instance
	return ConnectOrStart("1.5a")
}

// Helper function to check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Helper function to get environment variable
func getenv(key string) string {
	return os.Getenv(key)
}

// Close closes the connection to the Everything server
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe != windows.InvalidHandle {
		windows.CloseHandle(c.pipe)
		c.pipe = windows.InvalidHandle
	}
	if c.sendEvent != 0 {
		windows.CloseHandle(c.sendEvent)
		c.sendEvent = 0
	}
	if c.recvEvent != 0 {
		windows.CloseHandle(c.recvEvent)
		c.recvEvent = 0
	}
	return nil
}

// writePipe writes data to the pipe with overlapped I/O
func (c *Client) writePipe(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	var overlapped windows.Overlapped
	overlapped.HEvent = c.sendEvent

	offset := 0
	for offset < len(data) {
		windows.ResetEvent(c.sendEvent)
		var written uint32
		err := windows.WriteFile(c.pipe, data[offset:], &written, &overlapped)
		if err != nil {
			if errors.Is(err, windows.ERROR_IO_PENDING) {
				_, err = windows.WaitForSingleObject(c.sendEvent, 30000)
				if err != nil {
					return fmt.Errorf("write timeout: %w", err)
				}
				windows.GetOverlappedResult(c.pipe, &overlapped, &written, false)
			} else {
				return fmt.Errorf("write error: %w", err)
			}
		}
		offset += int(written)
	}
	return nil
}

// readPipe reads data from the pipe with overlapped I/O
func (c *Client) readPipe(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	var overlapped windows.Overlapped
	overlapped.HEvent = c.recvEvent

	offset := 0
	for offset < len(data) {
		windows.ResetEvent(c.recvEvent)
		var read uint32
		err := windows.ReadFile(c.pipe, data[offset:], &read, &overlapped)
		if err != nil {
			if errors.Is(err, windows.ERROR_IO_PENDING) {
				_, err = windows.WaitForSingleObject(c.recvEvent, 30000)
				if err != nil {
					return fmt.Errorf("read timeout: %w", err)
				}
				windows.GetOverlappedResult(c.pipe, &overlapped, &read, false)
			} else {
				return fmt.Errorf("read error: %w", err)
			}
		}
		if read == 0 {
			return io.EOF
		}
		offset += int(read)
	}
	return nil
}

// sendCommand sends a command and receives the response
func (c *Client) sendCommand(code uint32, data []byte) (uint32, []byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe == windows.InvalidHandle {
		return 0, nil, errors.New("client is closed")
	}

	// Send message header (code + size)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], code)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(data)))

	if err := c.writePipe(header); err != nil {
		return 0, nil, err
	}

	// Send data
	if err := c.writePipe(data); err != nil {
		return 0, nil, err
	}

	// Read response header
	respHeader := make([]byte, 8)
	if err := c.readPipe(respHeader); err != nil {
		return 0, nil, err
	}

	respCode := binary.LittleEndian.Uint32(respHeader[0:4])
	respSize := binary.LittleEndian.Uint32(respHeader[4:8])

	// Read response data
	var respData []byte
	if respSize > 0 {
		respData = make([]byte, respSize)
		if err := c.readPipe(respData); err != nil {
			return 0, nil, err
		}
	}

	return respCode, respData, nil
}

// encodeVLQ encodes a size using variable-length quantity encoding
func encodeVLQ(value uint64) []byte {
	if value < 0xff {
		return []byte{byte(value)}
	}

	value -= 0xff
	if value < 0xffff {
		result := make([]byte, 3)
		result[0] = 0xff
		binary.LittleEndian.PutUint16(result[1:], uint16(value))
		return result
	}

	value -= 0xffff
	if value < 0xffffffff {
		result := make([]byte, 7)
		result[0] = 0xff
		binary.LittleEndian.PutUint16(result[1:3], 0xffff)
		binary.LittleEndian.PutUint32(result[3:], uint32(value))
		return result
	}

	value -= 0xffffffff
	result := make([]byte, 15)
	result[0] = 0xff
	binary.LittleEndian.PutUint16(result[1:3], 0xffff)
	binary.LittleEndian.PutUint32(result[3:7], 0xffffffff)
	binary.LittleEndian.PutUint64(result[7:], value)
	return result
}

// decodeVLQ decodes a VLQ-encoded size and returns the value and bytes consumed
func decodeVLQ(data []byte) (uint64, int) {
	if len(data) == 0 {
		return 0, 0
	}

	if data[0] < 0xff {
		return uint64(data[0]), 1
	}

	if len(data) < 3 {
		return 0, 0
	}

	val16 := binary.LittleEndian.Uint16(data[1:3])
	if val16 < 0xffff {
		return uint64(val16) + 0xff, 3
	}

	if len(data) < 7 {
		return 0, 0
	}

	val32 := binary.LittleEndian.Uint32(data[3:7])
	if val32 < 0xffffffff {
		return uint64(val32) + 0xff + 0xffff, 7
	}

	if len(data) < 15 {
		return 0, 0
	}

	val64 := binary.LittleEndian.Uint64(data[7:15])
	return val64 + 0xff + 0xffff + 0xffffffff, 15
}

// Search performs a search and returns the results
func (c *Client) Search(opts *SearchOptions) (*ResultList, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	// Build search request
	data := c.buildSearchRequest(opts)

	// Send search request
	respCode, respData, err := c.sendCommand(cmdSearch, data)
	if err != nil {
		return nil, err
	}

	if respCode != respOK && respCode != respOKMoreData {
		return nil, fmt.Errorf("search failed with response code %d", respCode)
	}

	// Parse response
	return c.parseSearchResponse(respData)
}

// buildSearchRequest creates the binary request data for a search
func (c *Client) buildSearchRequest(opts *SearchOptions) []byte {
	// Convert search text to UTF-8
	searchTextUTF8 := []byte(opts.Text)

	// Build flags
	var flags uint32
	if opts.MatchCase {
		flags |= searchFlagMatchCase
	}
	if opts.MatchWholeWords {
		flags |= searchFlagMatchWholeWord
	}
	if opts.MatchPath {
		flags |= searchFlagMatchPath
	}
	if opts.UseRegex {
		flags |= searchFlagRegex
	}
	if opts.MatchDiacritics {
		flags |= searchFlagMatchDiacritics
	}
	if opts.MatchPrefix {
		flags |= searchFlagMatchPrefix
	}
	if opts.MatchSuffix {
		flags |= searchFlagMatchSuffix
	}
	if opts.IgnorePunctuation {
		flags |= searchFlagIgnorePunctuation
	}

	// Add folders first flags
	switch opts.FoldersFirst {
	case FoldersFirstYes:
		flags |= searchFlagFoldersFirstAlways
	case FoldersFirstNo:
		flags |= searchFlagFoldersFirstNever
	case FoldersFirstFilesFirst:
		flags |= searchFlagFoldersFirstDescending
	}

	// Add 64-bit flag if running as 64-bit
	if c.is64bit {
		flags |= searchFlag64Bit
	}

	// Size of SIZE_T depends on platform
	sizeTLen := 4
	if c.is64bit {
		sizeTLen = 8
	}

	// Calculate buffer size
	searchLenVLQ := encodeVLQ(uint64(len(searchTextUTF8)))
	sortCountVLQ := encodeVLQ(uint64(len(opts.Sorts)))
	propCountVLQ := encodeVLQ(uint64(len(opts.RequestedProperties)))

	bufSize := 4 + // flags
		len(searchLenVLQ) + len(searchTextUTF8) + // search text
		sizeTLen + sizeTLen + // viewport offset and count
		len(sortCountVLQ) + len(opts.Sorts)*8 + // sorts
		len(propCountVLQ) + len(opts.RequestedProperties)*8 // properties

	data := make([]byte, bufSize)
	offset := 0

	// Write flags
	binary.LittleEndian.PutUint32(data[offset:], flags)
	offset += 4

	// Write search text (VLQ length + UTF-8 bytes)
	copy(data[offset:], searchLenVLQ)
	offset += len(searchLenVLQ)
	copy(data[offset:], searchTextUTF8)
	offset += len(searchTextUTF8)

	// Write viewport offset and count
	if c.is64bit {
		binary.LittleEndian.PutUint64(data[offset:], uint64(opts.Offset))
		offset += 8
		count := opts.Count
		if count == 0 {
			count = 100
		}
		binary.LittleEndian.PutUint64(data[offset:], uint64(count))
		offset += 8
	} else {
		binary.LittleEndian.PutUint32(data[offset:], opts.Offset)
		offset += 4
		count := opts.Count
		if count == 0 {
			count = 100
		}
		binary.LittleEndian.PutUint32(data[offset:], count)
		offset += 4
	}

	// Write sorts
	copy(data[offset:], sortCountVLQ)
	offset += len(sortCountVLQ)
	for _, sort := range opts.Sorts {
		binary.LittleEndian.PutUint32(data[offset:], sort.PropertyID)
		offset += 4
		var sortFlags uint32
		if !sort.Ascending {
			sortFlags |= sortFlagDescending
		}
		binary.LittleEndian.PutUint32(data[offset:], sortFlags)
		offset += 4
	}

	// Write property requests
	copy(data[offset:], propCountVLQ)
	offset += len(propCountVLQ)
	for _, prop := range opts.RequestedProperties {
		binary.LittleEndian.PutUint32(data[offset:], prop)
		offset += 4
		// Property request flags (0 = no formatting/highlighting)
		binary.LittleEndian.PutUint32(data[offset:], 0)
		offset += 4
	}

	return data[:offset]
}

// Property value types from Everything SDK (from Everything3.h)
const (
	propValueTypeNull                   = 0
	propValueTypeByte                   = 1
	propValueTypeWord                   = 2
	propValueTypeDWord                  = 3
	propValueTypeDWordFixedQ1K          = 4
	propValueTypeUint64                 = 5
	propValueTypeUint128                = 6
	propValueTypeDimensions             = 7
	propValueTypePString                = 8
	propValueTypePStringMultistring     = 9
	propValueTypePStringStringRef       = 10
	propValueTypeSizeT                  = 11
	propValueTypeInt32FixedQ1K          = 12
	propValueTypeInt32FixedQ1M          = 13
	propValueTypePStringFolderRef       = 14
	propValueTypePStringFileOrFolderRef = 15
	propValueTypeBlob8                  = 16
	propValueTypeDWordGetText           = 17
	propValueTypeWordGetText            = 18
	propValueTypeBlob16                 = 19
	propValueTypeByteGetText            = 20
	propValueTypePropVariant            = 21
)

// propertyInfo holds information about a requested property
type propertyInfo struct {
	propertyID uint32
	flags      uint32
	valueType  byte
}

// parseSearchResponse parses the binary response from a search
func (c *Client) parseSearchResponse(data []byte) (*ResultList, error) {
	if len(data) < 4 {
		return nil, errors.New("response too short")
	}

	result := &ResultList{
		Results: make([]Result, 0),
	}

	offset := 0

	// Read valid_flags (DWORD)
	validFlags := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// Determine SIZE_T size based on 64-bit flag in response
	sizeTLen := 4
	is64bit := (validFlags & searchFlag64Bit) != 0
	if is64bit {
		sizeTLen = 8
	}

	// Read folder_result_count and file_result_count
	if offset+sizeTLen*2 > len(data) {
		return nil, errors.New("response too short for counts")
	}

	if is64bit {
		result.FolderCount = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
		result.FileCount = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
	} else {
		result.FolderCount = uint64(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
		result.FileCount = uint64(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
	}
	result.TotalCount = result.FolderCount + result.FileCount

	// Skip total_result_size if TOTAL_SIZE flag is set (we didn't request it)
	// The flag is 0x00000800
	if validFlags&0x00000800 != 0 {
		if offset+8 > len(data) {
			return result, nil
		}
		offset += 8 // Skip uint64 total_result_size
	}

	// Read viewport_offset and viewport_count
	if offset+sizeTLen*2 > len(data) {
		return result, nil
	}

	var viewportOffset uint64
	if is64bit {
		viewportOffset = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
		result.ViewportCount = binary.LittleEndian.Uint64(data[offset:])
		offset += 8
	} else {
		viewportOffset = uint64(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
		result.ViewportCount = uint64(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
	}
	_ = viewportOffset

	// Read sort count and skip sort info
	sortCount, vlqLen := decodeVLQ(data[offset:])
	offset += vlqLen

	// Skip sort info (each is DWORD property_id + DWORD flags)
	for i := uint64(0); i < sortCount; i++ {
		if offset+8 > len(data) {
			return result, nil
		}
		offset += 8 // DWORD + DWORD
	}

	// Read property request count
	propCount, vlqLen := decodeVLQ(data[offset:])
	offset += vlqLen

	// Read property request info (each is DWORD property_id + DWORD flags + BYTE value_type)
	properties := make([]propertyInfo, propCount)
	for i := uint64(0); i < propCount; i++ {
		if offset+9 > len(data) {
			break
		}
		properties[i].propertyID = binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		properties[i].flags = binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		properties[i].valueType = data[offset]
		offset++
	}

	// Parse result items
	for i := uint64(0); i < result.ViewportCount && offset < len(data); i++ {
		r, bytesRead, err := c.parseResultItemWithProps(data[offset:], properties, sizeTLen)
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		result.Results = append(result.Results, r)
		offset += bytesRead
	}

	return result, nil
}

// parseResultItemWithProps parses a single result item based on property info
func (c *Client) parseResultItemWithProps(data []byte, properties []propertyInfo, sizeTLen int) (Result, int, error) {
	if len(data) < 1 {
		return Result{}, 0, io.EOF
	}

	r := Result{
		Properties: make(map[uint32]interface{}),
	}
	offset := 0

	// Read item flags (1 byte)
	itemFlags := data[offset]
	offset++
	r.IsFolder = itemFlags&resultFlagFolder != 0

	// Read each property based on its value type
	for _, prop := range properties {
		if offset >= len(data) {
			break
		}

		switch prop.valueType {
		case propValueTypePString, propValueTypePStringMultistring,
			propValueTypePStringStringRef, propValueTypePStringFolderRef,
			propValueTypePStringFileOrFolderRef:
			// VLQ length + UTF-8 string
			str, n := c.readUTF8PString(data[offset:])
			offset += n

			// Store in the appropriate field based on property ID
			switch prop.propertyID {
			case PropertyIDName:
				r.Name = str
			case PropertyIDPath:
				r.Path = str
			case PropertyIDExtension:
				r.Extension = str
			case PropertyIDType:
				r.Type = str
			default:
				r.Properties[prop.propertyID] = str
			}

		case propValueTypeByte, propValueTypeByteGetText:
			if offset+1 > len(data) {
				break
			}
			val := data[offset]
			offset++
			r.Properties[prop.propertyID] = val

		case propValueTypeWord, propValueTypeWordGetText:
			if offset+2 > len(data) {
				break
			}
			val := binary.LittleEndian.Uint16(data[offset:])
			offset += 2
			r.Properties[prop.propertyID] = val

		case propValueTypeDWord, propValueTypeDWordFixedQ1K, propValueTypeDWordGetText:
			if offset+4 > len(data) {
				break
			}
			val := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			switch prop.propertyID {
			case PropertyIDAttributes:
				r.Attributes = val
			default:
				r.Properties[prop.propertyID] = val
			}

		case propValueTypeUint64:
			if offset+8 > len(data) {
				break
			}
			val := binary.LittleEndian.Uint64(data[offset:])
			offset += 8

			switch prop.propertyID {
			case PropertyIDSize:
				r.Size = val
			case PropertyIDDateModified:
				r.DateModified = filetimeToTime(val)
			case PropertyIDDateCreated:
				r.DateCreated = filetimeToTime(val)
			case PropertyIDDateAccessed:
				r.DateAccessed = filetimeToTime(val)
			default:
				r.Properties[prop.propertyID] = val
			}

		case propValueTypeSizeT:
			if offset+sizeTLen > len(data) {
				break
			}
			var val uint64
			if sizeTLen == 8 {
				val = binary.LittleEndian.Uint64(data[offset:])
			} else {
				val = uint64(binary.LittleEndian.Uint32(data[offset:]))
			}
			offset += sizeTLen
			r.Properties[prop.propertyID] = val

		case propValueTypeBlob8, propValueTypeBlob16:
			// VLQ length + blob data
			blobLen, vlqLen := decodeVLQ(data[offset:])
			offset += vlqLen
			if offset+int(blobLen) > len(data) {
				break
			}
			blobData := make([]byte, blobLen)
			copy(blobData, data[offset:offset+int(blobLen)])
			offset += int(blobLen)

			// Store hash values
			hash := Hash{Valid: blobLen > 0, Value: blobData}
			switch prop.propertyID {
			case PropertyIDCRC32:
				r.CRC32 = hash
			case PropertyIDCRC64:
				r.CRC64 = hash
			case PropertyIDMD5:
				r.MD5 = hash
			case PropertyIDSHA1:
				r.SHA1 = hash
			case PropertyIDSHA256:
				r.SHA256 = hash
			case PropertyIDSHA384:
				r.SHA384 = hash
			case PropertyIDSHA512:
				r.SHA512 = hash
			default:
				r.Properties[prop.propertyID] = blobData
			}

		case propValueTypeUint128:
			if offset+16 > len(data) {
				break
			}
			// Read 16 bytes
			val := make([]byte, 16)
			copy(val, data[offset:offset+16])
			offset += 16
			r.Properties[prop.propertyID] = val

		case propValueTypeDimensions:
			if offset+8 > len(data) {
				break
			}
			// Dimensions is width + height (2 DWORDs)
			width := binary.LittleEndian.Uint32(data[offset:])
			height := binary.LittleEndian.Uint32(data[offset+4:])
			offset += 8
			r.Properties[prop.propertyID] = [2]uint32{width, height}

		case propValueTypeInt32FixedQ1K, propValueTypeInt32FixedQ1M:
			if offset+4 > len(data) {
				break
			}
			val := int32(binary.LittleEndian.Uint32(data[offset:]))
			offset += 4
			r.Properties[prop.propertyID] = val

		default:
			// Unknown type, try to skip
			// This is a best-effort approach
		}
	}

	// Build full path
	if r.Path != "" && r.Name != "" {
		r.FullPath = filepath.Join(r.Path, r.Name)
	} else if r.Name != "" {
		r.FullPath = r.Name
	}

	return r, offset, nil
}

// readUTF8PString reads a length-prefixed UTF-8 string
func (c *Client) readUTF8PString(data []byte) (string, int) {
	if len(data) == 0 {
		return "", 0
	}

	// Read length (VLQ encoded)
	strLen, vlqLen := decodeVLQ(data)
	if vlqLen == 0 || strLen == 0 {
		return "", vlqLen
	}

	if len(data) < vlqLen+int(strLen) {
		return "", vlqLen
	}

	return string(data[vlqLen : vlqLen+int(strLen)]), vlqLen + int(strLen)
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
	// Encode path as UTF-16LE with null terminator
	pathUTF16 := utf16.Encode([]rune(path))
	pathBytes := make([]byte, (len(pathUTF16)+1)*2)
	for i, r := range pathUTF16 {
		binary.LittleEndian.PutUint16(pathBytes[i*2:], r)
	}
	// Null terminator
	binary.LittleEndian.PutUint16(pathBytes[len(pathUTF16)*2:], 0)

	respCode, respData, err := c.sendCommand(cmdGetFileAttributes, pathBytes)
	if err != nil {
		return 0, err
	}

	if respCode != respOK {
		return 0, fmt.Errorf("get file attributes failed with response code %d", respCode)
	}

	if len(respData) < 4 {
		return 0, errors.New("invalid response")
	}

	return binary.LittleEndian.Uint32(respData), nil
}

// GetFileInfo returns detailed file information for the specified file
func (c *Client) GetFileInfo(path string) (*FileInfo, error) {
	// Encode path as UTF-16LE with null terminator
	pathUTF16 := utf16.Encode([]rune(path))
	pathBytes := make([]byte, (len(pathUTF16)+1)*2)
	for i, r := range pathUTF16 {
		binary.LittleEndian.PutUint16(pathBytes[i*2:], r)
	}
	binary.LittleEndian.PutUint16(pathBytes[len(pathUTF16)*2:], 0)

	respCode, respData, err := c.sendCommand(cmdGetFileAttributesEx, pathBytes)
	if err != nil {
		return nil, err
	}

	if respCode != respOK {
		return nil, fmt.Errorf("get file info failed with response code %d", respCode)
	}

	if len(respData) < 36 {
		return nil, errors.New("invalid response")
	}

	fi := &FileInfo{}
	offset := 0

	// Parse WIN32_FIND_DATA-like structure
	fi.Attributes = binary.LittleEndian.Uint32(respData[offset:])
	offset += 4

	fi.CreationTime = filetimeToTime(binary.LittleEndian.Uint64(respData[offset:]))
	offset += 8

	fi.LastAccessTime = filetimeToTime(binary.LittleEndian.Uint64(respData[offset:]))
	offset += 8

	fi.LastWriteTime = filetimeToTime(binary.LittleEndian.Uint64(respData[offset:]))
	offset += 8

	sizeHigh := binary.LittleEndian.Uint32(respData[offset:])
	offset += 4
	sizeLow := binary.LittleEndian.Uint32(respData[offset:])
	offset += 4
	fi.Size = uint64(sizeHigh)<<32 | uint64(sizeLow)

	// Read filename if available
	if offset < len(respData) {
		fi.Name = c.readUTF16NullTerminated(respData[offset:])
	}

	return fi, nil
}

// readUTF16NullTerminated reads a null-terminated UTF-16LE string
func (c *Client) readUTF16NullTerminated(data []byte) string {
	var chars []uint16
	for i := 0; i+1 < len(data); i += 2 {
		ch := binary.LittleEndian.Uint16(data[i:])
		if ch == 0 {
			break
		}
		chars = append(chars, ch)
	}
	return string(utf16.Decode(chars))
}

// FindHandle represents a file search iterator
type FindHandle struct {
	client   *Client
	handleID uint64
}

// FindFirstFile starts a file search for the specified pattern
func (c *Client) FindFirstFile(pattern string) (*FindHandle, *FileInfo, error) {
	// Encode pattern as UTF-16LE with null terminator
	patternUTF16 := utf16.Encode([]rune(pattern))
	patternBytes := make([]byte, (len(patternUTF16)+1)*2)
	for i, r := range patternUTF16 {
		binary.LittleEndian.PutUint16(patternBytes[i*2:], r)
	}
	binary.LittleEndian.PutUint16(patternBytes[len(patternUTF16)*2:], 0)

	respCode, respData, err := c.sendCommand(cmdFindFirstFile, patternBytes)
	if err != nil {
		return nil, nil, err
	}

	if respCode != respOK {
		return nil, nil, fmt.Errorf("find first file failed with response code %d", respCode)
	}

	if len(respData) < 8 {
		return nil, nil, errors.New("invalid response")
	}

	handleID := binary.LittleEndian.Uint64(respData[0:8])
	if handleID == 0 {
		return nil, nil, errors.New("no files found")
	}

	handle := &FindHandle{
		client:   c,
		handleID: handleID,
	}

	fi, err := c.parseFindData(respData[8:])
	if err != nil {
		return nil, nil, err
	}

	return handle, fi, nil
}

// Next returns the next file in the search
func (h *FindHandle) Next() (*FileInfo, error) {
	// Note: FindNextFile is not a direct IPC command in SDK3
	// The SDK uses a streaming approach where results come in chunks
	// For simplicity, we return EOF here - the proper implementation
	// would need to track the search state and call GetResults
	return nil, io.EOF
}

// Close closes the find handle
func (h *FindHandle) Close() error {
	// No explicit close command needed for the search handle
	return nil
}

// parseFindData parses file information from response data
func (c *Client) parseFindData(data []byte) (*FileInfo, error) {
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
		fi.Name = c.readUTF16NullTerminated(data[offset:])
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

// GetFileHash returns the hash value for a file
// This uses the search functionality to get hash properties
func (c *Client) GetFileHash(path string, propertyID uint32) (Hash, error) {
	// Use search to find the exact file and get its hash
	opts := &SearchOptions{
		Text:  "\"" + path + "\"",
		Count: 1,
		RequestedProperties: []uint32{
			PropertyIDName,
			PropertyIDPath,
			propertyID,
		},
	}

	results, err := c.Search(opts)
	if err != nil {
		return Hash{}, err
	}

	if len(results.Results) == 0 {
		return Hash{}, errors.New("file not found")
	}

	// Check if the hash property was returned
	result := results.Results[0]
	switch propertyID {
	case PropertyIDCRC32:
		return result.CRC32, nil
	case PropertyIDCRC64:
		return result.CRC64, nil
	case PropertyIDMD5:
		return result.MD5, nil
	case PropertyIDSHA1:
		return result.SHA1, nil
	case PropertyIDSHA256:
		return result.SHA256, nil
	case PropertyIDSHA384:
		return result.SHA384, nil
	case PropertyIDSHA512:
		return result.SHA512, nil
	}

	return Hash{}, errors.New("hash not available")
}

// GetFileCRC32 returns the CRC32 checksum for a file
func (c *Client) GetFileCRC32(path string) (uint32, error) {
	hash, err := c.GetFileHash(path, PropertyIDCRC32)
	if err != nil {
		return 0, err
	}
	if !hash.Valid {
		return 0, errors.New("CRC32 not available")
	}
	return hash.CRC32(), nil
}

// GetFileCRC64 returns the CRC64 checksum for a file
func (c *Client) GetFileCRC64(path string) (uint64, error) {
	hash, err := c.GetFileHash(path, PropertyIDCRC64)
	if err != nil {
		return 0, err
	}
	if !hash.Valid {
		return 0, errors.New("CRC64 not available")
	}
	return hash.CRC64(), nil
}

// GetFileMD5 returns the MD5 hash for a file as a hex string
func (c *Client) GetFileMD5(path string) (string, error) {
	hash, err := c.GetFileHash(path, PropertyIDMD5)
	if err != nil {
		return "", err
	}
	if !hash.Valid {
		return "", errors.New("MD5 not available")
	}
	return hash.String(), nil
}

// GetFileSHA1 returns the SHA1 hash for a file as a hex string
func (c *Client) GetFileSHA1(path string) (string, error) {
	hash, err := c.GetFileHash(path, PropertyIDSHA1)
	if err != nil {
		return "", err
	}
	if !hash.Valid {
		return "", errors.New("SHA1 not available")
	}
	return hash.String(), nil
}

// GetFileSHA256 returns the SHA256 hash for a file as a hex string
func (c *Client) GetFileSHA256(path string) (string, error) {
	hash, err := c.GetFileHash(path, PropertyIDSHA256)
	if err != nil {
		return "", err
	}
	if !hash.Valid {
		return "", errors.New("SHA256 not available")
	}
	return hash.String(), nil
}

// GetFileSHA512 returns the SHA512 hash for a file as a hex string
func (c *Client) GetFileSHA512(path string) (string, error) {
	hash, err := c.GetFileHash(path, PropertyIDSHA512)
	if err != nil {
		return "", err
	}
	if !hash.Valid {
		return "", errors.New("SHA512 not available")
	}
	return hash.String(), nil
}

// GetFileHashes returns multiple hash values for a file
func (c *Client) GetFileHashes(path string) (*FileHashes, error) {
	hashes := &FileHashes{}

	// Try to get each hash type (errors are ignored for individual hashes)
	hashes.CRC32, _ = c.GetFileHash(path, PropertyIDCRC32)
	hashes.CRC64, _ = c.GetFileHash(path, PropertyIDCRC64)
	hashes.MD5, _ = c.GetFileHash(path, PropertyIDMD5)
	hashes.SHA1, _ = c.GetFileHash(path, PropertyIDSHA1)
	hashes.SHA256, _ = c.GetFileHash(path, PropertyIDSHA256)
	hashes.SHA384, _ = c.GetFileHash(path, PropertyIDSHA384)
	hashes.SHA512, _ = c.GetFileHash(path, PropertyIDSHA512)

	// Check if at least one hash was retrieved
	if !hashes.CRC32.Valid && !hashes.MD5.Valid && !hashes.SHA1.Valid && !hashes.SHA256.Valid {
		return nil, errors.New("no hashes available for this file")
	}

	return hashes, nil
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
	opts := &SearchOptions{
		Text:  "\"" + path + "\"",
		Count: 1,
		RequestedProperties: []uint32{
			PropertyIDName,
			PropertyIDPath,
			propertyID,
		},
	}

	results, err := c.Search(opts)
	if err != nil {
		return "", err
	}

	if len(results.Results) == 0 {
		return "", errors.New("file not found")
	}

	result := results.Results[0]
	if val, ok := result.Properties[propertyID]; ok {
		if s, ok := val.(string); ok {
			return s, nil
		}
	}

	return "", errors.New("property not found")
}

// GetPropertyUint64 returns a uint64 property value for a file
func (c *Client) GetPropertyUint64(path string, propertyID uint32) (uint64, error) {
	opts := &SearchOptions{
		Text:  "\"" + path + "\"",
		Count: 1,
		RequestedProperties: []uint32{
			PropertyIDName,
			PropertyIDPath,
			propertyID,
		},
	}

	results, err := c.Search(opts)
	if err != nil {
		return 0, err
	}

	if len(results.Results) == 0 {
		return 0, errors.New("file not found")
	}

	result := results.Results[0]
	if val, ok := result.Properties[propertyID]; ok {
		if u, ok := val.(uint64); ok {
			return u, nil
		}
	}

	// Special case for Size
	if propertyID == PropertyIDSize {
		return result.Size, nil
	}

	return 0, errors.New("property not found")
}

// GetPropertyUint32 returns a uint32 property value for a file
func (c *Client) GetPropertyUint32(path string, propertyID uint32) (uint32, error) {
	val, err := c.GetPropertyUint64(path, propertyID)
	if err != nil {
		return 0, err
	}
	return uint32(val), nil
}

// Platform check
func init() {
	// This package only works on Windows
	_ = unsafe.Sizeof(windows.Handle(0))
}
