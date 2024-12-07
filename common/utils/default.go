package utils

func Default[T int | string](v, d T) T {
	if !isZero(v) {
		return v
	}
	return d
}

func isZero[T comparable](v T) bool {
	var zero T
	return v == zero
}
