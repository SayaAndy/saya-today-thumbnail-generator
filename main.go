package main

import (
	"bytes"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/SayaAndy/saya-today-thumbnail-generator/config"
	"github.com/SayaAndy/saya-today-thumbnail-generator/internal/client/input"
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

	converters := make([]converter.Converter, 0, len(cfg.Converters))
	var converterTypes []string
	for _, converterCfg := range cfg.Converters {
		conv, err := converter.NewConverterMap[converterCfg.Type](&converterCfg)
		if err != nil {
			slog.Error("fail to initialize converter", slog.String("error", err.Error()))
			os.Exit(1)
		}
		converters = append(converters, conv)
		converterTypes = append(converterTypes, converterCfg.Type)
	}

	generalLogger := slog.With(slog.String("input_storage", cfg.Input.Storage.Type))
	generalLogger.Info("initialized input client and converters", slog.String("converter_types", strings.Join(converterTypes, " ")))

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
	fileCount := len(files)
	generalLogger.Info("scanned files", slog.Int("file_count", fileCount))

	select {
	case <-sigTermChan:
		generalLogger.Info("exiting due to termination signal")
		os.Exit(130)
	default:
	}

	processSemaphore := make(chan struct{}, cfg.MaxProcessThreads)
	queueSemaphore := make(chan struct{}, cfg.MaxPreProcessThreads)
	var wg sync.WaitGroup
	wg.Add(fileCount)

	for i, file := range files {
		queueSemaphore <- struct{}{}
		go func(index int, inputName string) {
			fileLogger := generalLogger.With(slog.String("input_path", inputName), slog.Int("file_index", index))

			threadSigTermChannel := make(chan os.Signal, 1)
			signal.Notify(threadSigTermChannel, os.Interrupt, syscall.SIGTERM)

			defer func() { <-queueSemaphore; wg.Done() }()

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

			convertersToLaunch := []int{}
			for j, conv := range converters {
				outputName := conv.DeductOutputPath(inputName)
				originalInputHash := ""
				convLogger := fileLogger.With(slog.String("output_path", outputName), slog.Int("conv_index", j))

				if !cfg.ForceRewrite && !conv.IsMissing(outputName) {
					outputMetadata, err := conv.ReadMetadata(outputName)
					if err != nil {
						convLogger.Warn("fail to read metadata of (supposedly existing) output file", slog.String("error", err.Error()))
						continue
					}
					originalInputHash = outputMetadata.HashOriginal
					if inputMetadata.Hash == originalInputHash {
						convLogger.Info("skip already processed file (based on equal hash)", slog.String("input_hash", inputMetadata.Hash))
						continue
					}
				}
				convertersToLaunch = append(convertersToLaunch, j)
			}

			if len(convertersToLaunch) == 0 {
				return
			}

			processSemaphore <- struct{}{}
			defer func() { <-processSemaphore }()

			fileLogger.Info("start to process file", slog.String("input_hash", inputMetadata.Hash))
			reader, err := inputClient.GetReader(inputName)
			if err != nil {
				fileLogger.Warn("fail to get reader for input file", slog.String("error", err.Error()))
				return
			}

			fileContent := make([]byte, inputMetadata.Size)
			if _, err = reader.Read(fileContent); err != nil {
				fileLogger.Warn("fail to read content of input file", slog.String("error", err.Error()))
				return
			}
			reader.Close()

			for _, convIndex := range convertersToLaunch {
				conv := converters[convIndex]
				outputName := conv.DeductOutputPath(inputName)
				convLogger := fileLogger.With(slog.String("output_path", outputName), slog.Int("conv_index", convIndex))

				if err := conv.Process(inputMetadata, bytes.NewReader(fileContent), outputName); err != nil {
					convLogger.Warn("fail to convert file", slog.String("error", err.Error()))
					return
				}

				convLogger.Info("successfully processed file", slog.String("input_hash", inputMetadata.Hash))
			}
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
