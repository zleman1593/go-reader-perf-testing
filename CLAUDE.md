# CLAUDE.md - Go-Reader-Learn Project Guidelines

## Build Commands
- Build: `go build`
- Run: `go run main.go -source=/path/to/file -port=8080 -exec=[NOBUFFER|BUFIOREADER|PRELOAD] -buffer=8192`
- Format code: `go fmt ./...`
- Lint code: `golint ./...`
- Test: `go test ./...`
- Single test: `go test -run TestName ./...`

## Code Style Guidelines

### Formatting & Structure
- Use `go fmt` to ensure standard formatting
- Group imports: standard library first, then third-party packages
- Document all exported functions, types, and constants
- Use detailed comments for complex logic or algorithms

### Types & Naming
- Use descriptive variable names (camelCase for variables, PascalCase for exported)
- Use strong typing - avoid interfaces{} when possible
- Prefer enums (using const and type definitions) for fixed sets of values
- Use meaningful type names that describe what the type represents

### Error Handling
- Check and handle all errors explicitly
- Use descriptive error messages with context
- Propagate errors up with additional context when appropriate
- Use log.Fatalf() for unrecoverable errors only

### Performance Considerations
- Be explicit about buffer sizes when using buffered I/O
- Log memory usage for operations that may use significant memory
- Use time measurements for performance-critical operations
- Consider both memory usage and I/O speed in implementations