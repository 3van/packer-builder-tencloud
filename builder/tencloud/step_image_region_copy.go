package tencloud

import (
	"context"
	"fmt"
	"time"

	"github.com/3van/tencloud-go"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepImageRegionCopy struct {
	Regions []string
	Name    string
}

func (step *StepImageRegionCopy) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	tc := state.Get("tc").(*tcapi.Client)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)
	images := state.Get("images").(map[string]string)
	image := images[config.Region]
	syncRegions := make([]string, 0, len(step.Regions))

	if len(step.Regions) <= 0 {
		ui.Message("no regions to copy image to")
		return multistep.ActionContinue
	}

	ui.Say(fmt.Sprintf("copying built image artifact '%s' to other regions", image))

	for _, region := range step.Regions {
		if region == config.Region {
			ui.Message(fmt.Sprintf("duplicate region '%s' found, skipping", region))
			continue
		}

		ui.Message(fmt.Sprintf("adding region '%s' to copy list", region))
		syncRegions = append(syncRegions, region)
	}

	if len(syncRegions) <= 0 {
		ui.Message("no additional regions to copy image to")
		return multistep.ActionContinue
	}

	err := tc.SyncImages(&tcapi.SyncImagesRequest{
		ImageIds: []string{
			image,
		},
		DestinationRegions: syncRegions,
	})
	if err != nil {
		state.Put("error", fmt.Errorf("could not copy image to specified regions: %s", err))
		return multistep.ActionHalt
	}

	errs := new(packer.MultiError)
	for _, region := range syncRegions {
		ui.Message(fmt.Sprintf("searching for copied image ID in region '%s'", region))
		images[region] = ""
		thisClient := tc.Copy(region, nil)
		iterCount := 0

		req := &tcapi.DescribeImagesRequest{
			Filters: []tcapi.Filter{
				tcapi.Filter{
					Name: "image-type",
					Values: []string{
						"PRIVATE_IMAGE",
					},
				},
				tcapi.Filter{
					Name: "image-name",
					Values: []string{
						step.Name,
					},
				},
			},
			Limit: 1,
		}

		for true {
			if iterCount >= 5 {
				errs = packer.MultiErrorAppend(errs, fmt.Errorf("could not find image copy in region '%s'", region))
				break
			}
			resp, err := thisClient.DescribeImages(req)
			if err != nil {
				errs = packer.MultiErrorAppend(errs, err)
				break
			}
			if (resp.TotalCount == 0) || (len(resp.ImageSet) == 0) {
				time.Sleep(2 * time.Second)
				iterCount++
				continue
			}

			// yes, the ImageId is literally "unkown", this isn't a mistake
			if (resp.ImageSet[0].ImageId != "unkown") && (resp.ImageSet[0].ImageName == step.Name) {
				images[region] = resp.ImageSet[0].ImageId
				break
			}

			time.Sleep(2 * time.Second)
			iterCount++
			continue
		}
	}

	for imageRegion, imageId := range images {
		if imageId == "" {
			continue
		}
		thisClient := tc.Copy(imageRegion, nil)
		stateChange := StateChangeConf{
			Pending:   []string{"SYNCING"},
			Target:    "NORMAL",
			Refresh:   ImageStateRefreshFunc(thisClient, imageId),
			StepState: state,
		}

		if _, err := WaitForState(&stateChange); err != nil {
			errs = packer.MultiErrorAppend(errs, fmt.Errorf("error waiting for image copy '%s' in region '%s': %s", imageId, imageRegion, err))
			continue
		}
	}

	if len(errs.Errors) > 0 {
		state.Put("error", errs)
		ui.Error(errs.Error())
		return multistep.ActionHalt
	}

	state.Put("images", images)
	return multistep.ActionContinue
}

func (step *StepImageRegionCopy) Cleanup(_ multistep.StateBag) {
	return
}
