package main

import "math"

func actionIntToString(action int) string {
	switch action {
	case 0:
		return "NONE"
	case 1:
		return "CHCK"
	case 2:
		return "CALL"
	case 3:
		return "FOLD"
	case 4:
		return "BETT"
	case 5:
		return "LEFT"
	default:
		return "UNKNOWN"
	}
}

func countDigits(n int) int {
	if n == 0 {
		return 1
	}
	absNum := math.Abs(float64(n))
	log10Floor := math.Floor(math.Log10(absNum)) + 1
	digitCount := int(log10Floor)
	if n < 1 {
		digitCount += 1
	}
	return digitCount
}
