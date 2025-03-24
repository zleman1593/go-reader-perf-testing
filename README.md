# Go I/O Performance Benchmarking Tool

This project demonstrates and measures the performance differences between various I/O buffering strategies when serving files via HTTP. Understanding these differences is crucial for optimizing file-serving applications.

## Execution Modes

The program implements three distinct buffering strategies:

- **NOBUFFER**: Reads directly from the file without additional buffering
- **BUFIOREADER**: Uses `bufio.Reader` with a configurable buffer size
- **PRELOAD**: Loads the entire file into memory at startup for fastest response

## Basic Usage

```bash
# Run with no additional buffer
go run main.go -source=/path/to/largefile.dat -port=8080 -exec=NOBUFFER

# Run with bufio.Reader (8KB buffer)
go run main.go -source=/path/to/largefile.dat -port=8080 -exec=BUFIOREADER -buffer=8192

# Run with preloaded file
go run main.go -source=/path/to/largefile.dat -port=8080 -exec=PRELOAD
```

Then download the file using curl:

```bash
curl http://localhost:8080 -o output.file
```

## Performance Metrics

The program instruments and measures several key performance indicators:

- **Timing**: Response time from both server and client perspectives
- **Memory Usage**: Detailed memory allocation and usage statistics
- **System Calls**: Count of read/write operations at the system level
- **Throughput**: Transfer rates in MB/s

## Benchmarking Options

### Quick Benchmark

For a simple comparison of the three execution modes:

```bash
./simple_benchmark.sh /path/to/largefile.dat
```

This provides a quick summary of performance differences without detailed statistics.

### Comprehensive Benchmarking

For thorough analysis with multiple runs and statistical aggregation:

```bash
# Make the script executable
chmod +x run_benchmark.sh

# Run benchmark with default settings
./run_benchmark.sh --source /path/to/largefile.dat

# Run benchmark with custom settings
./run_benchmark.sh \
  --source /path/to/largefile.dat \
  --port 8082 \
  --runs 5 \
  --buffers "1024,8192,65536,1048576" \
  --output my_benchmark_results
```

#### Configuration Options

- `--source` (required): File to benchmark with
- `--port` (default: 8082): HTTP server port
- `--runs` (default: 3): Number of runs per configuration
- `--buffers` (default: "1024,8192,65536,1048576"): Buffer sizes to test
- `--output` (default: benchmark_results): Output directory

## Visualizing Results

Results are saved in two formats:
1. Raw data as JSON files
2. Interactive HTML visualization

To view the visualization:
1. Open `report.html` in your web browser
2. Click "Load Results" and select the JSON file from your benchmark run
3. Explore interactive charts and data tables

## Technical Details

### Implementation Features

- **Custom I/O Wrappers**: `CountingReader` and `CountingWriter` for accurate system call tracking
- **Memory Profiling**: Detailed memory statistics including heap usage
- **Statistical Analysis**: Averages, standard deviations, min/max values for all metrics
- **Concurrent Testing**: Multiple test runs with automatic port management
- **Detailed Comments**: Explanations of HTTP buffering, OS behaviors, and performance implications

### Learning Points

This project helps understand:
- How buffer sizes affect performance and memory usage
- The impact of system call frequency on throughput
- Trade-offs between memory usage and response time
- Behavior of HTTP server buffering
- Performance characteristics of different I/O strategies

## Performance Insights

Different strategies shine in different scenarios:

- **NOBUFFER**: Minimal memory overhead but may result in more system calls
- **BUFIOREADER**: Balance between memory usage and system call efficiency
- **PRELOAD**: Fastest response times but highest initial memory cost

The optimal approach depends on your specific use case:
- Small, frequently accessed files → PRELOAD
- Large files with infrequent access → BUFIOREADER
- Memory-constrained environments → NOBUFFER with appropriate OS-level caching