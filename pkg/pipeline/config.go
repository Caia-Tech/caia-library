package pipeline

import (
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/logging"
)

// PipelineConfig holds complete pipeline configuration
type PipelineConfig struct {
	// Logging configuration
	Logging *logging.LogConfig `json:"logging"`
	
	// Storage configuration
	Storage *storage.HybridStorageConfig `json:"storage"`
	
	// Processing configuration
	Processing *ProcessingConfig `json:"processing"`
	
	// Server configuration
	Server *ServerConfig `json:"server"`
	
	// Data paths
	DataPaths *DataPathsConfig `json:"data_paths"`
}

// ProcessingConfig holds processing pipeline settings
type ProcessingConfig struct {
	// Extraction settings
	MaxFileSize    int64         `json:"max_file_size"`     // bytes
	OCRLanguage    string        `json:"ocr_language"`      // tesseract language
	PDFMaxPages    int           `json:"pdf_max_pages"`     // max pages to process
	EnableOCR      bool          `json:"enable_ocr"`        // enable OCR processing
	
	// Quality settings
	MinQualityScore float64      `json:"min_quality_score"` // minimum quality threshold
	
	// Timeout settings
	ExtractionTimeout time.Duration `json:"extraction_timeout"`
	EmbeddingTimeout  time.Duration `json:"embedding_timeout"`
	StorageTimeout    time.Duration `json:"storage_timeout"`
	
	// Concurrency
	MaxWorkers    int `json:"max_workers"`    // parallel processing workers
	BatchSize     int `json:"batch_size"`     // batch processing size
}

// ServerConfig holds server settings
type ServerConfig struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	MaxRequestSize int64         `json:"max_request_size"`
}

// DataPathsConfig holds all data directory paths
type DataPathsConfig struct {
	// Root data directory
	DataRoot string `json:"data_root"`
	
	// Storage paths
	GitRepo     string `json:"git_repo"`
	GovcData    string `json:"govc_data"`
	IndexPath   string `json:"index_path"`
	
	// Log paths
	LogDir      string `json:"log_dir"`
	
	// Temporary paths
	TempDir     string `json:"temp_dir"`
	UploadDir   string `json:"upload_dir"`
	
	// Backup paths
	BackupDir   string `json:"backup_dir"`
}

// DefaultPipelineConfig returns a complete default configuration
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		Logging: &logging.LogConfig{
			Level:      "info",
			Format:     "json",
			OutputFile: "logs/caia-library.log",
			MaxSize:    100 * 1024 * 1024, // 100MB
			Console:    true,
		},
		
		Storage: &storage.HybridStorageConfig{
			PrimaryBackend:   "govc",
			EnableFallback:   true,
			OperationTimeout: 30 * time.Second,
			EnableSync:       true,
			SyncInterval:     5 * time.Minute,
		},
		
		Processing: &ProcessingConfig{
			MaxFileSize:       50 * 1024 * 1024, // 50MB
			OCRLanguage:       "eng",
			PDFMaxPages:       1000,
			EnableOCR:         true,
			MinQualityScore:   0.3,
			ExtractionTimeout: 5 * time.Minute,
			EmbeddingTimeout:  2 * time.Minute,
			StorageTimeout:    1 * time.Minute,
			MaxWorkers:        4,
			BatchSize:         10,
		},
		
		Server: &ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxRequestSize: 100 * 1024 * 1024, // 100MB
		},
		
		DataPaths: &DataPathsConfig{
			DataRoot:   "./data",
			GitRepo:    "./data/caia-repo",
			GovcData:   "./data/govc-storage",
			IndexPath:  "./data/indexes",
			LogDir:     "./logs",
			TempDir:    "./data/temp",
			UploadDir:  "./data/uploads", 
			BackupDir:  "./data/backups",
		},
	}
}

// ProductionPipelineConfig returns production-ready configuration
func ProductionPipelineConfig() *PipelineConfig {
	config := DefaultPipelineConfig()
	
	// Production logging
	config.Logging.Level = "info"
	config.Logging.Format = "json"
	config.Logging.Console = false
	
	// Production storage
	config.Storage.EnableSync = true
	config.Storage.SyncInterval = 1 * time.Minute
	
	// Production processing
	config.Processing.MaxWorkers = 8
	config.Processing.BatchSize = 20
	
	return config
}

// DevelopmentPipelineConfig returns development configuration
func DevelopmentPipelineConfig() *PipelineConfig {
	config := DefaultPipelineConfig()
	
	// Development logging
	config.Logging.Level = "debug"
	config.Logging.Format = "pretty"
	config.Logging.Console = true
	
	// Development storage
	config.Storage.EnableSync = false
	
	// Development processing
	config.Processing.MaxWorkers = 2
	config.Processing.BatchSize = 5
	
	return config
}