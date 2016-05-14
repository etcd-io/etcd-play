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
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/satori/go.uuid"
)

// Flags is a set of etcd flags.
type Flags struct {
	Name    string `flag:"name"`
	DataDir string `flag:"data-dir"`

	ListenClientURLs    map[string]struct{} `flag:"listen-client-urls"`
	AdvertiseClientURLs map[string]struct{} `flag:"advertise-client-urls"`
	ListenPeerURLs      map[string]struct{} `flag:"listen-peer-urls"`
	AdvertisePeerURLs   map[string]struct{} `flag:"initial-advertise-peer-urls"`

	InitialCluster      map[string]string `flag:"initial-cluster"`
	InitialClusterToken string            `flag:"initial-cluster-token"`
	InitialClusterState string            `flag:"initial-cluster-state"`

	ClientAutoTLS bool `flag:"auto-tls"`
	PeerAutoTLS   bool `flag:"peer-auto-tls"`
}

func defaultFlags() *Flags {
	fs := &Flags{}

	fs.DataDir = "data.etcd"

	fs.ListenClientURLs = map[string]struct{}{"http://localhost:2379": struct{}{}}
	fs.AdvertiseClientURLs = map[string]struct{}{"http://localhost:2379": struct{}{}}

	fs.ListenPeerURLs = map[string]struct{}{"http://localhost:2380": struct{}{}}
	fs.AdvertisePeerURLs = map[string]struct{}{"http://localhost:2380": struct{}{}}

	fs.InitialCluster = make(map[string]string)
	fs.InitialClusterToken = ""
	fs.InitialClusterState = "new"

	// TODO: enable auto TLS
	fs.ClientAutoTLS = false
	fs.PeerAutoTLS = false

	return fs
}

// ports between 1178 ~ 65480
var globalPortPrefix uint32 = 12

// GenerateFlags returns generated default flags.
func GenerateFlags(name, host string, remote bool) (*Flags, error) {
	portPrefix := atomic.LoadUint32(&globalPortPrefix)
	if !remote {
		atomic.AddUint32(&globalPortPrefix, 1)
	}
	if remote {
		portPrefix = 23
	}

	clientURLPort := fmt.Sprintf(":%d79", portPrefix)
	peerURLPort := fmt.Sprintf(":%d80", portPrefix)

	hs := "localhost"
	if host != "" {
		hs = host
	}
	clientURL := "http://" + hs + clientURLPort
	peerURL := "http://" + hs + peerURLPort

	fs := defaultFlags()
	fs.Name = name
	fs.DataDir = name + ".etcd"

	if fs.ClientAutoTLS {
		clientURL = strings.Replace(clientURL, "http://", "https://", -1)
	}
	if fs.PeerAutoTLS {
		peerURL = strings.Replace(peerURL, "http://", "https://", -1)
	}

	fs.ListenClientURLs = map[string]struct{}{clientURL: struct{}{}}
	fs.AdvertiseClientURLs = map[string]struct{}{clientURL: struct{}{}}

	fs.ListenPeerURLs = map[string]struct{}{peerURL: struct{}{}}
	fs.AdvertisePeerURLs = map[string]struct{}{peerURL: struct{}{}}

	return fs, nil
}

// CombineFlags combine flags under a same cluster.
func CombineFlags(remote bool, cs ...*Flags) error {
	nameToPeerURL := make(map[string]string)
	portCheck := ""
	for i := range cs {
		if _, ok := nameToPeerURL[cs[i].Name]; ok {
			return fmt.Errorf("%s is duplicate!", cs[i].Name)
		}
		tp := strings.Join(cs[i].getAllPorts(), "___")
		if portCheck == "" {
			portCheck = tp
		} else if portCheck == tp && !remote {
			return fmt.Errorf("%q has duplicate ports in another node!", cs[i].getAllPorts())
		}
		nameToPeerURL[cs[i].Name] = mapToCommaString(cs[i].AdvertisePeerURLs)
	}
	token := uuid.NewV4().String() // must be same (used for cluster-id)
	for i := range cs {
		cs[i].InitialClusterToken = token
		cs[i].InitialCluster = nameToPeerURL
	}
	return nil
}

func (f *Flags) IsValid() (bool, error) {
	if len(f.Name) == 0 {
		return false, errors.New("Name must be specified!")
	}
	if f.InitialClusterState != "new" && f.InitialClusterState != "existing" {
		return false, errors.New("InitialClusterState must be either 'new' or 'existing'.")
	}
	return true, nil
}

