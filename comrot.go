// Comrot provides a compressed log rotation writer.
// It satisfies the io.WriteCloser interface.
package comrot

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	MaxInt  = int(^uint(0) >> 1)
	MB      = 1 << (10 * 2)
	TimeFmt = time.RFC3339
)

var (
	DefaultMaxSize  = 10 * MB
	DefaultMaxFiles = MaxInt
)

// Ensure we always implement io.WriteCloser
var _ io.WriteCloser = (*RotateWriter)(nil)

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

// NewRotateWriter creates a new RotateWriter.
// Returns nil if an error occurs during setup.
func NewRotateWriter(filename string) *RotateWriter {
	w := &RotateWriter{
		filename: filename,
		MaxSize:  DefaultMaxSize,
		MaxFiles: DefaultMaxFiles,
		Compress: true,
	}
	err := w.Open()
	if err != nil {
		return nil
	}
	return w
}

// Write satisfies the io.Writer interface.
func (w *RotateWriter) Write(out []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Open if closed.
	if w.fp == nil {
		w.open()
	}

	// Rotate if write exceeds threshold.
	if w.fsize+int64(len(out)) > int64(w.MaxSize) {
		w.rotate()
	}

	return w.write(out)
}

// write actually writes to the log.
func (w *RotateWriter) write(out []byte) (int, error) {
	n, err := w.fp.Write(out)
	w.fsize += int64(n)
	return n, err
}

// Close statisfies the io.Closer interface.
func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.close()
}

// close actually closes the writer.
func (w *RotateWriter) close() error {
	if w.fp == nil {
		return nil
	}
	w.fp = nil
	return w.fp.Close()
}

// Open opens the log file if it exists. Creates a
// new file otherwise.
func (w *RotateWriter) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.open()
}

// open opens or creates the log file.
func (w *RotateWriter) open() error {
	info, err := os.Stat(w.filename)
	if os.IsNotExist(err) {
		w.fp, err = os.Create(w.filename)
		w.fsize = int64(0)
		return err
	}
	w.fp, err = os.OpenFile(w.filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	w.fsize = info.Size()
	return nil
}

// Rotate performs the rotation and creation of files.
func (w *RotateWriter) Rotate() (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotate()
}

// rotate actually rotates the log file.
func (w *RotateWriter) rotate() (err error) {
	// Close existing file if open.
	if w.fp != nil {
		err = w.fp.Close()
		w.fp = nil
		if err != nil {
			return
		}
	}

	// Rename dest file if it already exists.
	_, err = os.Stat(w.filename)
	if err == nil {
		rot := w.filename + "." + time.Now().Format(TimeFmt)
		err = os.Rename(w.filename, rot)
		if err != nil {
			return err
		}
		if w.Compress {
			err = w.compress(rot) // TODO: async
			if err != nil {
				return err
			}
		}
	}

	// Clean up old.
	w.drain()

	// Create new.
	return w.open()
}

// compress compresses a file with gzip algorithm.
func (w *RotateWriter) compress(source string) (err error) {
	// Read uncompressed file.
	rawfile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer rawfile.Close()

	// Calculate the buffer size.
	info, _ := rawfile.Stat()
	rawbytes := make([]byte, info.Size())

	// Read rawfile content into buffer.
	buffer := bufio.NewReader(rawfile)
	_, err = buffer.Read(rawbytes)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	writer.Write(rawbytes)
	writer.Close()

	err = ioutil.WriteFile(source+".gz", buf.Bytes(), info.Mode())
	if err != nil {
		return err
	}

	// Remove uncompressed.
	go os.Remove(source)

	return nil
}

// drain cleans old and archived files.
func (w *RotateWriter) drain() {
	if w.MaxFiles == MaxInt {
		return
	}
	files, err := ioutil.ReadDir(filepath.Dir(w.filename))
	if err != nil {
		return
	}

	// Collect log fragments.
	frags := []fragInfo{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if f.Name() == w.filename {
			continue
		}
		base, file := filepath.Base(w.filename), f.Name()
		// Extract timestamp from filename.
		ts := file[len(base)+1 : len(base)+1+len(TimeFmt)]
		t, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			frags = append(frags, fragInfo{t, f})
		}
	}
	sort.Sort(byTime(frags))

	// Collect deletable fragmets.
	deletes := []fragInfo{}
	if w.MaxFiles < len(frags) {
		deletes = frags[w.MaxFiles:]
		frags = frags[:w.MaxFiles]
	}

	go func(fs []fragInfo) {
		for _, f := range fs {
			os.Remove(filepath.Join(filepath.Dir(w.filename), f.Name()))

		}
	}(deletes)
}

// fragInfo is a log fragment with file info.
type fragInfo struct {
	t time.Time
	os.FileInfo
}

// byTime implements sort.Interface for []fragInfo.
// Sorts based on the time field and in descending order.
type byTime []fragInfo

func (b byTime) Len() int           { return len(b) }
func (b byTime) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byTime) Less(i, j int) bool { return b[i].t.After(b[j].t) }
