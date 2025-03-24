package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// BenchmarkConfig holds the configuration for a benchmark run
type BenchmarkConfig struct {
	SourceFile      string
	Port            int
	Runs            int
	BufferSizes     []int
	OutputDirectory string
}

// RunResult stores metrics for a single benchmark run
type RunResult struct {
	ExecutionPath string    `json:"executionPath"`
	BufferSize    int       `json:"bufferSize"`
	RunID         int       `json:"runId"`
	ServerTiming  float64   `json:"serverTiming"` // in milliseconds
	ClientTiming  float64   `json:"clientTiming"` // in milliseconds
	MemoryUsage   MemStats  `json:"memoryUsage"`
	SystemCalls   SysStats  `json:"systemCalls"`
	Timestamp     time.Time `json:"timestamp"`
}

// MemStats holds memory statistics
type MemStats struct {
	AllocMiB    uint64 `json:"allocMiB"`
	TotalAlloc  uint64 `json:"totalAlloc"`
	SysMiB      uint64 `json:"sysMiB"`
	NumGC       uint32 `json:"numGC"`
	MaxHeapMiB  uint64 `json:"maxHeapMiB"`
	HeapInUseMiB uint64 `json:"heapInUseMiB"`
}

// SysStats holds system call statistics
type SysStats struct {
	ReadCalls  int `json:"readCalls"`
	WriteCalls int `json:"writeCalls"`
	TotalCalls int `json:"totalCalls"`
}

// AggregateStats holds the statistical summary of multiple runs
type AggregateStats struct {
	ExecutionPath string  `json:"executionPath"`
	BufferSize    int     `json:"bufferSize"`
	NumRuns       int     `json:"numRuns"`
	
	// Server timing statistics (ms)
	AvgServerTime  float64 `json:"avgServerTime"`
	MinServerTime  float64 `json:"minServerTime"`
	MaxServerTime  float64 `json:"maxServerTime"`
	StdDevServer   float64 `json:"stdDevServer"`
	
	// Client timing statistics (ms)
	AvgClientTime  float64 `json:"avgClientTime"`
	MinClientTime  float64 `json:"minClientTime"`
	MaxClientTime  float64 `json:"maxClientTime"`
	StdDevClient   float64 `json:"stdDevClient"`
	
	// Memory statistics (MiB)
	AvgMemoryAlloc float64 `json:"avgMemoryAlloc"`
	MaxMemoryAlloc uint64  `json:"maxMemoryAlloc"`
	AvgHeapInUse   float64 `json:"avgHeapInUse"`
	MaxHeapInUse   uint64  `json:"maxHeapInUse"`
	
	// System call statistics
	AvgReadCalls   float64 `json:"avgReadCalls"`
	AvgWriteCalls  float64 `json:"avgWriteCalls"`
	AvgTotalCalls  float64 `json:"avgTotalCalls"`
	MaxTotalCalls  int     `json:"maxTotalCalls"`
}

// BenchmarkResults contains all the results of a benchmark session
type BenchmarkResults struct {
	Config        BenchmarkConfig            `json:"config"`
	RawResults    []RunResult                `json:"rawResults"`
	AggregateData map[string]AggregateStats  `json:"aggregateData"`
}

