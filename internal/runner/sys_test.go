package runner

import "testing"

func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{name: "a greater", a: 10, b: 5, expected: 10},
		{name: "b greater", a: 3, b: 7, expected: 7},
		{name: "equal values", a: 4, b: 4, expected: 4},
		{name: "negative values, a greater", a: -1, b: -5, expected: -1},
		{name: "negative values, b greater", a: -10, b: -3, expected: -3},
		{name: "zero and positive", a: 0, b: 1, expected: 1},
		{name: "zero and negative", a: 0, b: -1, expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := max(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
