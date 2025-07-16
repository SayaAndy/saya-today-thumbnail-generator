package input

import (
	"io"
	"time"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
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

var NewInputClientMap = map[string]func(cfg *config.InputConfig) (InputClient, error){
	"b2":         NewB2InputClient,
	"local-unix": NewLocalUnixInputClient,
}