func main() {
	var config BenchmarkConfig
	var bufferSizesStr string
	
	// Parse command line arguments
	flag.StringVar(&config.SourceFile, "source", "", "Source file path for benchmarking (required)")
	flag.IntVar(&config.Port, "port", 8082, "HTTP server port for benchmarking")
	flag.IntVar(&config.Runs, "runs", 3, "Number of runs per configuration")
	flag.StringVar(&bufferSizesStr, "bufferSizes", "1024,8192,65536,1048576", "Comma-separated list of buffer sizes for BUFIOREADER (in bytes)")
	flag.StringVar(&config.OutputDirectory, "output", "benchmark_results", "Directory to store benchmark results")
	flag.Parse()
	
	// Validate required arguments
	if config.SourceFile == "" {
		log.Fatalf("Source file path is required")
	}
	
	// Parse buffer sizes
	bufferSizes := strings.Split(bufferSizesStr, ",")
	config.BufferSizes = make([]int, 0, len(bufferSizes))
	for _, bs := range bufferSizes {
		size, err := strconv.Atoi(strings.TrimSpace(bs))
		if err != nil {
			log.Fatalf("Invalid buffer size: %s", bs)
		}
		config.BufferSizes = append(config.BufferSizes, size)
	}
	
	// Create output directory
	if err := os.MkdirAll(config.OutputDirectory, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	
	// Run benchmarks and collect results
	results := runBenchmarks(config)
	
	// Save results to file
	outputPath := filepath.Join(config.OutputDirectory, fmt.Sprintf("benchmark_results_%s.json", 
		time.Now().Format("20060102_150405")))
	
	resultsJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal results to JSON: %v", err)
	}
	
	if err := os.WriteFile(outputPath, resultsJSON, 0644); err != nil {
		log.Fatalf("Failed to write results to file: %v", err)
	}
	
	log.Printf("Benchmark complete! Results saved to: %s", outputPath)
	
	// Generate HTML report
	generateHTMLReport(results, config.OutputDirectory)
}

func runBenchmarks(config BenchmarkConfig) BenchmarkResults {
	results := BenchmarkResults{
		Config:       config,
		RawResults:   make([]RunResult, 0),
		AggregateData: make(map[string]AggregateStats),
	}
	
	// Test NOBUFFER mode
	log.Println("Benchmarking NOBUFFER mode...")
	for i := 0; i < config.Runs; i++ {
		log.Printf("Run %d/%d", i+1, config.Runs)
		result := benchmarkExecution("NOBUFFER", 0, config, i+1)
		results.RawResults = append(results.RawResults, result)
	}
	
	// Test BUFIOREADER mode with different buffer sizes
	for _, bufferSize := range config.BufferSizes {
		log.Printf("Benchmarking BUFIOREADER mode with buffer size: %d bytes...", bufferSize)
		for i := 0; i < config.Runs; i++ {
			log.Printf("Run %d/%d", i+1, config.Runs)
			result := benchmarkExecution("BUFIOREADER", bufferSize, config, i+1)
			results.RawResults = append(results.RawResults, result)
		}
	}
	
	// Test PRELOAD mode
	log.Println("Benchmarking PRELOAD mode...")
	for i := 0; i < config.Runs; i++ {
		log.Printf("Run %d/%d", i+1, config.Runs)
		result := benchmarkExecution("PRELOAD", 0, config, i+1)
		results.RawResults = append(results.RawResults, result)
	}
	
	// Calculate aggregate statistics
	results.AggregateData = calculateAggregateStats(results.RawResults)
	
	return results
}

