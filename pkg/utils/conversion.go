package utils

func ConvertToFloat32(f []float64) []float32 {
	out := make([]float32, len(f))
	for i, v := range f {
		out[i] = float32(v)
	}
	return out
}

func ConvertToFloat64(f float32) float64 {
	return float64(f)
}
