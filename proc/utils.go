// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proc

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
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

// gracefulClose drains http.Response.Body until it hits EOF
// and closes it. This prevents TCP/TLS connections from closing,
// therefore available for reuse.
func gracefulClose(resp *http.Response) {
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
}
