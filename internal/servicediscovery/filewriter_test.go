package servicediscovery

import (
	"os"
	"path/filepath"

	"github.com/stretchr/testify/assert"

	"github.com/spf13/afero"

	"testing"
)

func TestWritesTheGivenFileIfTheDirectoryExists(t *testing.T) {
	memoryFS := afero.NewMemMapFs()

	err := memoryFS.MkdirAll("/test-dir/path", 0755)

	if err != nil {
		t.Errorf("error creating test directory: \"%s\"", err)
	}

	fileWriter := NewFileWriter("/test-dir/path", memoryFS)
	_, err = fileWriter.Write([]byte("file"))
	if err != nil {
		t.Errorf("error running write: \"%s\"", err)
	}
	path := filepath.Join("/test-dir/path/", Filename)
	_, err = memoryFS.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("file \"%s\" does not exist.\n", path)
	}
}

func TestWritesTheGivenFileIfTheDirectoryDoesNotExist(t *testing.T) {
	memoryFS := afero.NewMemMapFs()

	fileWriter := NewFileWriter("/test-dir/path", memoryFS)
	_, err := fileWriter.Write([]byte("file"))
	if err != nil {
		t.Errorf("error running write: \"%s\"", err)
	}
	path := filepath.Join("/test-dir/path/", Filename)
	_, err = memoryFS.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("file \"%s\" does not exist.\n", path)
	}
}

func TestWritesTheCorrectContent(t *testing.T) {
	content := []byte("{\"some\": \"json\"}")
	memoryFS := afero.NewMemMapFs()
	path := filepath.Join("/test-dir/path", Filename)

	fileWriter := NewFileWriter("/test-dir/path", memoryFS)
	bytesWritten, err := fileWriter.Write(content)
	if err != nil {
		t.Errorf("error running write: \"%s\"", err)
	}
	file, err := memoryFS.Open(path)
	if err != nil {
		t.Errorf("error opening file: \"%s\"", err)
	}
	written := make([]byte, len(content))
	bytesRead, err := file.Read(written)
	if err != nil {
		t.Errorf("error reading file: \"%s\"", err)
	}

	assert.Exactly(t, len(content), bytesWritten, "Expected bytes written to match length of given string")
	assert.Exactly(t, content, written, "Expected correct content to be written to file but it did not match")
	assert.Exactly(t, bytesWritten, bytesRead, "Expected bytes written to match bytes read from file")
}
