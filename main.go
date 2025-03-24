package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// Execution path enum represents different file serving strategies
type ExecutionPath string

const (
	// NOBUFFER serves files with no additional buffering
	NOBUFFER ExecutionPath = "NOBUFFER"
	
	// BUFIOREADER uses bufio.Reader for efficient reading
	BUFIOREADER ExecutionPath = "BUFIOREADER"
	
	// PRELOAD loads the entire file into memory before serving
	PRELOAD ExecutionPath = "PRELOAD"
)

var (
	sourceFilePath  string        // Path to file that will be served
	port            int           // HTTP server port
	bufferSizeBytes int           // Size of buffer for BUFIOREADER mode
	executionPath   string        // Which execution strategy to use
	preloadedData   []byte        // In-memory storage for PRELOAD mode
	server          *http.Server  // Global server reference for shutdown
	shutdownChan    chan struct{} // Channel for signaling server shutdown
	
	// System call stats
	readCalls  int // Count of read operations
	writeCalls int // Count of write operations
)

// logMemUsage outputs the current memory statistics
func logMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("MEMORY: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
	log.Printf("MEMORY: HeapInUse = %v MiB, MaxHeap = %v MiB", 
		bToMb(m.HeapInuse), bToMb(m.HeapSys))
}

// bToMb converts bytes to megabytes for more readable output
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

// CountingReader wraps an io.Reader and counts read operations
type CountingReader struct {
	reader io.Reader
}

func (cr *CountingReader) Read(p []byte) (n int, err error) {
	n, err = cr.reader.Read(p)
	if n > 0 {
		// Only count successful reads
		incrementReadCalls()
	}
	return n, err
}

// CountingWriter wraps an io.Writer and counts write operations
type CountingWriter struct {
	writer io.Writer
}

func (cw *CountingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.writer.Write(p)
	if n > 0 {
		// Only count successful writes
		incrementWriteCalls()
	}
	return n, err
}

// incrementReadCalls atomically increments the read calls counter
func incrementReadCalls() {
	readCalls++
}

// incrementWriteCalls atomically increments the write calls counter
func incrementWriteCalls() {
	writeCalls++
}

// logSyscalls outputs the current system call statistics
func logSyscalls() {
	log.Printf("SYSCALLS: Read = %d, Write = %d, Total = %d",
		readCalls, writeCalls, readCalls+writeCalls)
}

// setupGracefulShutdown configures signal handling for proper server termination
func setupGracefulShutdown() {
	// Initialize the shutdown channel
	shutdownChan = make(chan struct{})
	
	// Create a channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	
	// Register for SIGINT (Ctrl+C), SIGTERM, and SIGHUP
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	
	// Launch a goroutine to handle the signals
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v, initiating graceful shutdown", sig)
		
		// Create a timeout context for the shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Attempt graceful shutdown of the server
		if server != nil {
			if err := server.Shutdown(ctx); err != nil {
				log.Printf("Error during server shutdown: %v", err)
			}
		} else {
			// If server doesn't exist, just close the channel
			close(shutdownChan)
		}
		
		// Log exit
		log.Printf("Server shutdown completed. Exiting.")
	}()
}

