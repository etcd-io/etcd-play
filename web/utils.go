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

package web

import (
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"time"
)

func nowPST() time.Time {
	tzone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.Now()
	}
	return time.Now().In(tzone)
}

func openToRead(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		return f, err
	}
	return f, nil
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

func openToOverwrite(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(fpath)
		if err != nil {
			return f, err
		}
	}
	return f, nil
}

func boldHTMLMsg(msg string) string {
	return "<br><b>[etcd-play log] " + msg + "</b><br>"
}

func getRealIP(req *http.Request) string {
	ts := []string{"X-Forwarded-For", "x-forwarded-for", "X-FORWARDED-FOR"}
	for _, k := range ts {
		if v := req.Header.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func hashSha512(s string) string {
	sum := sha512.Sum512([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func getUserID(req *http.Request) string {
	ip := getRealIP(req)
	if ip == "" {
		ip = strings.Split(req.RemoteAddr, ":")[0]
	}
	return ip + "_" + hashSha512(req.UserAgent())[:10]
}

func urlToName(s string) string {
	ss := strings.Split(s, "_")
	suffix := ss[len(ss)-1]
	return "etcd" + suffix
}
