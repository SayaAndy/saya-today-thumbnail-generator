package output

import (
	"io"
	"time"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
)

type OutputClient interface {
	GetWriter(path string, inputMetadata *input.MetadataStruct) (io.WriteCloser, error)
	ReadMetadata(path string) (*MetadataStruct, error)
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
	Size         int64
	Misc         map[string]string
}

var NewOutputClientMap = map[string]func(cfg *config.OutputConfig) (OutputClient, error){
	"b2":         NewB2OutputClient,
	"local-unix": NewLocalUnixOutputClient,
}
