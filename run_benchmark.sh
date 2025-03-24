#!/bin/bash

# Default values
SOURCE_FILE=""
PORT=8082
RUNS=3
BUFFER_SIZES="1024,8192,65536,1048576,16777216,1073741824"
OUTPUT_DIR="benchmark_results"

# Help function
show_help() {
    echo "Usage: run_benchmark.sh [options]"
    echo "Options:"
    echo "  -s, --source FILE     Source file to use for benchmarking (required)"
    echo "  -p, --port PORT       HTTP server port (default: 8082)"
    echo "  -r, --runs NUM        Number of runs per configuration (default: 3)"
    echo "  -b, --buffers SIZES   Comma-separated list of buffer sizes in bytes (default: 1024,8192,65536,1048576,16777216,1073741824)"
    echo "  -o, --output DIR      Output directory for benchmark results (default: benchmark_results)"
    echo "  -h, --help            Show this help message"
    echo ""
    echo "Example:"
    echo "  ./run_benchmark.sh --source /path/to/large/file.dat --runs 5"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -s|--source)
            SOURCE_FILE="$2"
            shift
            shift
            ;;
        -p|--port)
            PORT="$2"
            shift
            shift
            ;;
        -r|--runs)
            RUNS="$2"
            shift
            shift
            ;;
        -b|--buffers)
            BUFFER_SIZES="$2"
            shift
            shift
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Check required parameters
if [ -z "$SOURCE_FILE" ]; then
    echo "Error: Source file is required"
    show_help
    exit 1
fi

# Check if source file exists
if [ ! -f "$SOURCE_FILE" ]; then
    echo "Error: Source file '$SOURCE_FILE' does not exist"
    exit 1
fi

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Run benchmark
echo "Starting benchmark with the following configuration:"
echo "Source file: $SOURCE_FILE"
echo "Port: $PORT"
echo "Runs per configuration: $RUNS"
echo "Buffer sizes: $BUFFER_SIZES"
echo "Output directory: $OUTPUT_DIR"
echo ""

go run benchmark.go \
    -source="$SOURCE_FILE" \
    -port=$PORT \
    -runs=$RUNS \
    -bufferSizes="$BUFFER_SIZES" \
    -output="$OUTPUT_DIR"

echo ""
echo "Benchmark complete! Results saved to $OUTPUT_DIR"
echo "Open report.html in a web browser to visualize the results"

# Final cleanup to ensure no stray server processes are left running
echo "Performing final cleanup of any lingering processes..."
# Find and kill any Go processes that might be running on ports in the range we used
for p in $(seq $PORT $(($PORT+20))); do
    if lsof -i :$p -t 2>/dev/null; then
        echo "Found lingering process on port $p, cleaning up..."
        lsof -i :$p -t | xargs -r kill -9
    fi
done

# Also look for any 'main.go' processes that might be running
if pgrep -f "go.*main.go" >/dev/null; then
    echo "Found lingering main.go process, cleaning up..."
    pkill -9 -f "go.*main.go"
fi

echo "Cleanup complete."