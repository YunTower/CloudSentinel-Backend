package services

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestCalculateUptimeFormatsDurationsAndRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{int64(0), "0分"}, {float64(59), "0分"}, {int64(3600 + 120), "1小时 2分"},
		{int64(86400 + 2*3600 + 3*60), "1天 2小时 3分"}, {int64(-1), "0分"}, {"bad", "0分"},
	}
	for _, tc := range tests {
		if got := CalculateUptime(tc.input); got != tc.want {
			t.Errorf("%v => %q want %q", tc.input, got, tc.want)
		}
	}
	past := time.Now().Add(-65 * time.Minute)
	if got := CalculateUptime(past); got != "1小时 5分" && got != "1小时 4分" {
		t.Fatalf("time uptime=%q", got)
	}
	var nilTime *time.Time
	if got := CalculateUptime(nilTime); got != "0分" {
		t.Fatalf("nil time=%q", got)
	}
}

func TestFormatMetricValueRoundsSupportedNumericTypes(t *testing.T) {
	if got := FormatMetricValue(12.345); got != 12.35 {
		t.Fatalf("got=%v", got)
	}
	if got := FormatMetricValue(int64(12)); got != 12 {
		t.Fatalf("got=%v", got)
	}
	if got := FormatMetricValue("12"); got != 0 {
		t.Fatalf("got=%v", got)
	}
}

func TestNumericConversionHelpersHandleJSONAndOverflow(t *testing.T) {
	if toString(nil) != "" || toString(12) != "12" {
		t.Fatal("string conversion mismatch")
	}
	if toFloat64(json.Number("12.5")) != 12.5 || toFloat64(float32(3.5)) != 3.5 || toFloat64("bad") != 0 {
		t.Fatal("float conversion mismatch")
	}
	if toInt64(json.Number("12")) != 12 || toInt64(float64(3.9)) != 3 || toInt64(uint64(math.MaxUint64)) != math.MaxInt64 {
		t.Fatal("int conversion mismatch")
	}
	if n, ok := toPositiveInt(3); !ok || n != 3 {
		t.Fatal("positive int mismatch")
	}
	if _, ok := toPositiveInt(0); ok {
		t.Fatal("zero should fail")
	}
	if _, ok := toPositiveInt(nil); ok {
		t.Fatal("nil should fail")
	}
}
