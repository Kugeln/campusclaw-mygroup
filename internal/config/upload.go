package config

import (
	"errors"
	"strconv"
)

func UploadLimit(get func(string) string) (int64, error) {
	v := get("MAX_UPLOAD_BYTES")
	if v == "" {
		return 20 * 1024 * 1024, nil
	}
	n, e := strconv.ParseInt(v, 10, 64)
	// Leave space for bounded multipart framing and the overflow sentinel.
	if e != nil || n <= 0 || n > int64(^uint64(0)>>1)-(1<<20)-1 {
		return 0, errors.New("invalid MAX_UPLOAD_BYTES")
	}
	return n, nil
}
