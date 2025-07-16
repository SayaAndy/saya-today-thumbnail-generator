package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/output"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/converter"
)

var (
	configPath  = flag.String("c", "config.json", "Path to the configuration file")
	sigTermChan = make(chan os.Signal, 1)
)

func main() {
	signal.Notify(sigTermChan, os.Interrupt, syscall.SIGTERM)

	flag.Parse()

	cfg := &config.Config{}
	if err := config.LoadConfig(*configPath, cfg); err != nil {
		slog.Error("fail to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(cfg.LogLevel)
	slog.Info("starting thumbnail generator...")

	select {
	case <-sigTermChan:
		slog.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	inputClient, err := input.NewInputClientMap[cfg.Input.Storage.Type](&cfg.Input)
	if err != nil {
		slog.Error("fail to initialize input client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	outputClient, err := output.NewOutputClientMap[cfg.Output.Storage.Type](&cfg.Output)
	if err != nil {
		slog.Error("fail to initialize output client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	conv, err := converter.NewConverterMap[cfg.Converter.Type](&cfg.Converter)
	if err != nil {
		slog.Error("fail to initialize converter", slog.String("error", err.Error()))
		os.Exit(1)
	}

	generalLogger := slog.With(
		slog.String("input_storage", cfg.Input.Storage.Type),
		slog.String("converter_type", cfg.Converter.Type),
		slog.String("output_storage", cfg.Output.Storage.Type),
	)
	generalLogger.Info("initialized clients and converter")

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	files, err := inputClient.Scan()
	if err != nil {
		generalLogger.Error("fail to scan input files", slog.String("error", err.Error()))
		os.Exit(1)
	}
	generalLogger.Info("scanned files", slog.Int("file_count", len(files)))

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	semaphore := make(chan struct{}, cfg.MaxConcurrentJobs)
	var wg sync.WaitGroup
	wg.Add(len(files))

	for i, file := range files {
		go func(index int, inputName string) {
			outputName := conv.DeductOutputPath(inputName)
			fileLogger := generalLogger.With(slog.String("input_path", inputName), slog.String("output_path", outputName), slog.Int("file_index", index))

			threadSigTermChannel := make(chan os.Signal, 1)
			signal.Notify(threadSigTermChannel, os.Interrupt, syscall.SIGTERM)

			defer func() { <-semaphore }()
			defer wg.Done()
			semaphore <- struct{}{}
			select {
			case <-threadSigTermChannel:
				fileLogger.Info("exiting due to termination signal")
				return
			default:
			}

			inputMetadata, err := inputClient.ReadMetadata(inputName)
			if err != nil {
				fileLogger.Warn("fail to read metadata of (supposedly existing) input file", slog.String("error", err.Error()))
				return
			}

			originalInputHash := ""
			if !cfg.ForceRewrite && !outputClient.IsMissing(outputName) {
				outputMetadata, err := outputClient.ReadMetadata(outputName)
				if err != nil {
					fileLogger.Warn("fail to read metadata of (supposedly existing) output file", slog.String("error", err.Error()))
					return
				}
				originalInputHash = outputMetadata.HashOriginal
				if inputMetadata.Hash == originalInputHash {
					fileLogger.Info("skip already processed file (based on equal hash)", slog.String("input_hash", inputMetadata.Hash))
					return
				}
			}

			fileLogger.Info("start to process file", slog.String("input_hash", inputMetadata.Hash), slog.String("original_input_hash", originalInputHash))
			reader, err := inputClient.GetReader(inputName)
			if err != nil {
				fileLogger.Warn("fail to get reader for input file", slog.String("error", err.Error()))
				return
			}

			writer, err := outputClient.GetWriter(outputName, inputMetadata)
			if err != nil {
				fileLogger.Warn("fail to get writer for output file", slog.String("error", err.Error()))
				return
			}

			if err := conv.Process(inputMetadata.ContentType, reader, writer); err != nil {
				fileLogger.Warn("fail to convert file", slog.String("error", err.Error()))
				return
			}

			fileLogger.Info("successfully processed file", slog.String("input_hash", inputMetadata.Hash))
		}(i, file)
	}

	wg.Wait()

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
		generalLogger.Info("all files processed successfully, exiting")
		os.Exit(0)
	}
}
