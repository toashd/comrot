# Comrot
Package comrot provides a compressed log rotation writer.

[![CI Status](http://img.shields.io/travis/toashd/comrot.svg?style=flat)](https://travis-ci.org/toashd/comrot)

## Installation

```bash
$ go get github.com/toashd/comrot
```

## Usage

```go
import "github.com/toashd/comrot"
```

Create a new `RotateWriter` and start writing to it.

```go
w := comrot.NewRotateWriter(filename)
w.Write([]byte("Foo"))

fmt.Fprint(w, "Bar")
```

comrot can be easily configured to rotate depending on file size, whether to compress
rotated files or not and how many rotated files to retain.

```go
// RotateWriter is the (compressed) rotated log writer.
type RotateWriter struct {
	// Filename is the name of the file to write to.
	filename string

	// MaxSize is the threshold in MB of the log file size.
	// Once exceeded the writer rotates. It defaults to 10 MB.
	MaxSize int

	// MaxFiles is the maximum number of rotated files
	// to retain. Default is infinite.
	MaxFiles int

	// Compress is the flag indicating whether rotated
	// files should be compressed or not. Default is true.
	Compress bool

	// fp is the handle to the current log file.
	fp *os.File

	// fsize caches the current log file size.
	fsize int64

	// mu syncs writer ops.
	mu sync.Mutex
}
```

## Todo

* add `MaxAge` field and support cleaning up old logs based on timestamps.

## Contribution

Please feel free to suggest any kind of improvements, refactorings, just file an
issue or fork and submit a pull requests.

## License

Comrot is available under the MIT license. See the LICENSE file for more info.

