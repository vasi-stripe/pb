package diff

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// Given a directory name and a .proto file, generate a FileDescriptorSet.
//
// Requires protoc to be installed.
func generateFileSet(t *testing.T, prefix string, names ...string) descriptor.FileDescriptorSet {
	var fds descriptor.FileDescriptorSet
	protoDir := filepath.Join("testdata", prefix)

	protoFiles := []string{}
	for _, name := range names {
		protoFiles = append(protoFiles, filepath.Join(protoDir, name+".proto"))
	}

	fdsDir := filepath.Join("testdata", prefix+"_fds")
	fdsFile := filepath.Join(fdsDir, strings.Join(names, "_")+".fds")
	if err := os.MkdirAll(fdsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Run protoc
	cmdArgs := append([]string{"-o", fdsFile}, protoFiles...)
	cmd := exec.Command("protoc", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %s %s", err, out)
	}
	blob, err := ioutil.ReadFile(fdsFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := proto.Unmarshal(blob, &fds); err != nil {
		t.Fatalf("parsing prev proto: %s", err)
	}
	return fds
}

func TestDiffing(t *testing.T) {
	files := map[string]string{
		"changed_client_streaming":   "changed client streaming for method 'Invoke' on service 'Foo': false -> true",
		"changed_server_streaming":   "changed server streaming for method 'Invoke' on service 'Foo': true -> false",
		"changed_enum_value":         "changed value 'bat' on enum 'FOO': 1 -> 2",
		"changed_field_label":        "changed label for field 'name' on message 'HelloRequest': LABEL_OPTIONAL -> LABEL_REPEATED",
		"changed_field_name":         "changed name for field #1 on message 'HelloRequest': foo -> bar",
		"changed_field_type":         "changed types for field 'name' on message 'HelloRequest': TYPE_STRING -> TYPE_BOOL",
		"changed_field_type_message": "changed types for field 'name' on message 'HelloRequest': .google.protobuf.StringValue -> .google.protobuf.Int64Value",
		"changed_package":            "removed package 'foo'",
		"changed_service_input":      "changed input type for method 'Invoke' on service 'Foo': .helloworld.FooRequest -> .helloworld.BarRequest",
		"changed_service_output":     "changed output type for method 'Invoke' on service 'Foo': .helloworld.FooResponse -> .helloworld.BarResponse",
		"removed_enum":               "removed enum 'FOO'",
		"removed_enum_field":         "removed value 'bat' from enum 'FOO'",
		"removed_field":              "removed field 'name' from message 'HelloRequest'",
		"removed_message":            "removed message 'HelloRequest'",
		"removed_service":            "removed service 'Foo'",
		"removed_service_method":     "removed method 'Bar' from service 'Foo'",
		"unreserved_name":            "un-reserved field name 'name' from message 'HelloRequest'",
		"unreserved_number":          "un-reserved field number(s) in range 1 to 3 from message 'HelloRequest'",
		"changed_enum_name":          "changed name of field #1 on enum 'FOO': foo -> bat",
	}
	for name, problem := range files {
		t.Run(name, func(t *testing.T) {
			prev := generateFileSet(t, "previous", name)
			curr := generateFileSet(t, "current", name)
			// Won't work with --include_imports
			report, err := DiffSet(&prev, &curr)
			if err == nil {
				t.Fatal("expected diff to have an error")
			}
			if len(report.Changes) == 0 {
				t.Fatal("expected report to have at least one problem")
			}
			if len(report.Changes) > 1 {
				t.Errorf("expected report to have one problem, has %d: %v", len(report.Changes), report)
			}
			if report.Changes[0].String() != problem {
				t.Errorf("expected problem: %s", problem)
				t.Errorf("  actual problem: %s", report.Changes[0].String())
			}
		})
	}

	t.Run("unchanged", func(t *testing.T) {
		files := generateFileSet(t, "current", "unchanged", "unchanged_import")
		reordered := generateFileSet(t, "current", "unchanged_import", "unchanged")
		report, err := DiffSet(&files, &reordered)
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		if len(report.Changes) != 0 {
			t.Fatalf("%d unexpected problem reports", len(report.Changes))
		}
	})

	t.Run("field removal with name & number reserved", func(t *testing.T) {
		current := generateFileSet(t, "current", "removed_field_but_reserved")
		previous := generateFileSet(t, "previous", "removed_field_but_reserved")
		report, err := DiffSet(&previous, &current)
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		if len(report.Changes) != 0 {
			t.Fatalf("%d unexpected problem reports", len(report.Changes))
		}
	})
}
