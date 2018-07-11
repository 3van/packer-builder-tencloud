package tencloud

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/3van/tencloud-go"

	"github.com/hashicorp/packer/packer"
)

type Artifact struct {
	Images         map[string]string
	BuilderIdValue string
	Session        *tcapi.Client
}

func (a Artifact) BuilderId() string {
	return a.BuilderIdValue
}

func (a Artifact) Files() []string {
	return nil
}

func (a Artifact) Id() string {
	parts := make([]string, 0, len(a.Images))
	for region, image := range a.Images {
		parts = append(parts, fmt.Sprintf("%s:%s", region, image))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (a Artifact) String() string {
	parts := make([]string, 0, len(a.Images))
	for region, image := range a.Images {
		parts = append(parts, fmt.Sprintf("%s: %s", region, image))
	}
	sort.Strings(parts)
	return fmt.Sprintf("Images were created:\n%s\n", strings.Join(parts, "\n"))
}

func (a Artifact) State(name string) interface{} {
	switch name {
	case "atlas.artifact.metadata":
		return a.stateAtlasMetadata()
	default:
		return nil
	}
}

func (a Artifact) Destroy() error {
	errors := make([]error, 0)
	for region, imageId := range a.Images {
		log.Printf("deleting image '%s' from region '%s'", imageId, region)
		thisClient := a.Session.Copy(region, nil)
		req := &tcapi.DeleteImagesRequest{
			ImageIds: []string{
				imageId,
			},
		}
		if err := thisClient.DeleteImages(req); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		if len(errors) == 1 {
			return errors[0]
		} else {
			return &packer.MultiError{Errors: errors}
		}
	}

	return nil
}

func (a *Artifact) stateAtlasMetadata() interface{} {
	metadata := make(map[string]string)
	for region, imageId := range a.Images {
		k := fmt.Sprintf("region.%s", region)
		metadata[k] = imageId
	}

	return metadata
}
