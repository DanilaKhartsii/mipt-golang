package main

import (
	"testing"
	"time"
)

func TestRequestTimeoutFromEnv(t *testing.T) {
	tests := []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{value: "", want: 2 * time.Second},
		{value: "50ms", want: 50 * time.Millisecond},
		{value: "3s", want: 3 * time.Second},
		{value: "0s", wantErr: true},
		{value: "-1s", wantErr: true},
		{value: "invalid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("REQUEST_TIMEOUT", tt.value)
			got, err := requestTimeoutFromEnv()
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("requestTimeoutFromEnv() = %v, %v; want %v, error %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
