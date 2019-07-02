package utils

import (
	"strings"
)

func SmallHumpNamed(name string) string {
	strArr := strings.Split(name, "_")
	str := ""
	str += strings.ToLower(strArr[0])

	for i := 1; i < len(strArr); i++ {
		str += strings.ToUpper(strArr[i])
	}

	return str
}

func FirstLetterToLower(str string) string {
	firstLetter := strings.ToLower(Substr(str, 0, 1))
	return firstLetter + Substr(str, 1, len(str))
}

// 截取字符串 start to end
func Substr(str string, start int, end int) string {
	r := []rune(str)
	length := len(r)

	if start < 0 || start > length || start > end {
		return ""
	}

	if end > length {
		end = length
	}

	return string(r[start:end])
}