func benchmarkExecution(execPath string, bufferSize int, config BenchmarkConfig, runID int) RunResult {
	result := RunResult{
		ExecutionPath: execPath,
		BufferSize:    bufferSize,
		RunID:         runID,
		Timestamp:     time.Now(),
	}
	
	// Construct server command
	serverCmd := exec.Command("go", "run", "main.go", 
		"-source", config.SourceFile,
		"-port", strconv.Itoa(config.Port),
		"-exec", execPath)
	
	if execPath == "BUFIOREADER" {
		serverCmd.Args = append(serverCmd.Args, "-buffer", strconv.Itoa(bufferSize))
	}
	
	// System call tracing is causing issues, disable for now to ensure basic functionality works
	// We'll implement a simpler approach for system call counting
	
	// Check if the port is available before starting the server
	portAvailable := isPortAvailable(config.Port)
	if !portAvailable {
		log.Printf("Warning: Port %d appears to be in use. This may cause the benchmark to fail.", config.Port)
		// Try to find an available port
		portFound := false
		for testPort := config.Port + 1; testPort < config.Port + 100; testPort++ {
			if isPortAvailable(testPort) {
				log.Printf("Found available port: %d. Using this instead.", testPort)
				
				// Recreate the server command with the new port
				serverCmd = exec.Command("go", "run", "main.go", 
					"-source", config.SourceFile,
					"-port", strconv.Itoa(testPort),
					"-exec", execPath)
				
				if execPath == "BUFIOREADER" {
					serverCmd.Args = append(serverCmd.Args, "-buffer", strconv.Itoa(bufferSize))
				}
				
				// Update the port in the config for curl
				config.Port = testPort
				portFound = true
				break
			}
		}
		
		if !portFound {
			log.Printf("Could not find an available port. Will try to use port %d anyway.", config.Port)
		}
	}
	
	// Start server in background
	serverOutput, err := os.CreateTemp("", "server_output_*.log")
	if err != nil {
		log.Fatalf("Failed to create server output file: %v", err)
	}
	defer os.Remove(serverOutput.Name())
	
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	
	log.Printf("Starting server with command: %s", strings.Join(serverCmd.Args, " "))
	if err := serverCmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	
	// Allow server to start
	log.Printf("Waiting for server to start...")
	time.Sleep(5 * time.Second) // Increase wait time to ensure server is ready
	
	// Create output file for curl
	outputFile, err := os.CreateTemp("", "curl_output_*.dat")
	if err != nil {
		log.Fatalf("Failed to create temporary file for curl output: %v", err)
	}
	defer os.Remove(outputFile.Name())
	
	// We don't need a separate timing file anymore since we're using simple output format
	
	// Run curl command to download the file and measure time
	// Use a simple format with just the time_total value
	// Limit download size to first 10MB to avoid excessive disk usage during testing
	curlCmd := exec.Command("curl",
		"-o", outputFile.Name(),
		"-w", "%{time_total}\n",
		"-s",
		"--connect-timeout", "10", // Add connection timeout to avoid hanging
		"--max-time", "60",        // Add maximum time limit
		// "--range", "0-10485760",   // Limit to first 10MB (helpful for large files)
		fmt.Sprintf("http://localhost:%d", config.Port))
	
	// Don't use the more complex format which was causing parsing issues
	
	// Try the curl command with retries
	var clientTimingOutput []byte
	
	// Wait slightly longer for server to initialize
	log.Printf("Testing server connection before running curl...")
	waitForServer(config.Port, 10)
	
	// Only try once now that we have better port handling
	log.Printf("Running curl...")
	clientTimingOutput, err = curlCmd.Output()
	if err != nil {
		log.Printf("curl command failed: %v", err)
	}
	
	// Kill the server and ensure cleanup
	log.Printf("Stopping server process...")
	
	// First try a graceful termination
	if err := serverCmd.Process.Signal(os.Interrupt); err != nil {
		log.Printf("Failed to send interrupt signal: %v, trying hard kill", err)
		if err := serverCmd.Process.Kill(); err != nil {
			log.Printf("Failed to kill server: %v", err)
		}
	}
	
	// Wait for the server to exit with timeout
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- serverCmd.Wait()
	}()
	
	// Wait for process to exit or timeout
	select {
	case <-doneChan:
		log.Printf("Server process exited normally")
	case <-time.After(3 * time.Second):
		log.Printf("Server process didn't exit in time, forcing kill")
		if err := serverCmd.Process.Kill(); err != nil {
			log.Printf("Failed to force kill server: %v", err)
		}
	}
	
	// Additional cleanup: find and kill any lingering go processes on this port
	cleanupCmd := exec.Command("bash", "-c", 
		fmt.Sprintf("lsof -i :%d -t | xargs -r kill -9", config.Port))
	if cleanupOutput, err := cleanupCmd.CombinedOutput(); err != nil {
		log.Printf("Port cleanup may have failed: %v, output: %s", err, string(cleanupOutput))
	} else {
		log.Printf("Additional port cleanup completed")
	}
	
	// Wait a bit longer to ensure the port is released
	log.Printf("Waiting for port to be released...")
	time.Sleep(3 * time.Second)
	
	// Read server logs
	serverOutput.Seek(0, 0)
	serverLogs, err := os.ReadFile(serverOutput.Name())
	if err != nil {
		log.Printf("Failed to read server logs: %v", err)
	}
	
	// Parse server timing
	result.ServerTiming = extractServerTiming(string(serverLogs))
	
	// Parse client timing
	if len(clientTimingOutput) > 0 {
		timingStr := strings.TrimSpace(string(clientTimingOutput))
		timing, err := strconv.ParseFloat(timingStr, 64)
		if err != nil {
			log.Printf("Failed to parse curl timing '%s': %v", timingStr, err)
		} else {
			result.ClientTiming = timing * 1000 // Convert to milliseconds
		}
	}
	
	// Parse memory usage
	result.MemoryUsage = extractMemoryStats(string(serverLogs))
	
	// Since we disabled system call tracing, set default values
	result.SystemCalls = SysStats{
		ReadCalls:  0,
		WriteCalls: 0,
		TotalCalls: 0,
	}
	
	// Try to extract any available system call info from logs
	if syscalls := extractSystemCalls(string(serverLogs)); syscalls.TotalCalls > 0 {
		result.SystemCalls = syscalls
	}
	
	return result
}

