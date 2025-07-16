package converter

import (
	"io"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
)

type Converter interface {
	DeductOutputPath(inputPath string) string
	Process(ext string, reader io.ReadCloser, writer io.WriteCloser) error
}

var NewConverterMap = map[string]func(cfg *config.ConverterConfig) (Converter, error){
	"webp": NewWebpConverter,
}
