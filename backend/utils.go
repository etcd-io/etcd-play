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

package backend

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

func urlToName(s string) string {
	ss := strings.Split(s, "_")
	suffix := ss[len(ss)-1]
	return "etcd" + suffix
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
	ip = strings.Replace(ip, ".", "", -1)
	ua := req.UserAgent()
	return ip + "_" + simpleUA(ua) + "_" + hashSha512(ip + ua)[:15]
}

// simpleUA simple parses user agent for user ID generation.
func simpleUA(ua string) string {
	us := ""
	raw := strings.ToLower(ua)

	// OS
	if strings.Contains(raw, "linux") {
		us += "linux"
	} else if strings.Contains(raw, "macintosh") || strings.Contains(raw, "mac os") {
		us += "mac"
	} else if strings.Contains(raw, "windows") {
		us += "window"
	} else if strings.Contains(raw, "iphone") {
		us += "iphone"
	} else if strings.Contains(raw, "android") {
		us += "android"
	} else {
		us += "unknown"
	}

	us += "_"

	// browser
	if strings.Contains(raw, "firefox/") && !strings.Contains(raw, "seammonkey/") {
		us += "firefox"
	} else if strings.Contains(raw, ";msie") {
		us += "ie"
	} else if strings.Contains(raw, "safari/") && !strings.Contains(raw, "chrome/") && !strings.Contains(raw, "chromium/") {
		us += "safari"
	} else if strings.Contains(raw, "chrome/") || strings.Contains(raw, "chromium/") {
		us += "chrome"
	}

	return us
}