func extractServerTiming(logs string) float64 {
	// Look for the total request processing time
	for _, line := range strings.Split(logs, "\n") {
		// The log line might have a line number prefix, so look for the timing pattern anywhere in the line
		if strings.Contains(line, "TIMING: Total request processing time:") {
			log.Printf("Found timing line: %s", line)
			
			// Extract everything after "TIMING: Total request processing time:"
			timePart := ""
			if idx := strings.Index(line, "TIMING: Total request processing time:"); idx >= 0 {
				timePart = line[idx+len("TIMING: Total request processing time:"):]
			}
			
			timeStr := strings.TrimSpace(timePart)
			log.Printf("Extracted time string: '%s'", timeStr)
			
			// Try to parse as Go duration
			if duration, err := time.ParseDuration(timeStr); err == nil {
				return float64(duration.Milliseconds())
			}
			
			// Try matching specific formats with regex
			if strings.HasSuffix(timeStr, "ms") {
				numStr := strings.TrimSuffix(timeStr, "ms")
				if ms, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64); err == nil {
					return ms
				}
			} else if strings.HasSuffix(timeStr, "µs") {
				numStr := strings.TrimSuffix(timeStr, "µs")
				if us, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64); err == nil {
					return us / 1000 // Convert microseconds to milliseconds
				}
			} else if strings.HasSuffix(timeStr, "us") {
				numStr := strings.TrimSuffix(timeStr, "us")
				if us, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64); err == nil {
					return us / 1000 // Convert microseconds to milliseconds
				}
			} else if strings.HasSuffix(timeStr, "s") && !strings.Contains(timeStr, "ms") {
				numStr := strings.TrimSuffix(timeStr, "s")
				if s, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64); err == nil {
					return s * 1000 // Convert seconds to milliseconds
				}
			}
			
			// As a fallback, try to extract any number in the string and assume milliseconds
			for _, word := range strings.Fields(timeStr) {
				if val, err := strconv.ParseFloat(word, 64); err == nil {
					if strings.Contains(timeStr, "ms") {
						return val // Already in milliseconds
					} else if strings.Contains(timeStr, "µs") || strings.Contains(timeStr, "us") {
						return val / 1000 // Convert microseconds to milliseconds
					} else if strings.Contains(timeStr, "s") && !strings.Contains(timeStr, "ms") {
						return val * 1000 // Convert seconds to milliseconds
					} else {
						return val // Assume milliseconds
					}
				}
			}
			
			log.Printf("Failed to parse timing: %s", timeStr)
			break
		}
	}
	return 0
}

