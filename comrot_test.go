package comrot

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	logDir   = "crt"
	filename = "comrot.log"
)

var (
	logFile = filepath.Join(logDir, filename)
)

// setup initializes the test env.
func setup() {
	os.MkdirAll(logDir, 0755)
}

// teardown cleans up the test env.
func teardown() {
	defer os.RemoveAll(logDir)
}

// TestNew verifies that the returned instance is of proper type.
func TestNew(t *testing.T) {
	setup()
	defer teardown()

	w := NewRotateWriter(logFile)

	want := "*comrot.RotateWriter"
	if reflect.TypeOf(w).String() != want {
		t.Error("NewRotateWriter returned incorrect type.")
	}
}

// TestCreate verifies that the log file is being created.
func TestCreate(t *testing.T) {
	setup()
	defer teardown()

	NewRotateWriter(logFile)

	_, err := os.Stat(logFile)
	if err != nil {
		t.Error("expected file to exist")
	}
}

// TestOpen verifies that the log is appended to the existing.
func TestOpen(t *testing.T) {
	setup()
	defer teardown()

	ioutil.WriteFile(logFile, []byte("foo"), 0644)

	w := NewRotateWriter(logFile)
	w.Write([]byte("bar"))

	want := "foobar"
	b, _ := ioutil.ReadFile(logFile)
	if string(b) != want {
		t.Errorf("TestOpen, got %v, want %v", string(b), want)
	}
}

// TestRotate verifies that the log is rotated when MaxSixe is exceeded.
func TestRotate(t *testing.T) {
	setup()
	defer teardown()

	w := &RotateWriter{
		filename: logFile,
		MaxSize:  56,
		MaxFiles: 10,
		Compress: false,
	}
	w.Open()
	defer w.Close()

	w.Write([]byte("Text that clearly exceeds the MaxSize of 10 Bytes."))
	w.Write([]byte("Some more bytes."))

	files, _ := ioutil.ReadDir(logDir)
	if len(files) != 2 {
		t.Errorf("TestRotate, got %v, want %v", len(files), 2)
	}
}

// TestCompress verifies that rotated files get compressed.
func TestCompress(t *testing.T) {
	setup()
	defer teardown()

	w := &RotateWriter{
		filename: logFile,
		MaxSize:  56,
		MaxFiles: 10,
		Compress: true,
	}
	w.Open()
	defer w.Close()

	w.Write([]byte("Text that clearly exceeds the MaxSize of 10 Bytes."))
	w.Write([]byte("Some more bytes."))

	files, _ := filepath.Glob(logDir + "/*.gz")
	if len(files) != 1 {
		t.Errorf("TestCompress, got %v, want %v", len(files), 1)
	}
}
