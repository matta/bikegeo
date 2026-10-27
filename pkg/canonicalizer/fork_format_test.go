package canonicalizer_test

import (
	"testing"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/model"
)

func TestFormatRigidFork(t *testing.T) {
	tests := []struct {
		a2c  float64
		want string
	}{
		{483.0, "Rigid (483mm)"},
		{495.2, "Rigid (495mm)"},
		{440.0, "Rigid (440mm)"},
	}
	for _, tc := range tests {
		got := canonicalizer.FormatRigidFork(tc.a2c)
		if got != tc.want {
			t.Errorf("FormatRigidFork(%.1f) = %q, want %q", tc.a2c, got, tc.want)
		}
	}
}

func TestFormatSuspensionFork(t *testing.T) {
	tests := []struct {
		travel float64
		sag    float64
		want   string
	}{
		{110.0, 20.0, "110mm @ 20% sag"},
		{120.0, 25.0, "120mm @ 25% sag"},
		{100.0, 15.0, "100mm @ 15% sag"},
	}
	for _, tc := range tests {
		got := canonicalizer.FormatSuspensionFork(tc.travel, tc.sag)
		if got != tc.want {
			t.Errorf("FormatSuspensionFork(%.1f, %.1f) = %q, want %q", tc.travel, tc.sag, got, tc.want)
		}
	}
}

func TestFormatCustomA2cFork(t *testing.T) {
	got := canonicalizer.FormatCustomA2cFork(506.0, 44.0)
	want := "a2c=506/44"
	if got != want {
		t.Errorf("FormatCustomA2cFork(506, 44) = %q, want %q", got, want)
	}
}

func TestFormatStockFork(t *testing.T) {
	if got := canonicalizer.FormatStockFork(); got != "Stock" {
		t.Errorf("FormatStockFork() = %q, want %q", got, "Stock")
	}
}

func TestIsRigidFork(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{"Rigid (483mm)", true},
		{"Rigid (483mm A2C)", true},
		{"Rigid", true},
		{"rigid", true},
		{"RIGID (495mm)", true},
		{"0mm", true},
		{"0", true},
		{"110mm @ 20% sag", false},
		{"120mm", false},
		{"Stock", false},
		{"a2c=506/44", false},
		{"", false},
	}
	for _, tc := range tests {
		got := canonicalizer.IsRigidFork(tc.label)
		if got != tc.want {
			t.Errorf("IsRigidFork(%q) = %v, want %v", tc.label, got, tc.want)
		}
	}
}

func TestFormatForkEffA2c(t *testing.T) {
	tests := []struct {
		name string
		row  model.CanonicalComparisonRow
		want string
	}{
		{
			name: "rigid fork with A2C in DisplayLabel",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "Rigid (483mm)",
				EffectiveA2c: 483,
			},
			want: "Rigid (483mm)",
		},
		{
			name: "rigid fork with legacy A2C suffix in DisplayLabel",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "Rigid (483mm A2C)",
				EffectiveA2c: 483,
			},
			want: "Rigid (483mm)",
		},
		{
			name: "rigid fork bare label",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "Rigid",
				EffectiveA2c: 495,
			},
			want: "Rigid (495mm)",
		},
		{
			name: "rigid fork lowercase",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "rigid",
				EffectiveA2c: 483,
			},
			want: "Rigid (483mm)",
		},
		{
			name: "rigid fork from Setup.DisplayLabel fallback",
			row: model.CanonicalComparisonRow{
				Setup: model.ComparisonSetup{
					DisplayLabel: "Rigid (440mm)",
				},
				EffectiveA2c: 440,
			},
			want: "Rigid (440mm)",
		},
		{
			name: "suspension fork with sag",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "110mm @ 20% sag",
				EffectiveA2c: 499.2,
			},
			want: "110mm @ 20% sag (499mm)",
		},
		{
			name: "suspension fork short label",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "100mm @ 20%",
				EffectiveA2c: 486,
			},
			want: "100mm @ 20% (486mm)",
		},
		{
			name: "label already containing formatted effective A2C",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "Custom Fork (500mm)",
				EffectiveA2c: 500,
			},
			want: "Custom Fork (500mm)",
		},
		{
			name: "empty label falls back to A2C",
			row: model.CanonicalComparisonRow{
				EffectiveA2c: 483,
			},
			want: "483mm A2C",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalizer.FormatForkEffA2c(tc.row)
			if got != tc.want {
				t.Errorf("FormatForkEffA2c() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatForkFlow(t *testing.T) {
	tests := []struct {
		name string
		row  model.CanonicalComparisonRow
		want string
	}{
		{
			name: "rigid fork flow",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "Rigid (483mm)",
				EffectiveA2c: 483,
			},
			want: "Rigid (483mm)",
		},
		{
			name: "suspension fork flow",
			row: model.CanonicalComparisonRow{
				DisplayLabel: "110mm @ 20% sag",
				EffectiveA2c: 499.2,
			},
			want: "110mm @ 20% sag -> 499mm A2C",
		},
		{
			name: "empty label flow",
			row: model.CanonicalComparisonRow{
				EffectiveA2c: 506,
			},
			want: "506mm A2C",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalizer.FormatForkFlow(tc.row)
			if got != tc.want {
				t.Errorf("FormatForkFlow() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatForkTypeAndSag(t *testing.T) {
	rigidRow := model.CanonicalComparisonRow{
		DisplayLabel: "Rigid (483mm)",
		EffectiveA2c: 483,
	}
	if got := canonicalizer.FormatForkType(rigidRow); got != "rigid" {
		t.Errorf("FormatForkType(rigid) = %q, want %q", got, "rigid")
	}
	if got := canonicalizer.FormatForkSag(rigidRow); got != "rigid" {
		t.Errorf("FormatForkSag(rigid) = %q, want %q", got, "rigid")
	}

	suspRow := model.CanonicalComparisonRow{
		DisplayLabel: "110mm @ 20% sag",
		EffectiveA2c: 499,
	}
	if got := canonicalizer.FormatForkType(suspRow); got != "110mm" {
		t.Errorf("FormatForkType(susp) = %q, want %q", got, "110mm")
	}
	if got := canonicalizer.FormatForkSag(suspRow); got != "110mm @ 20%" {
		t.Errorf("FormatForkSag(susp) = %q, want %q", got, "110mm @ 20%")
	}
}
