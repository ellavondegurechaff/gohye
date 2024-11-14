package utils

import (
	"strconv"
)

func FormatNumber(n int64) string {
	str := strconv.FormatInt(n, 10)
	if n < 0 {
		str = str[1:] // Remove minus sign for processing
	}

	var result []byte
	for i := len(str) - 1; i >= 0; i-- {
		if (len(str)-i-1)%3 == 0 && i != len(str)-1 {
			result = append([]byte{','}, result...)
		}
		result = append([]byte{str[i]}, result...)
	}

	if n < 0 {
		return "-" + string(result)
	}
	return string(result)
}
