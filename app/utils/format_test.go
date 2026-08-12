package utils

import "testing"

func TestFormatStorageSizeUsesBinaryUnitsAndStablePrecision(t *testing.T) {
	tests := map[int64]string{
		0: "", 512: "0.5KB", 1024: "1.0KB", 1536: "1.5KB",
		1024 * 1024: "1.0MB", 1024 * 1024 * 1024: "1.0GB",
		1024 * 1024 * 1024 * 1024: "1.0TB",
	}
	for input, want := range tests {
		if got := FormatStorageSize(input); got != want {
			t.Errorf("%d => %q, want %q", input, got, want)
		}
	}
}
