#!/bin/bash

# A simplified benchmark script that focuses on just two configurations

if [ -z "$1" ]; then
    echo "Usage: ./simple_benchmark.sh /path/to/test/file"
    exit 1
fi

SOURCE_FILE="$1"
OUTPUT_DIR="benchmark_results"

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

echo "Running simplified benchmark with just NOBUFFER and BUFIOREADER (8KB)"
echo "Source file: $SOURCE_FILE"
echo "Output directory: $OUTPUT_DIR"
echo ""

# First test NOBUFFER mode only
echo "Testing NOBUFFER mode..."
go run main.go -source="$SOURCE_FILE" -port=8085 -exec=NOBUFFER &
SERVER_PID=$!

# Wait for server to start
sleep 3

# Run curl and measure time
echo "Running curl to test NOBUFFER mode..."
NOBUFFER_TIME=$(curl -s -w "%{time_total}\n" --range "0-10485760" -o /dev/null http://localhost:8085)
echo "NOBUFFER download time: ${NOBUFFER_TIME}s"

# Kill the server and make sure it's actually gone
echo "Stopping server process..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null

# Additional cleanup in case the main process doesn't exit properly
if kill -0 $SERVER_PID 2>/dev/null; then
    echo "Server didn't exit properly, forcing kill..."
    kill -9 $SERVER_PID
    wait $SERVER_PID 2>/dev/null
fi

# Make sure no processes are listening on the port
if lsof -i :8085 -t >/dev/null 2>&1; then
    echo "Cleaning up lingering processes on port 8085..."
    lsof -i :8085 -t | xargs -r kill -9
fi

# Wait to ensure port is released
sleep 3

# Then test BUFIOREADER mode
echo "Testing BUFIOREADER mode with 8KB buffer..."
go run main.go -source="$SOURCE_FILE" -port=8085 -exec=BUFIOREADER -buffer=8192 &
SERVER_PID=$!

# Wait for server to start
sleep 3

# Run curl and measure time
echo "Running curl to test BUFIOREADER mode..."
BUFIOREADER_TIME=$(curl -s -w "%{time_total}\n" --range "0-10485760" -o /dev/null http://localhost:8085)
echo "BUFIOREADER download time: ${BUFIOREADER_TIME}s"

# Kill the server and make sure it's actually gone
echo "Stopping server process..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null

# Additional cleanup in case the main process doesn't exit properly
if kill -0 $SERVER_PID 2>/dev/null; then
    echo "Server didn't exit properly, forcing kill..."
    kill -9 $SERVER_PID
    wait $SERVER_PID 2>/dev/null
fi

# Make sure no processes are listening on the port
if lsof -i :8085 -t >/dev/null 2>&1; then
    echo "Cleaning up lingering processes on port 8085..."
    lsof -i :8085 -t | xargs -r kill -9
fi

# Wait to ensure port is released
sleep 3

# Finally test PRELOAD mode
echo "Testing PRELOAD mode..."
go run main.go -source="$SOURCE_FILE" -port=8085 -exec=PRELOAD &
SERVER_PID=$!

# Wait for server to start
sleep 3

# Run curl and measure time
echo "Running curl to test PRELOAD mode..."
PRELOAD_TIME=$(curl -s -w "%{time_total}\n" --range "0-10485760" -o /dev/null http://localhost:8085)
echo "PRELOAD download time: ${PRELOAD_TIME}s"

# Kill the server and make sure it's actually gone
echo "Stopping server process..."
kill -TERM $SERVER_PID
wait $SERVER_PID 2>/dev/null

# Additional cleanup in case the main process doesn't exit properly
if kill -0 $SERVER_PID 2>/dev/null; then
    echo "Server didn't exit properly, forcing kill..."
    kill -9 $SERVER_PID
    wait $SERVER_PID 2>/dev/null
fi

# Make sure no processes are listening on the port
if lsof -i :8085 -t >/dev/null 2>&1; then
    echo "Cleaning up lingering processes on port 8085..."
    lsof -i :8085 -t | xargs -r kill -9
fi

# Output summary
echo ""
echo "BENCHMARK SUMMARY"
echo "----------------"
echo "NOBUFFER time:    ${NOBUFFER_TIME}s"
echo "BUFIOREADER time: ${BUFIOREADER_TIME}s"
echo "PRELOAD time:     ${PRELOAD_TIME}s"
echo ""

# Basic analysis
FASTEST="NOBUFFER"
FASTEST_TIME=$NOBUFFER_TIME

if (( $(echo "$BUFIOREADER_TIME < $FASTEST_TIME" | bc -l) )); then
    FASTEST="BUFIOREADER (8KB)"
    FASTEST_TIME=$BUFIOREADER_TIME
fi

if (( $(echo "$PRELOAD_TIME < $FASTEST_TIME" | bc -l) )); then
    FASTEST="PRELOAD"
    FASTEST_TIME=$PRELOAD_TIME
fi

echo "Fastest method: $FASTEST (${FASTEST_TIME}s)"
echo ""
echo "For more detailed benchmarking, use: ./run_benchmark.sh --source \"$SOURCE_FILE\" --runs 3"