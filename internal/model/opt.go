package model

// Opt is an optional value of type T. The zero value is unset.
type Opt[T any] struct {
	v  T
	ok bool
}

// Some returns an Opt[T] set to v.
func Some[T any](v T) Opt[T] {
	return Opt[T]{v: v, ok: true}
}

// IsSet reports whether o holds a value.
func (o Opt[T]) IsSet() bool {
	return o.ok
}

// Get returns the value and whether it is set.
func (o Opt[T]) Get() (T, bool) {
	return o.v, o.ok
}

// Or returns the value if set, or def otherwise.
func (o Opt[T]) Or(def T) T {
	if o.ok {
		return o.v
	}
	return def
}

// OrZero returns the value if set, or the zero value of T otherwise.
func (o Opt[T]) OrZero() T {
	return o.v
}

// Map applies fn to the value if set, or returns an unset Opt[T].
func (o Opt[T]) Map(fn func(T) T) Opt[T] {
	if !o.ok {
		return Opt[T]{}
	}
	return Some(fn(o.v))
}