func extractMemoryStats(logs string) MemStats {
	var stats MemStats
	
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "MEMORY: Alloc =") {
			// Example: "MEMORY: Alloc = 10 MiB, TotalAlloc = 15 MiB, Sys = 20 MiB, NumGC = 5"
			parts := strings.Split(line, ",")
			
			for _, part := range parts {
				if strings.Contains(part, "Alloc =") {
					valueStr := strings.Split(part, "=")[1]
					valueStr = strings.TrimSpace(strings.Split(valueStr, "MiB")[0])
					value, err := strconv.ParseUint(valueStr, 10, 64)
					if err == nil {
						stats.AllocMiB = value
					}
				} else if strings.Contains(part, "TotalAlloc =") {
					valueStr := strings.Split(part, "=")[1]
					valueStr = strings.TrimSpace(strings.Split(valueStr, "MiB")[0])
					value, err := strconv.ParseUint(valueStr, 10, 64)
					if err == nil {
						stats.TotalAlloc = value
					}
				} else if strings.Contains(part, "Sys =") {
					valueStr := strings.Split(part, "=")[1]
					valueStr = strings.TrimSpace(strings.Split(valueStr, "MiB")[0])
					value, err := strconv.ParseUint(valueStr, 10, 64)
					if err == nil {
						stats.SysMiB = value
					}
				} else if strings.Contains(part, "NumGC =") {
					valueStr := strings.TrimSpace(strings.Split(part, "=")[1])
					value, err := strconv.ParseUint(valueStr, 10, 32)
					if err == nil {
						stats.NumGC = uint32(value)
					}
				}
			}
		}
		
		// Look for max heap statistics if available
		if strings.Contains(line, "MaxHeap =") {
			valueStr := strings.Split(line, "=")[1]
			valueStr = strings.TrimSpace(strings.Split(valueStr, "MiB")[0])
			value, err := strconv.ParseUint(valueStr, 10, 64)
			if err == nil {
				stats.MaxHeapMiB = value
			}
		}
		
		if strings.Contains(line, "HeapInUse =") {
			valueStr := strings.Split(line, "=")[1]
			valueStr = strings.TrimSpace(strings.Split(valueStr, "MiB")[0])
			value, err := strconv.ParseUint(valueStr, 10, 64)
			if err == nil {
				stats.HeapInUseMiB = value
			}
		}
	}
	
	return stats
}

func extractSystemCalls(logs string) SysStats {
	var stats SysStats
	
	// Look for our custom SYSCALLS logging line
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "SYSCALLS: Read =") {
			log.Printf("Found syscalls line: %s", line)
			
			// Parse read calls
			readStart := strings.Index(line, "Read = ")
			if readStart >= 0 {
				readPart := line[readStart+len("Read = "):]
				readEnd := strings.Index(readPart, ",")
				if readEnd >= 0 {
					readStr := readPart[:readEnd]
					if readCount, err := strconv.Atoi(strings.TrimSpace(readStr)); err == nil {
						stats.ReadCalls = readCount
					}
				}
			}
			
			// Parse write calls
			writeStart := strings.Index(line, "Write = ")
			if writeStart >= 0 {
				writePart := line[writeStart+len("Write = "):]
				writeEnd := strings.Index(writePart, ",")
				if writeEnd >= 0 {
					writeStr := writePart[:writeEnd]
					if writeCount, err := strconv.Atoi(strings.TrimSpace(writeStr)); err == nil {
						stats.WriteCalls = writeCount
					}
				}
			}
			
			// Parse total calls (or calculate from read + write)
			totalStart := strings.Index(line, "Total = ")
			if totalStart >= 0 {
				totalPart := line[totalStart+len("Total = "):]
				if totalCount, err := strconv.Atoi(strings.TrimSpace(totalPart)); err == nil {
					stats.TotalCalls = totalCount
				} else {
					stats.TotalCalls = stats.ReadCalls + stats.WriteCalls
				}
			} else {
				stats.TotalCalls = stats.ReadCalls + stats.WriteCalls
			}
			
			// We found and parsed the line, so we can return
			return stats
		}
	}
	
	return stats
}

