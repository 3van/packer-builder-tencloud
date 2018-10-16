package tencloud

import (
	"testing"

	"github.com/hashicorp/packer/packer"
)

func testConfig() map[string]interface{} {
	return map[string]interface{}{
		"key_id":        "foo",
		"key":           "bar",
		"instance_type": "type_foo",
		"subnet_id":     "subnet_bar",
	}
}

func TestBuilder_ImplementsBuilder(t *testing.T) {
	var raw interface{}
	raw = &Builder{}
	if _, ok := raw.(packer.Builder); !ok {
		t.Fatalf("Builder should be a builder")
	}
}

func TestBuilderPrepare_workingConfig(t *testing.T) {
	var b Builder
	config := testConfig()
	config["source_image_id"] = "foo"

	warnings, err := b.Prepare(config)
	if len(warnings) > 0 {
		t.Fatalf("bad: %#v", warnings)
	}
	if err != nil {
		t.Fatalf("prepare should not have error: %v", err)
	}
}

func TestBuilderPrepare_sourceImage(t *testing.T) {
	// Both sources set, fail
	var b Builder
	config := testConfig()
	config["source_image_id"] = "foo"
	config["source_image_filters"] = map[string]interface{}{
		"most_recent": true,
		"filters": map[string]string{
			"foo-filter": "foo-filter-value",
		},
	}
	warnings, err := b.Prepare(config)
	if len(warnings) > 0 {
		t.Fatalf("bad: %#v", warnings)
	}
	if err == nil {
		t.Fatal("should have errored")
	}

	// only one source set, pass
	delete(config, "source_image_id")
	b = Builder{}
	warnings, err = b.Prepare(config)
	if len(warnings) > 0 {
		t.Fatalf("bad: %#v", warnings)
	}
	if err != nil {
		t.Fatalf("should not have error: %v", err)
	}

	// source_image_filter has no filters should fail
	delete(config, "source_image_filters")
	config["source_image_filters"] = map[string]interface{}{
		"most_recent": true,
	}
	b = Builder{}
	warnings, err = b.Prepare(config)
	if len(warnings) > 0 {
		t.Fatalf("bad: %#v", warnings)
	}
	if err == nil {
		t.Fatal("should have errored")
	}
}
