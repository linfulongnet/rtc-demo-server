package utils

import (
	"time"
)

const mask = "2006-01-02 15:04:05"

func NowString() string {
	return time.Now().Format(mask)
}

func NowUtcString() string {
	return time.Now().UTC().Format(mask)
}