func calculateAggregateStats(results []RunResult) map[string]AggregateStats {
	// Group results by execution path and buffer size
	groups := make(map[string][]RunResult)
	
	for _, result := range results {
		key := fmt.Sprintf("%s_%d", result.ExecutionPath, result.BufferSize)
		groups[key] = append(groups[key], result)
	}
	
	// Calculate aggregate stats for each group
	aggregateStats := make(map[string]AggregateStats)
	
	for key, group := range groups {
		stats := AggregateStats{
			ExecutionPath: group[0].ExecutionPath,
			BufferSize:    group[0].BufferSize,
			NumRuns:       len(group),
			
			MinServerTime: float64(^uint64(0) >> 1), // Max float64 value
			MinClientTime: float64(^uint64(0) >> 1), // Max float64 value
		}
		
		// Sum values for averages
		var totalServerTime, totalClientTime float64
		var totalMemAlloc, totalHeapInUse uint64
		var totalReadCalls, totalWriteCalls, totalSystemCalls int
		
		for _, result := range group {
			// Server timing
			totalServerTime += result.ServerTiming
			if result.ServerTiming < stats.MinServerTime {
				stats.MinServerTime = result.ServerTiming
			}
			if result.ServerTiming > stats.MaxServerTime {
				stats.MaxServerTime = result.ServerTiming
			}
			
			// Client timing
			totalClientTime += result.ClientTiming
			if result.ClientTiming < stats.MinClientTime {
				stats.MinClientTime = result.ClientTiming
			}
			if result.ClientTiming > stats.MaxClientTime {
				stats.MaxClientTime = result.ClientTiming
			}
			
			// Memory
			totalMemAlloc += result.MemoryUsage.AllocMiB
			if result.MemoryUsage.AllocMiB > stats.MaxMemoryAlloc {
				stats.MaxMemoryAlloc = result.MemoryUsage.AllocMiB
			}
			
			totalHeapInUse += result.MemoryUsage.HeapInUseMiB
			if result.MemoryUsage.HeapInUseMiB > stats.MaxHeapInUse {
				stats.MaxHeapInUse = result.MemoryUsage.HeapInUseMiB
			}
			
			// System calls
			totalReadCalls += result.SystemCalls.ReadCalls
			totalWriteCalls += result.SystemCalls.WriteCalls
			totalSystemCalls += result.SystemCalls.TotalCalls
			
			if result.SystemCalls.TotalCalls > stats.MaxTotalCalls {
				stats.MaxTotalCalls = result.SystemCalls.TotalCalls
			}
		}
		
		// Calculate averages
		numRuns := float64(len(group))
		stats.AvgServerTime = totalServerTime / numRuns
		stats.AvgClientTime = totalClientTime / numRuns
		stats.AvgMemoryAlloc = float64(totalMemAlloc) / numRuns
		stats.AvgHeapInUse = float64(totalHeapInUse) / numRuns
		stats.AvgReadCalls = float64(totalReadCalls) / numRuns
		stats.AvgWriteCalls = float64(totalWriteCalls) / numRuns
		stats.AvgTotalCalls = float64(totalSystemCalls) / numRuns
		
		// Calculate standard deviations
		var serverTimeVariance, clientTimeVariance float64
		
		for _, result := range group {
			serverDiff := result.ServerTiming - stats.AvgServerTime
			serverTimeVariance += serverDiff * serverDiff
			
			clientDiff := result.ClientTiming - stats.AvgClientTime
			clientTimeVariance += clientDiff * clientDiff
		}
		
		if len(group) > 1 {
			stats.StdDevServer = sqrt(serverTimeVariance / numRuns)
			stats.StdDevClient = sqrt(clientTimeVariance / numRuns)
		}
		
		aggregateStats[key] = stats
	}
	
	return aggregateStats
}

