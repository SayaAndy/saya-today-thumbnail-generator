package input

import (
	"io"
)

type InputClient interface {
	Scan() ([]string, error)
	ReadMetadata(string) (map[string]string, error)
	GetReader(string) (io.Reader, error)
}
