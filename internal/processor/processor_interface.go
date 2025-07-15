package processor

import "io"

type Processor interface {
	DeductOutputPath(inputPath string) string
	Process(ext string, reader io.Reader, writer io.Writer) error
}
