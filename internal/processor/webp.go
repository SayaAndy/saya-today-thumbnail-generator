package processor

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/draw"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

var _ Processor = (*WebpProcessor)(nil)

type WebpProcessor struct {
	maxWidth  int
	maxHeight int
	quality   int
}

func NewWebpProcessor(cfg *config.ProcessorConfig) (*WebpProcessor, error) {
	if cfg.Type != "webp" {
		return nil, fmt.Errorf("invalid storage type for WebpProcessor")
	}
	webpCfg := cfg.Config.(*config.WebpConfig)

	return &WebpProcessor{webpCfg.Size.MaxWidth, webpCfg.Size.MaxHeight, webpCfg.Quality}, nil
}

func (p *WebpProcessor) DeductOutputPath(inputPath string) string {
	pathParts := strings.Split(inputPath, ".")
	if len(pathParts) < 2 {
		return inputPath + ".webp"
	}
	pathParts[len(pathParts)-1] = "webp"
	return strings.Join(pathParts, ".")
}

func (p *WebpProcessor) Process(contentType string, reader io.Reader, writer io.Writer) error {
	var src image.Image
	var err error

	switch contentType {
	case "image/jpeg":
		src, err = jpeg.Decode(reader)
		if err != nil {
			return fmt.Errorf("decode jpeg: %w", err)
		}
	case "image/png":
		src, err = png.Decode(reader)
		if err != nil {
			return fmt.Errorf("decode png: %w", err)
		}
	}

	opts, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(p.quality))
	if err != nil {
		return fmt.Errorf("create webp encoder options: %w", err)
	}

	xCoef := p.maxWidth / src.Bounds().Max.X
	yCoef := p.maxHeight / src.Bounds().Max.Y
	if xCoef > 1 && yCoef > 1 {
		return webp.Encode(writer, src, opts)
	}

	minCoef := xCoef
	if yCoef < minCoef {
		minCoef = yCoef
	}

	dst := image.NewRGBA(image.Rect(0, 0, src.Bounds().Max.X*xCoef, src.Bounds().Max.Y*xCoef))
	draw.CatmullRom.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	return webp.Encode(writer, dst, opts)
}
