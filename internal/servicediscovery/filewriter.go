package servicediscovery

import (
	"io"
	"path/filepath"

	"github.com/spf13/afero"
)

// Filename the filename of the service discovery config
const Filename = "health-check-service-discovery.json"

type fileWriter struct {
	Directory string
	fs        afero.Fs
}

// NewFileWriter returns a default filewriter with an OS filesystem implementation
func NewFileWriter(directory string, fs afero.Fs) io.Writer {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return fileWriter{Directory: directory, fs: fs}
}

func (fileWriter fileWriter) Write(p []byte) (n int, err error) {
	if err := afero.WriteFile(fileWriter.fs, filepath.Join(fileWriter.Directory, Filename), p, 0644); err != nil {
		return 0, err
	}
	return len(p), nil
}
