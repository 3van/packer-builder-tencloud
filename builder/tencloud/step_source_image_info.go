package tencloud

import (
	"context"
	"fmt"

	"github.com/3van/tencloud-go"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepSourceImageInfo struct {
	SourceImage       string
	SourceImageFilter TagFilterOptions
}

func (step *StepSourceImageInfo) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	tc := state.Get("tc").(*tcapi.Client)

	if step.SourceImageFilter.Empty() {
		req := &tcapi.DescribeImagesRequest{
			ImageIds: []string{
				step.SourceImage,
			},
		}
		resp, err := tc.DescribeImages(req)
		if err != nil {
			state.Put("error", fmt.Errorf("error querying source image: %s", err))
			return multistep.ActionHalt
		}
		if len(resp.ImageSet) < 1 {
			state.Put("error", fmt.Errorf("no AMI '%s' was found", step.SourceImage))
			return multistep.ActionHalt
		}
		state.Put("source_image", resp.ImageSet[0])
		return multistep.ActionContinue
	}

	ui.Say("discovering source image from filters")
	image, err := step.SourceImageFilter.FindImage(tc)
	if err != nil {
		state.Put("error", fmt.Errorf("Could not find source image given filters: %v", err))
		return multistep.ActionHalt
	}
	ui.Say("using discovered image id: " + image.ImageId)
	state.Put("source_image", *image)
	return multistep.ActionContinue
}

func (step *StepSourceImageInfo) Cleanup(_ multistep.StateBag) {
	return
}
