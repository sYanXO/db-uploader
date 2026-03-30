package main

import "testing"

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		workers          int
		batchSize        int
		progressInterval int
		wantErr          bool
	}{
		{name: "valid", workers: 1, batchSize: 1, progressInterval: 0, wantErr: false},
		{name: "zero workers", workers: 0, batchSize: 1, progressInterval: 0, wantErr: true},
		{name: "negative workers", workers: -1, batchSize: 1, progressInterval: 0, wantErr: true},
		{name: "zero batch", workers: 1, batchSize: 0, progressInterval: 0, wantErr: true},
		{name: "negative progress", workers: 1, batchSize: 1, progressInterval: -1, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateConfig(tt.workers, tt.batchSize, tt.progressInterval, 0, 0, 0, 0)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
