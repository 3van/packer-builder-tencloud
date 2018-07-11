package tencloud

import (
	"context"
	"fmt"

	"github.com/3van/tencloud-go"

	retry "github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepCreateImage struct {
	Image tcapi.Image
}

func (step *StepCreateImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(Config)
	tc := state.Get("tc").(*tcapi.Client)
	instance := state.Get("instance").(tcapi.Instance)
	ui := state.Get("ui").(packer.Ui)

	// oh cool i guess we'll do a retry loop here too because that's fun 😥
	created := false
	err := retry.Retry(0.2, 30, 11, func(_ uint) (bool, error) {
		ui.Say(fmt.Sprintf("creating image '%s'", config.ImageName))
		req := &tcapi.CreateImageRequest{
			InstanceId: instance.InstanceId,
			ImageName:  config.ImageName,
		}
		err := tc.CreateImage(req)
		if err != nil {
			ui.Error(fmt.Sprintf("error creating image: %s", err))
			return false, nil
		} else {
			created = true
			return true, nil
		}
	})
	if !created || err != nil {
		state.Put("error", fmt.Errorf("error creating image: %s", err))
		return multistep.ActionHalt
	}

	stateChange := StateChangeConf{
		Pending:   []string{"SYNCING", "PENDING"},
		Target:    "NORMAL",
		Refresh:   ImageExistsRefreshFunc(tc, config.ImageName),
		StepState: state,
	}
	image, err := WaitForExists(&stateChange)
	if err != nil {
		state.Put("error", fmt.Errorf("error waiting for image: %s", err))
		return multistep.ActionHalt
	}

	imageInst := image.(tcapi.Image)
	ui.Message(fmt.Sprintf("image ID: %s", imageInst.ImageId))
	images := make(map[string]string)
	images[config.Region] = imageInst.ImageId
	state.Put("images", images)

	ui.Say("waiting for image to become ready")
	stateChange = StateChangeConf{
		Pending:   []string{"SYNCING", "PENDING"},
		Target:    "NORMAL",
		Refresh:   ImageStateRefreshFunc(tc, imageInst.ImageId),
		StepState: state,
	}
	if _, err := WaitForState(&stateChange); err != nil {
		state.Put("error", fmt.Errorf("error waiting for image: %s", err))
		return multistep.ActionHalt
	}

	step.Image = imageInst

	return multistep.ActionContinue
}

func (step *StepCreateImage) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	tc := state.Get("tc").(*tcapi.Client)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("deleting image because of cancellation")
	req := &tcapi.DeleteImagesRequest{ImageIds: []string{step.Image.ImageId}}
	if err := tc.DeleteImages(req); err != nil {
		ui.Error(fmt.Sprintf("could not delete image: %s", err))
		return
	}
}
