package converter

import "io"

type Converter interface {
	DeductOutputPath(inputPath string) string
	Process(ext string, reader io.ReadCloser, writer io.WriteCloser) error
}
