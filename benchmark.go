package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	flag.StringVar(&bufferSizesStr, "bufferSizes", "1024,8192,65536,1048576,16777216,1073741824", "Comma-separated list of buffer sizes for BUFIOREADER (in bytes)")
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
	
	// Prepare for system call tracing
	var straceTool string
	var straceArgs []string
	
	if runtime.GOOS == "linux" {
		straceTool = "strace"
		straceArgs = []string{"-c", "-f"}
	} else if runtime.GOOS == "darwin" {
		straceTool = "dtruss"
		straceArgs = []string{"-c"}
	}
	
	// If we have a system call tracer, use it
	if straceTool != "" {
		traceCmd := exec.Command(straceTool, append(straceArgs, serverCmd.Args...)...)
		serverCmd = traceCmd
	}
	
	// Start server in background
	serverOutput, err := os.CreateTemp("", "server_output_*.log")
	if err != nil {
		log.Fatalf("Failed to create server output file: %v", err)
	}
	defer os.Remove(serverOutput.Name())
	
	serverCmd.Stdout = serverOutput
	serverCmd.Stderr = serverOutput
	
	if err := serverCmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	
	// Allow server to start
	time.Sleep(2 * time.Second)
	
	// Create output file for curl
	outputFile, err := os.CreateTemp("", "curl_output_*.dat")
	if err != nil {
		log.Fatalf("Failed to create temporary file for curl output: %v", err)
	}
	defer os.Remove(outputFile.Name())
	
	// We don't need a separate timing file anymore since we're using simple output format
	
	// Run curl command to download the file and measure time
	// Use a simple format with just the time_total value
	curlCmd := exec.Command("curl",
		"-o", outputFile.Name(),
		"-w", "%{time_total}\n",
		"-s",
		fmt.Sprintf("http://localhost:%d", config.Port))
	
	// Don't use the more complex format which was causing parsing issues
	
	clientTimingOutput, err := curlCmd.Output()
	if err != nil {
		log.Printf("curl command failed: %v", err)
	}
	
	// Kill the server
	if err := serverCmd.Process.Kill(); err != nil {
		log.Printf("Failed to kill server: %v", err)
	}
	
	// Wait for the server to exit
	serverCmd.Wait()
	
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
	
	// Parse system calls
	result.SystemCalls = extractSystemCalls(string(serverLogs))
	
	return result
}

func extractServerTiming(logs string) float64 {
	// Look for the total request processing time
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "TIMING: Total request processing time:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				// Combine all remaining parts in case the time format contains colons
				timeStr := strings.TrimSpace(strings.Join(parts[2:], ":"))
				
				// Try to parse the duration based on different formats
				// First check if it's a Go duration format
				duration, err := time.ParseDuration(timeStr)
				if err == nil {
					return float64(duration.Milliseconds())
				}
				
				// Handle explicit units
				if strings.Contains(timeStr, "ms") {
					numStr := strings.TrimSpace(strings.TrimSuffix(timeStr, "ms"))
					ms, err := strconv.ParseFloat(numStr, 64)
					if err == nil {
						return ms
					}
				} else if strings.Contains(timeStr, "µs") || strings.Contains(timeStr, "us") {
					var numStr string
					if strings.Contains(timeStr, "µs") {
						numStr = strings.TrimSpace(strings.TrimSuffix(timeStr, "µs"))
					} else {
						numStr = strings.TrimSpace(strings.TrimSuffix(timeStr, "us"))
					}
					us, err := strconv.ParseFloat(numStr, 64)
					if err == nil {
						return us / 1000 // Convert microseconds to milliseconds
					}
				} else if strings.Contains(timeStr, "s") && !strings.Contains(timeStr, "ms") {
					numStr := strings.TrimSpace(strings.TrimSuffix(timeStr, "s"))
					s, err := strconv.ParseFloat(numStr, 64)
					if err == nil {
						return s * 1000 // Convert seconds to milliseconds
					}
				}
				
				// Last attempt: try to parse as a float (assuming seconds)
				s, err := strconv.ParseFloat(strings.TrimSpace(timeStr), 64)
				if err == nil {
					return s * 1000 // Convert seconds to milliseconds
				}
				
				log.Printf("Failed to parse timing: %s", timeStr)
			}
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
	
	// Look for strace or dtruss output
	inSyscallSection := false
	
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "syscall") && strings.Contains(line, "calls") {
			inSyscallSection = true
			continue
		}
		
		if inSyscallSection {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			
			syscallName := fields[len(fields)-1]
			countStr := fields[0]
			count, err := strconv.Atoi(countStr)
			if err != nil {
				continue
			}
			
			if strings.Contains(syscallName, "read") {
				stats.ReadCalls += count
			} else if strings.Contains(syscallName, "write") {
				stats.WriteCalls += count
			}
			
			stats.TotalCalls += count
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