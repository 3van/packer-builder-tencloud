package tencloud

import (
	"context"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepPreValidate struct {
	DestImageName   string
	ForceDeregister bool
}

func (step *StepPreValidate) Run(this context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	if step.ForceDeregister {
		ui.Say("ForceDeregister is set, will delete existing AMI if present")
		return multistep.ActionContinue
	}

	return multistep.ActionContinue
}

func (step *StepPreValidate) Cleanup(_ multistep.StateBag) {
	return
}
