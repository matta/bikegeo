package model

import "testing"

func TestOpt(t *testing.T) {
	var unset Opt[float64]
	if unset.IsSet() {
		t.Error("zero Opt should not be set")
	}
	if v, ok := unset.Get(); ok || v != 0 {
		t.Errorf("zero Opt Get() = (%v, %v), want (0, false)", v, ok)
	}
	if got := unset.Or(42.5); got != 42.5 {
		t.Errorf("zero Opt Or(42.5) = %v, want 42.5", got)
	}
	if got := unset.OrZero(); got != 0 {
		t.Errorf("zero Opt OrZero() = %v, want 0", got)
	}
	if got := unset.Map(func(x float64) float64 { return x * 2 }); got.IsSet() {
		t.Errorf("zero Opt Map() should remain unset, got %+v", got)
	}

	set := Some(1.5)
	if !set.IsSet() {
		t.Error("Some(1.5) should be set")
	}
	if v, ok := set.Get(); !ok || v != 1.5 {
		t.Errorf("Some(1.5).Get() = (%v, %v), want (1.5, true)", v, ok)
	}
	if got := set.Or(42.5); got != 1.5 {
		t.Errorf("Some(1.5).Or(42.5) = %v, want 1.5", got)
	}
	if got := set.OrZero(); got != 1.5 {
		t.Errorf("Some(1.5).OrZero() = %v, want 1.5", got)
	}
	mapped := set.Map(func(x float64) float64 { return x * 2 })
	if v, ok := mapped.Get(); !ok || v != 3.0 {
		t.Errorf("Some(1.5).Map(*2).Get() = (%v, %v), want (3.0, true)", v, ok)
	}

	if set != Some(1.5) {
		t.Error("expected value equality between equal Opt[float64] values")
	}
	if set == unset {
		t.Error("set Opt should not equal unset Opt")
	}
	if Some(0.0) == unset {
		t.Error("Some(0.0) should not equal unset Opt")
	}
}