func (f *Flags) Pairs() ([][]string, error) {
	valid, e := f.IsValid()
	if !valid || e != nil {
		return nil, e
	}

	var pairs [][]string

	nameTag, err := f.getTag("Name")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{nameTag, strings.TrimSpace(f.Name)})

	listenClientURLsTag, err := f.getTag("ListenClientURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{listenClientURLsTag, mapToCommaString(f.ListenClientURLs)})

	advertiseClientURLsTag, err := f.getTag("AdvertiseClientURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{advertiseClientURLsTag, mapToCommaString(f.AdvertiseClientURLs)})

	listenPeerURLsTag, err := f.getTag("ListenPeerURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{listenPeerURLsTag, mapToCommaString(f.ListenPeerURLs)})

	advertisePeerURLsTag, err := f.getTag("AdvertisePeerURLs")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{advertisePeerURLsTag, mapToCommaString(f.AdvertisePeerURLs)})

	initialClusterTag, err := f.getTag("InitialCluster")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{initialClusterTag, mapToMapString(f.InitialCluster)})

	initialClusterTokenTag, err := f.getTag("InitialClusterToken")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{initialClusterTokenTag, f.InitialClusterToken})

	initialClusterStateTag, err := f.getTag("InitialClusterState")
	if err != nil {
		return nil, err
	}
	if f.InitialClusterState == "new" {
		pairs = append(pairs, []string{initialClusterStateTag, "new"})
	} else {
		pairs = append(pairs, []string{initialClusterStateTag, "existing"})
	}

	dataDirTag, err := f.getTag("DataDir")
	if err != nil {
		return nil, err
	}
	pairs = append(pairs, []string{dataDirTag, strings.TrimSpace(f.DataDir)})

	clientAutoTLSTag, err := f.getTag("ClientAutoTLS")
	if err != nil {
		return nil, err
	}
	if f.ClientAutoTLS {
		pairs = append(pairs, []string{clientAutoTLSTag, "true"})
	}

	peerAutoTLSTag, err := f.getTag("PeerAutoTLS")
	if err != nil {
		return nil, err
	}
	if f.PeerAutoTLS {
		pairs = append(pairs, []string{peerAutoTLSTag, "true"})
	}

	return pairs, nil
}

func (f *Flags) StringSlice() ([]string, error) {
	pairs, err := f.Pairs()
	if err != nil {
		return nil, err
	}
	slice := []string{}
	for _, pair := range pairs {
		if pair[1] == "true" {
			slice = append(slice, "--"+pair[0])
		} else {
			slice = append(slice, "--"+pair[0], pair[1])
		}
	}
	return slice, nil
}

func (f *Flags) String() (string, error) {
	pairs, err := f.Pairs()
	if err != nil {
		return "", err
	}
	sb := new(bytes.Buffer)
	for _, pair := range pairs {
		sb.WriteString(fmt.Sprintf("--%s='%s'", pair[0], pair[1]))
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String()), nil
}

func (f *Flags) getAllPorts() []string {
	tm := make(map[string]struct{})
	for k := range f.ListenClientURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.AdvertiseClientURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.ListenPeerURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	for k := range f.AdvertisePeerURLs {
		ss := strings.Split(k, ":")
		tm[strings.TrimSpace(ss[len(ss)-1])] = struct{}{}
	}
	sl := make([]string, len(tm))
	i := 0
	for k := range tm {
		sl[i] = k
		i++
	}
	sort.Strings(sl)
	return sl
}

func (f *Flags) getTag(fn string) (string, error) {
	field, ok := reflect.TypeOf(f).Elem().FieldByName(fn)
	if !ok {
		return "", fmt.Errorf("Field %s is not found!", fn)
	}
	return field.Tag.Get("flag"), nil
}

func getPairValueByName(fn, flags string) string {
	f := &Flags{}
	tag, _ := f.getTag(fn)
	// "--%s='%s'"
	prefix := fmt.Sprintf(`--%s='`, tag)
	rs := ""
	if strings.Contains(flags, prefix) {
		ss := strings.SplitN(flags, prefix, 2)
		if len(ss) > 1 {
			rs = strings.TrimSpace(strings.Split(ss[1], `' `)[0])
		}
	}
	return rs
}
