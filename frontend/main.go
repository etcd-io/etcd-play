package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	buf := new(bytes.Buffer)
	buf.WriteString(`// Copyright 2016 CoreOS, Inc.
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

` + "// updated at " + nowPST().String() + `

import (
	"fmt"
	"net/http"

	"golang.org/x/net/context"
)

func staticLocalHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	fmt.Fprintln(w, htmlSourceFileLocal)
	return nil
}

func staticRemoteHandler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	fmt.Fprintln(w, htmlSourceFileRemote)
	return nil
}

`)
	buf.WriteString("var htmlSourceFileLocal = `")
	f1, err := openToRead(filepath.Join(os.Getenv("GOPATH"), "src/github.com/coreos/etcd-play/frontend/local.html"))
	if err != nil {
		log.Fatal(err)
	}
	defer f1.Close()
	b1, err := ioutil.ReadAll(f1)
	if err != nil {
		log.Fatal(err)
	}
	buf.WriteString(string(b1))
	buf.WriteString("`\n\n")

	buf.WriteString("var htmlSourceFileRemote = `")
	f2, err := openToRead(filepath.Join(os.Getenv("GOPATH"), "src/github.com/coreos/etcd-play/frontend/remote.html"))
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()
	b2, err := ioutil.ReadAll(f2)
	if err != nil {
		log.Fatal(err)
	}
	buf.WriteString(string(b2))
	buf.WriteString("`\n")

	txt := buf.String()
	if err := toFile(txt, filepath.Join(os.Getenv("GOPATH"), "src/github.com/coreos/etcd-play/backend/static.go")); err != nil {
		log.Fatal(err)
	}

	if err := os.Chdir(filepath.Join(os.Getenv("GOPATH"), "src/github.com/coreos/etcd-play")); err != nil {
		log.Fatal(err)
	}
	if err := exec.Command("go", "fmt", "./...").Run(); err != nil {
		log.Fatal(err)
	}
}

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

func toFile(txt, fpath string) error {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(fpath)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	if _, err := f.WriteString(txt); err != nil {
		return err
	}
	return nil
}
