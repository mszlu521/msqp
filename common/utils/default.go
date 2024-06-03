package utils

func Default(v, d string) string {
	if len(v) == 0 {
		return d
	}
	return v
}
