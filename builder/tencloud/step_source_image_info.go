package tencloud

import (
	"context"
	"fmt"

	"github.com/3van/tencloud-go"

	"github.com/hashicorp/packer/helper/multistep"
)

type StepSourceImageInfo struct {
	SourceImage string
}

func (step *StepSourceImageInfo) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	tc := state.Get("tc").(*tcapi.Client)

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

func (step *StepSourceImageInfo) Cleanup(_ multistep.StateBag) {
	return
}