func main() {
	// Parse and validate command line arguments
	flag.StringVar(&sourceFilePath, "source", "", "Source file path")
	flag.IntVar(&port, "port", 8080, "HTTP server port")
	flag.IntVar(&bufferSizeBytes, "buffer", 4096, "Buffer size in bytes (only relevant for BUFIOREADER)")
	flag.StringVar(&executionPath, "exec", string(NOBUFFER), "Execution path: NOBUFFER, BUFIOREADER, PRELOAD")
	flag.Parse()
	
	// Reset system call counters for this run
	readCalls = 0
	writeCalls = 0

	if sourceFilePath == "" {
		log.Fatalf("Source file path is required")
	}
	
	// Set up graceful shutdown handling
	setupGracefulShutdown()

	// Check if file exists and is readable
	fileInfo, err := os.Stat(sourceFilePath)
	if err != nil {
		log.Fatalf("Cannot access source file: %v", err)
	}
	log.Printf("File size: %d bytes", fileInfo.Size())

	// Log initial memory stats for comparison
	log.Println("Initial memory statistics:")
	logMemUsage()

	// For PRELOAD mode, load the entire file into memory once
	if ExecutionPath(executionPath) == PRELOAD {
		startTime := time.Now()
		
		// Open the file for reading and count system calls
		file, err := os.Open(sourceFilePath)
		if err != nil {
			log.Fatalf("Failed to open file for preloading: %v", err)
		}
		defer file.Close()
		
		// Create a counting reader
		countingReader := &CountingReader{reader: file}
		
		// Read the entire file into memory at once
		// This is efficient for subsequent requests but uses more memory
		preloadedData, err = io.ReadAll(countingReader)
		if err != nil {
			log.Fatalf("Failed to preload file: %v", err)
		}
		
		duration := time.Since(startTime)
		log.Printf("TIMING: Preloaded %d bytes in %v (%.2f MB/s)", 
			len(preloadedData), 
			duration,
			float64(len(preloadedData))/duration.Seconds()/1024/1024)
		
		// Log memory after preload to show memory impact
		log.Println("Memory statistics after preloading:")
		logMemUsage()
	}

	// Set up HTTP server with a single handler for the root path
	http.HandleFunc("/", handleRequest)
	serverAddr := fmt.Sprintf(":%d", port)
	
	// Log server configuration
	log.Printf("Starting server on http://localhost%s", serverAddr)
	log.Printf("Execution mode: %s", executionPath)
	if ExecutionPath(executionPath) == BUFIOREADER {
		log.Printf("Buffer size: %d bytes", bufferSizeBytes)
	}
	log.Printf("Serving file: %s", sourceFilePath)
	log.Printf("Use: curl http://localhost%s -o output.file", serverAddr)
	
	// Create a custom server for better shutdown control
	server = &http.Server{
		Addr:    serverAddr,
		Handler: nil, // Use the default ServeMux
	}
	
	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
		log.Printf("Server has shut down")
		
		// Signal that server has shut down
		close(shutdownChan)
	}()
	
	// Block until shutdown signal received
	<-shutdownChan
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	requestStartTime := time.Now()
	log.Printf("Received request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

	/*
	   About http.ResponseWriter and its buffering:
	   
	   The default implementation of http.ResponseWriter in Go's standard library
	   has some built-in buffering behavior:
	   
	   1. Headers are buffered until the first write to the body or until 
	      an explicit call to WriteHeader()
	   
	   2. The exact buffering behavior of the body depends on the specific
	      ResponseWriter implementation, but generally:
	      
	      - Small writes may be buffered to improve efficiency
	      - Large writes are typically sent immediately
	      - The actual buffer size may vary based on Go version and server config
	      
	   3. Go's HTTP server also employs chunked transfer encoding automatically
	      when needed, which affects how data is buffered and sent
	   
	   For our learning purpose, we're focusing on the buffering we explicitly
	   control in our code, but it's good to be aware that the ResponseWriter
	   itself may have some buffering behavior.
	*/

	// Set response headers for file download
	filename := filepath.Base(sourceFilePath)
	
	// Content-Disposition tells browser to download rather than display
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	
	// Set Content-Type for the file
	w.Header().Set("Content-Type", "application/octet-stream")
	
	// If we know the file size, set Content-Length header which helps client
	// and enables HTTP keepalive
	if ExecutionPath(executionPath) != PRELOAD {
		if fileInfo, err := os.Stat(sourceFilePath); err == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		}
	} else {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(preloadedData)))
	}

	// Serve file using the selected execution path
	switch ExecutionPath(executionPath) {
	case NOBUFFER:
		serveWithNoBuffer(w)
	case BUFIOREADER:
		serveWithBufioReader(w)
	case PRELOAD:
		serveWithPreloadedData(w)
	default:
		http.Error(w, "Invalid execution path", http.StatusInternalServerError)
		return
	}
	
	// Log total request processing time
	log.Printf("TIMING: Total request processing time: %v", time.Since(requestStartTime))
	
	// Log system call statistics
	logSyscalls()
}

