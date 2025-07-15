package input

import (
	"io"
	"time"
)

type InputClient interface {
	Scan() ([]string, error)
	ReadMetadata(string) (*MetadataStruct, error)
	GetReader(string) (io.ReadCloser, error)
}

type MetadataStruct struct {
	Name         string
	StorageType  string
	Hash         string
	ContentType  string
	FirstCreated time.Time
	LastModified time.Time
	Misc         map[string]string
}
