This Go program demonstrates the performance differences between different I/O buffering strategies when serving files via HTTP. It implements three different execution modes:

NOBUFFER - Reads directly from the file without any additional buffering
BUFIOREADER - Uses bufio.Reader with a configurable buffer size
PRELOAD - Loads the entire file into memory at startup for fastest response

How to use it:

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

Learning points:

The program measures and logs timing information for all critical operations
Memory usage is tracked to understand the impact of different buffering strategies
Detailed comments explain what's happening behind the scenes with each approach
You can experiment with different buffer sizes to find optimal performance

The code includes explanations about HTTP ResponseWriter's internal buffering and system call patterns for each approach, making it a great educational tool for understanding I/O performance.