func serveWithNoBuffer(w http.ResponseWriter) {
	/*
	   NOBUFFER MODE
	   
	   This mode directly connects the file to the ResponseWriter with no 
	   additional buffering in between. Any buffering that occurs is due to:
	   
	   1. OS-level file system caching
	   2. Any internal buffering in http.ResponseWriter implementation
	   3. TCP's own buffering mechanisms
	   
	   System call pattern:
	   - Multiple read() system calls as data is read from file
	   - Multiple write() system calls as data is written to network
	   
	   This provides a baseline for comparison with other methods.
	*/
	
	// Open the file for reading
	startTime := time.Now()
	file, err := os.Open(sourceFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close() // Ensure file is closed when function exits
	
	openDuration := time.Since(startTime)
	log.Printf("TIMING: Opened file in %v", openDuration)

	// Create a counting reader to track read system calls
	countingReader := &CountingReader{reader: file}
	
	// Create a counting writer to track write system calls
	countingWriter := &CountingWriter{writer: w}
	
	// Copy file directly to the HTTP response writer
	// Although we're using no explicit buffer, io.Copy uses a 32KB
	// internal buffer by default when copying
	startTime = time.Now()
	bytesWritten, err := io.Copy(countingWriter, countingReader)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
		return
	}
	
	writeDuration := time.Since(startTime)
	log.Printf("TIMING: Wrote %d bytes with NOBUFFER in %v (%.2f MB/s)", 
		bytesWritten, 
		writeDuration,
		float64(bytesWritten)/writeDuration.Seconds()/1024/1024)
	
	// Log memory usage for comparison
	logMemUsage()
}

func serveWithBufioReader(w http.ResponseWriter) {
	/*
	   BUFIOREADER MODE
	   
	   This mode uses bufio.Reader for reading from the file in larger chunks.
	   Benefits:
	   - Reduces system calls by reading larger chunks at once
	   - Allows peeking and other buffered operations if needed
	   
	   System call pattern:
	   - Fewer read() system calls due to buffering
	   - Same number of write() system calls to network
	   
	   The key performance factor is the buffer size which determines:
	   - How much memory is used per request
	   - How frequently the program needs to make system calls to read more data
	*/
	
	// Open the file for reading
	startTime := time.Now()
	file, err := os.Open(sourceFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	
	openDuration := time.Since(startTime)
	log.Printf("TIMING: Opened file in %v", openDuration)

	// Create a counting reader to track the underlying file reads 
	countingFile := &CountingReader{reader: file}
	
	// Create a buffered reader with specified buffer size
	// The bufio.Reader will read data in chunks of bufferSizeBytes from the file
	startTime = time.Now()
	reader := bufio.NewReaderSize(countingFile, bufferSizeBytes)
	
	// Create a counting writer to track write system calls
	countingWriter := &CountingWriter{writer: w}
	
	// Copy data to response writer using the buffered reader
	// Note: io.Copy has its own internal buffer, but it will use the
	// bufio.Reader's buffer when reading from it, avoiding double buffering
	bytesWritten, err := io.Copy(countingWriter, reader)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
		return
	}
	
	writeDuration := time.Since(startTime)
	log.Printf("TIMING: Read and wrote %d bytes with BUFIOREADER (buffer size: %d) in %v (%.2f MB/s)", 
		bytesWritten, 
		bufferSizeBytes, 
		writeDuration,
		float64(bytesWritten)/writeDuration.Seconds()/1024/1024)
	
	// Log memory usage for comparison
	logMemUsage()
}

func serveWithPreloadedData(w http.ResponseWriter) {
	/*
	   PRELOAD MODE
	   
	   This mode loads the entire file into memory at program startup, so:
	   - No file I/O during request handling
	   - Fast response time for each request
	   - Higher memory usage regardless of request volume
	   
	   System call pattern:
	   - No read() system calls during request handling
	   - Same number of write() system calls to network
	   
	   Best for:
	   - Small to medium files that are requested frequently
	   - When response time is critical
	*/
	
	// Create a counting writer to track write system calls
	countingWriter := &CountingWriter{writer: w}
	
	// Simply write the preloaded data to the response
	// This is the fastest method as the data is already in memory
	startTime := time.Now()
	bytesWritten, err := countingWriter.Write(preloadedData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write data: %v", err), http.StatusInternalServerError)
		return
	}
	
	writeDuration := time.Since(startTime)
	log.Printf("TIMING: Wrote %d bytes with PRELOAD in %v (%.2f MB/s)", 
		bytesWritten, 
		writeDuration,
		float64(bytesWritten)/writeDuration.Seconds()/1024/1024)
	
	// Log memory usage for comparison
	logMemUsage()
}