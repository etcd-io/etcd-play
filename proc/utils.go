package proc

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"
)

func randBytes(bytesN int) []byte {
	const (
		letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		letterIdxBits = 6                    // 6 bits to represent a letter index
		letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
		letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	)
	src := rand.NewSource(time.Now().UnixNano())
	b := make([]byte, bytesN)
	for i, cache, remain := bytesN-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return b
}

func multiRandBytes(bytesN, sliceN int) [][]byte {
	m := make(map[string]struct{})
	var rs [][]byte
	for len(rs) != sliceN {
		b := randBytes(bytesN)
		if _, ok := m[string(b)]; !ok {
			rs = append(rs, b)
			m[string(b)] = struct{}{}
		} else {
			continue
		}
	}
	return rs
}

func mapToCommaString(m map[string]struct{}) string {
	if len(m) == 0 {
		return ""
	}
	var ss []string
	for k := range m {
		ss = append(ss, k)
	}
	sort.Strings(ss)
	return strings.TrimSpace(strings.Join(ss, ","))
}

func mapToMapString(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	var ss []string
	for k, v := range m {
		val := fmt.Sprintf("%s=%s", k, v)
		ss = append(ss, val)
	}
	sort.Strings(ss)
	return strings.TrimSpace(strings.Join(ss, ","))
}

func openToAppend(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_APPEND, 0777)
	if err != nil {
		f, err = os.Create(fpath)
		if err != nil {
			return f, err
		}
	}
	return f, nil
}
