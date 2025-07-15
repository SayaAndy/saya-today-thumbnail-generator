package output

import (
	"io"
	"time"

	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
)

type OutputClient interface {
	GetWriter(path string, inputMetadata *input.MetadataStruct) (io.WriteCloser, error)
	ReadMetadata(string) (*MetadataStruct, error)
	IsMissing(path string) bool
}

type MetadataStruct struct {
	Name         string
	StorageType  string
	Hash         string
	HashOriginal string
	ContentType  string
	FirstCreated time.Time
	LastModified time.Time
	Misc         map[string]string
}
