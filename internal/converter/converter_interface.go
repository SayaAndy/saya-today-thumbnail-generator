package converter

import (
	"io"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/output"
)

type Converter interface {
	Process(inputMetadata *input.MetadataStruct, reader io.Reader, outputName string) error
	DeductOutputPath(inputPath string) string
	ReadMetadata(path string) (*output.MetadataStruct, error)
	IsMissing(path string) bool
}

var NewConverterMap = map[string]func(cfg *config.ConverterConfig) (Converter, error){
	"webp": NewWebpConverter,
}
