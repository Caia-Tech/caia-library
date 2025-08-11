package logging

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LogConfig holds logging configuration
type LogConfig struct {
	Level      string `json:"level"`      // debug, info, warn, error
	Format     string `json:"format"`     // json, pretty
	OutputFile string `json:"output_file"` // file path for logs
	MaxSize    int64  `json:"max_size"`   // max file size in bytes
	Console    bool   `json:"console"`    // also log to console
}

// DefaultLogConfig returns sensible defaults
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:      "info",
		Format:     "json",
		OutputFile: "logs/caia-library.log",
		MaxSize:    100 * 1024 * 1024, // 100MB
		Console:    true,
	}
}

// SetupLogger configures the global logger
func SetupLogger(config *LogConfig) error {
	// Parse log level
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(level)

	var writers []io.Writer

	// Console output
	if config.Console {
		if config.Format == "pretty" {
			writers = append(writers, zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: time.RFC3339,
				NoColor:    false,
			})
		} else {
			writers = append(writers, os.Stdout)
		}
	}

	// File output
	if config.OutputFile != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(config.OutputFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		logFile, err := os.OpenFile(config.OutputFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		writers = append(writers, logFile)
	}

	// Set up multi-writer
	if len(writers) > 1 {
		log.Logger = zerolog.New(io.MultiWriter(writers...)).With().Timestamp().Logger()
	} else if len(writers) == 1 {
		log.Logger = zerolog.New(writers[0]).With().Timestamp().Logger()
	}

	// Log startup
	log.Info().
		Str("level", config.Level).
		Str("format", config.Format).
		Str("output_file", config.OutputFile).
		Bool("console", config.Console).
		Msg("Logger initialized")

	return nil
}

// GetLogger returns a contextual logger
func GetLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// GetPipelineLogger returns a logger for pipeline operations
func GetPipelineLogger(pipeline, stage string) zerolog.Logger {
	return log.With().
		Str("pipeline", pipeline).
		Str("stage", stage).
		Logger()
}

// GetStorageLogger returns a logger for storage operations  
func GetStorageLogger(operation, backend string) zerolog.Logger {
	return log.With().
		Str("storage_operation", operation).
		Str("backend", backend).
		Logger()
}

// GetWorkflowLogger returns a logger for workflow operations
func GetWorkflowLogger(workflowID, activityName string) zerolog.Logger {
	return log.With().
		Str("workflow_id", workflowID).
		Str("activity", activityName).
		Logger()
}