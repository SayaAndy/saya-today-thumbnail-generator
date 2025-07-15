package processor

import "io"

type Processor interface {
	DeductOutputPath(inputPath string) string
	Process(ext string, reader io.ReadCloser, writer io.WriteCloser) error
}
