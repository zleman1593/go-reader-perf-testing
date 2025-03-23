This Go program demonstrates the performance differences between different I/O buffering strategies when serving files via HTTP. It implements three different execution modes:

NOBUFFER - Reads directly from the file without any additional buffering
BUFIOREADER - Uses bufio.Reader with a configurable buffer size
PRELOAD - Loads the entire file into memory at startup for fastest response

## Basic Usage

```
# Run with no additional buffer
go run main.go -source=/path/to/largefile.dat -port=8080 -exec=NOBUFFER

# Run with bufio.Reader (8KB buffer)
go run main.go -source=/path/to/largefile.dat -port=8080 -exec=BUFIOREADER -buffer=8192

# Run with preloaded file
go run main.go -source=/path/to/largefile.dat -port=8080 -exec=PRELOAD
```

Then download the file using curl:

```
curl http://localhost:8080 -o output.file
```

## Performance Benchmarking

The project includes a comprehensive benchmarking system to compare the performance of different buffering strategies across various metrics:

- Server-side response time
- Client-side download time
- Memory usage
- System call counts (read/write/total)

### Running Benchmarks

Use the provided benchmark script to run tests with different configurations:

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

### Benchmark Options

- `--source` (required): Source file to use for benchmarking
- `--port` (default: 8082): HTTP server port
- `--runs` (default: 3): Number of runs per configuration
- `--buffers` (default: various sizes from 1KB to 1GB): Comma-separated list of buffer sizes for BUFIOREADER
- `--output` (default: benchmark_results): Directory to store benchmark results

### Viewing Results

After running benchmarks, results are saved in:
1. JSON format for raw data
2. An HTML report for visualization

Open `report.html` in your web browser to view charts and tables comparing all execution modes.

## Learning Points

- The program measures and logs timing information for all critical operations
- Memory usage is tracked to understand the impact of different buffering strategies
- System call patterns are monitored to understand I/O efficiency
- Detailed comments explain what's happening behind the scenes with each approach
- You can experiment with different buffer sizes to find optimal performance

The benchmarking system provides statistical analysis including averages, standard deviations, and ranges for all metrics, making it a powerful tool for understanding I/O performance patterns and tradeoffs.