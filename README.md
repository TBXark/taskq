# TaskQ

A lightweight, generic task queue implementation in Go with support for timeouts and automatic retries.

## Features

- Generic type support
- Concurrent task execution
- Configurable timeout per task
- Automatic retry mechanism
- Graceful shutdown
- Panic recovery

## Installation

```bash
go get github.com/TBXark/taskq@latest
```

## Example

```go
package main

import (
    "fmt"
    "time"
    "github.com/TBXark/taskq"
)

func main() {
    // Create a new queue for string results
    queue := taskq.NewQueue[string](0)
    defer queue.Close()

    // Submit a task
    taskID, resultChan := queue.SubmitTask(
        func() (*string, error) {
            // Simulate some work
            time.Sleep(time.Second)
            result := "Hello, World!"
            return &result, nil
        },
        2,              // max retries
        5*time.Second,  // timeout
    )

    // Wait for the result
    result := <-resultChan
    if result.Err != nil {
        fmt.Printf("Task %d failed: %v\n", taskID, result.Err)
    } else {
        fmt.Printf("Task %d succeeded: %s\n", taskID, *result.Data)
    }
}
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
