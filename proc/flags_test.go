package proc

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFlagsTag(t *testing.T) {
	df := defaultFlags()
	field, ok := reflect.TypeOf(df).Elem().FieldByName("Name")
	if !ok {
		t.Error("Name field not found")
	}
	if string(field.Tag.Get("flag")) != "name" {
		t.Errorf("expected 'name' but got %s", string(field.Tag))
	}
}

func TestGenerateFlags(t *testing.T) {
	df, err := GenerateFlags("etcd1", "", false, nil)
	if err != nil {
		t.Error(err)
	}
	sf, err := df.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sf, `--name='etcd1' --experimental-v3demo='true' --experimental-gRPC-addr='localhost:`) {
		t.Errorf("wrong FlagString from %s", sf)
	}
}

func TestGetPairValueByName(t *testing.T) {
	df, err := GenerateFlags("etcd1", "", false, nil)
	if err != nil {
		t.Error(err)
	}
	sf, err := df.String()
	if err != nil {
		t.Fatal(err)
	}
	rs := getPairValueByName("ExperimentalgRPCAddr", sf)
	if !strings.HasPrefix(rs, "localhost:") {
		t.Errorf("expected 'localhost:*' but got %s", rs)
	}
}

func TestCombineFlags(t *testing.T) {
	fs := make([]*Flags, 5)
	for i := range fs {
		df, err := GenerateFlags(fmt.Sprintf("etcd%d", i), "", false, nil)
		if err != nil {
			t.Error(err)
		}
		fs[i] = df
	}
	if err := CombineFlags(false, fs...); err != nil {
		t.Error(err)
	}
	fa := []*Flags{fs[0], fs[0]}
	if err := CombineFlags(false, fa...); err == nil {
		t.Error(err)
	}
}

func TestGetAllPorts(t *testing.T) {
	df, err := GenerateFlags("etcd1", "", false, nil)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(df.getAllPorts())
}
