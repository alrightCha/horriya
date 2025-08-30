package utils

import "time"

func Now() int64 {
	now := time.Now() // current local time
	var sec int64 = now.Unix()
	return sec
}
