package handler

import (
	"fmt"
	"math"

	"github.com/HeartBtz/tempest/internal/storage"
)

const (
	maxSpeedBytesPerSecond int64   = 1 << 40
	maxTransferBytes       int64   = 1<<53 - 1
	maxTargetRatio         float64 = 1_000_000
)

func validateSpeed(field string, value int64) error {
	if value < 0 {
		return fmt.Errorf("%s must not be negative", field)
	}
	if value > maxSpeedBytesPerSecond {
		return fmt.Errorf("%s must not exceed %d bytes per second", field, maxSpeedBytesPerSecond)
	}
	return nil
}

func validateTransferLimit(field string, value int64) error {
	if value < 0 {
		return fmt.Errorf("%s must not be negative", field)
	}
	if value > maxTransferBytes {
		return fmt.Errorf("%s must not exceed %d bytes", field, maxTransferBytes)
	}
	return nil
}

func validateTargetRatio(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("target_ratio must be finite")
	}
	if value <= 0 || value > maxTargetRatio {
		return fmt.Errorf("target_ratio must be greater than 0 and at most %.0f", maxTargetRatio)
	}
	return nil
}

func validatePort(value int) error {
	if value < 1 || value > 65535 {
		return fmt.Errorf("announce port must be between 1 and 65535")
	}
	return nil
}

func validateSettings(settings *storage.Settings) error {
	if err := validateSpeed("upload_speed", settings.UploadSpeed); err != nil {
		return err
	}
	if err := validateSpeed("download_speed", settings.DownloadSpeed); err != nil {
		return err
	}
	if err := validateSpeed("speed_variance", settings.SpeedVariance); err != nil {
		return err
	}
	if err := validateTargetRatio(settings.TargetRatio); err != nil {
		return err
	}
	if err := validateTransferLimit("max_upload", settings.MaxUpload); err != nil {
		return err
	}
	return validateTransferLimit("max_download", settings.MaxDownload)
}
