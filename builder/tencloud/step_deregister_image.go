package tencloud

import (
	"context"
	"fmt"

	"github.com/3van/tencloud-go"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepDeregisterImage struct {
	ForceDeregister bool
	ImageName       string
	Regions         []string
}

func (step *StepDeregisterImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if !step.ForceDeregister {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packer.Ui)
	tc := state.Get("tc").(*tcapi.Client)
	config := state.Get("config").(Config)
	regions := append(step.Regions, config.Region)

	for _, region := range regions {
		thisClient := tc.Copy(region, nil)
		req := &tcapi.DescribeImagesRequest{
			Filters: []tcapi.Filter{
				{
					Name: "image-type",
					Values: []string{
						"PRIVATE_IMAGE",
					},
				},
				{
					Name: "image-name",
					Values: []string{
						step.ImageName,
					},
				},
			},
		}
		resp, err := thisClient.DescribeImages(req)
		if err != nil {
			state.Put("error", fmt.Errorf("could not query image '%s' in region '%s': %s", step.ImageName, region, err))
			return multistep.ActionHalt
		}
		for _, image := range resp.ImageSet {
			err := thisClient.DeleteImages(&tcapi.DeleteImagesRequest{
				ImageIds: []string{
					image.ImageId,
				},
			})
			if err != nil {
				state.Put("error", fmt.Errorf("could not delete image '%s' in region '%s': %s", step.ImageName, region, err))
				return multistep.ActionHalt
			}
			ui.Say(fmt.Sprintf("deleted image '%s' (ID '%s') from region '%s'", step.ImageName, image.ImageId, region))
		}
	}

	return multistep.ActionContinue
}

func (step *StepDeregisterImage) Cleanup(_ multistep.StateBag) {
	return
}