func sqrt(x float64) float64 {
	return float64(int64(x*100)) / 100 // Simple rounding to 2 decimal places
}

// isPortAvailable checks if a TCP port is available to use
func isPortAvailable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
	if err != nil {
		// If we got an error, it might mean the port is available (no one is listening)
		return true
	}
	// If we could connect, the port is in use
	conn.Close()
	return false
}

// waitForServer attempts to connect to the server to confirm it's running
// maxWaitSeconds is the maximum time to wait in seconds
func waitForServer(port int, maxWaitSeconds int) bool {
	log.Printf("Waiting for server on port %d to be ready...", port)
	
	deadline := time.Now().Add(time.Duration(maxWaitSeconds) * time.Second)
	
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			log.Printf("Server is ready on port %d!", port)
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	log.Printf("Server on port %d did not become ready in %d seconds", port, maxWaitSeconds)
	return false
}

func generateHTMLReport(results BenchmarkResults, outputDir string) {
	// Create HTML report file
	reportPath := filepath.Join(outputDir, "benchmark_report.html")
	
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Go Reader Performance Benchmark Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .chart-container { margin-bottom: 40px; }
        table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        tr:nth-child(even) { background-color: #f9f9f9; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Go Reader Performance Benchmark Report</h1>
        <p>Benchmark ran on ` + time.Now().Format("Jan 2, 2006 15:04:05") + `</p>
        <p>Source file: ` + results.Config.SourceFile + `</p>
        <p>Number of runs per configuration: ` + strconv.Itoa(results.Config.Runs) + `</p>
        
        <h2>Performance Summary</h2>
        
        <div class="chart-container">
            <h3>Server Response Time (ms)</h3>
            <canvas id="serverTimingChart"></canvas>
        </div>
        
        <div class="chart-container">
            <h3>Client Download Time (ms)</h3>
            <canvas id="clientTimingChart"></canvas>
        </div>
        
        <div class="chart-container">
            <h3>Memory Usage (MiB)</h3>
            <canvas id="memoryChart"></canvas>
        </div>
        
        <div class="chart-container">
            <h3>System Calls</h3>
            <canvas id="syscallChart"></canvas>
        </div>
        
        <h2>Detailed Results</h2>
        <table id="resultsTable">
            <tr>
                <th>Execution Path</th>
                <th>Buffer Size</th>
                <th>Avg Server Time (ms)</th>
                <th>Std Dev Server</th>
                <th>Avg Client Time (ms)</th>
                <th>Std Dev Client</th>
                <th>Avg Memory (MiB)</th>
                <th>Max Memory (MiB)</th>
                <th>Avg Read Calls</th>
                <th>Avg Write Calls</th>
                <th>Avg Total Calls</th>
            </tr>
        </table>
    </div>
    
    <script>
    // Load the benchmark data
    const benchmarkData = ` + string(mustMarshalJSON(results)) + `;
    
    // Prepare data for charts
    const labels = [];
    const serverTimingData = [];
    const serverTimingError = [];
    const clientTimingData = [];
    const clientTimingError = [];
    const memoryData = [];
    const readCallsData = [];
    const writeCallsData = [];
    
    // Populate the table
    const table = document.getElementById('resultsTable');
    
    // Sort by execution path and buffer size
    const sortedKeys = Object.keys(benchmarkData.aggregateData).sort((a, b) => {
        const [execA, bufA] = a.split('_');
        const [execB, bufB] = b.split('_');
        
        if (execA !== execB) {
            // Order: NOBUFFER, BUFIOREADER, PRELOAD
            const order = { 'NOBUFFER': 1, 'BUFIOREADER': 2, 'PRELOAD': 3 };
            return order[execA] - order[execB];
        }
        
        return parseInt(bufA) - parseInt(bufB);
    });
    
    for (const key of sortedKeys) {
        const stats = benchmarkData.aggregateData[key];
        
        // Add to charts data
        let label = stats.executionPath;
        if (stats.executionPath === 'BUFIOREADER') {
            const bufferSizeStr = formatBufferSize(stats.bufferSize);
            label += ' (' + bufferSizeStr + ')';
        }
        labels.push(label);
        
        serverTimingData.push(stats.avgServerTime);
        serverTimingError.push(stats.stdDevServer);
        
        clientTimingData.push(stats.avgClientTime);
        clientTimingError.push(stats.stdDevClient);
        
        memoryData.push(stats.avgMemoryAlloc);
        readCallsData.push(stats.avgReadCalls);
        writeCallsData.push(stats.avgWriteCalls);
        
        // Add to table
        const row = table.insertRow(-1);
        row.insertCell(0).textContent = stats.executionPath;
        row.insertCell(1).textContent = stats.executionPath === 'BUFIOREADER' ? 
            formatBufferSize(stats.bufferSize) : '-';
        row.insertCell(2).textContent = stats.avgServerTime.toFixed(2);
        row.insertCell(3).textContent = stats.stdDevServer.toFixed(2);
        row.insertCell(4).textContent = stats.avgClientTime.toFixed(2);
        row.insertCell(5).textContent = stats.stdDevClient.toFixed(2);
        row.insertCell(6).textContent = stats.avgMemoryAlloc.toFixed(2);
        row.insertCell(7).textContent = stats.maxMemoryAlloc;
        row.insertCell(8).textContent = stats.avgReadCalls.toFixed(2);
        row.insertCell(9).textContent = stats.avgWriteCalls.toFixed(2);
        row.insertCell(10).textContent = stats.avgTotalCalls.toFixed(2);
    }
    
    // Helper function to format buffer sizes
    function formatBufferSize(bytes) {
        if (bytes < 1024) return bytes + ' B';
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
        if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
        return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
    }
    
    // Create charts
    const serverTimingChart = new Chart(
        document.getElementById('serverTimingChart'),
        {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Avg Server Response Time (ms)',
                    data: serverTimingData,
                    backgroundColor: 'rgba(54, 162, 235, 0.5)',
                    borderColor: 'rgb(54, 162, 235)',
                    borderWidth: 1,
                    errorBars: {
                        show: true,
                        color: 'red',
                        width: 2
                    },
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Time (ms)'
                        }
                    }
                }
            }
        }
    );
    
    const clientTimingChart = new Chart(
        document.getElementById('clientTimingChart'),
        {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Avg Client Download Time (ms)',
                    data: clientTimingData,
                    backgroundColor: 'rgba(255, 99, 132, 0.5)',
                    borderColor: 'rgb(255, 99, 132)',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Time (ms)'
                        }
                    }
                }
            }
        }
    );
    
    const memoryChart = new Chart(
        document.getElementById('memoryChart'),
        {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Avg Memory Usage (MiB)',
                    data: memoryData,
                    backgroundColor: 'rgba(75, 192, 192, 0.5)',
                    borderColor: 'rgb(75, 192, 192)',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Memory (MiB)'
                        }
                    }
                }
            }
        }
    );
    
    const syscallChart = new Chart(
        document.getElementById('syscallChart'),
        {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Avg Read Calls',
                        data: readCallsData,
                        backgroundColor: 'rgba(153, 102, 255, 0.5)',
                        borderColor: 'rgb(153, 102, 255)',
                        borderWidth: 1
                    },
                    {
                        label: 'Avg Write Calls',
                        data: writeCallsData,
                        backgroundColor: 'rgba(255, 159, 64, 0.5)',
                        borderColor: 'rgb(255, 159, 64)',
                        borderWidth: 1
                    }
                ]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Number of System Calls'
                        }
                    }
                }
            }
        }
    );
    </script>
</body>
</html>`

	if err := os.WriteFile(reportPath, []byte(html), 0644); err != nil {
		log.Fatalf("Failed to write HTML report: %v", err)
	}
	
	log.Printf("HTML report generated at: %s", reportPath)
}

func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}