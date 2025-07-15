package processor

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
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

func (p *WebpProcessor) Process(contentType string, reader io.ReadCloser, writer io.WriteCloser) error {
	var src image.Image
	var err error

	defer reader.Close()
	defer writer.Close()

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
	default:
		return fmt.Errorf("unsupported content type: %s", contentType)
	}

	opts, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(p.quality))
	if err != nil {
		return fmt.Errorf("create webp encoder options: %w", err)
	}

	xCoef := float64(p.maxWidth) / float64(src.Bounds().Max.X)
	if p.maxWidth == 0 {
		xCoef = 1
	}
	yCoef := float64(p.maxHeight) / float64(src.Bounds().Max.Y)
	if p.maxHeight == 0 {
		yCoef = 1
	}
	slog.Debug("calculated coefficients", slog.Float64("x_coef", xCoef), slog.Float64("y_coef", yCoef))

	if xCoef > 1 && yCoef > 1 {
		return webp.Encode(writer, src, opts)
	}

	minCoef := xCoef
	if yCoef < minCoef {
		minCoef = yCoef
	}

	dst := image.NewRGBA(image.Rect(0, 0, int(float64(src.Bounds().Max.X)*minCoef+0.5), int(float64(src.Bounds().Max.Y)*minCoef+0.5)))
	draw.CatmullRom.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	return webp.Encode(writer, dst, opts)
}
