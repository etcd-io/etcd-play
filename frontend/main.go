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
	buf.WriteString(`package commands

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
	if err := toFile(txt, filepath.Join(os.Getenv("GOPATH"), "src/github.com/coreos/etcd-play/commands/static.go")); err != nil {
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
