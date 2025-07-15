package output

import (
	"io"
)

type OutputClient interface {
	GetWriter(path string) (io.Writer, error)
	ReadMetadata(string) (map[string]string, error)
	IsMissing(path string) (bool, error)
}
