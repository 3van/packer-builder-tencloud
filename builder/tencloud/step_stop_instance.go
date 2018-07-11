package tencloud

import (
	"context"
	"fmt"

	"github.com/3van/tencloud-go"

	"github.com/hashicorp/packer/common"
	retry "github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepStopInstance struct {
	Skip                bool
	DisableStopInstance bool
}

func (step *StepStopInstance) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	tc := state.Get("tc").(*tcapi.Client)
	instance := state.Get("instance").(tcapi.Instance)
	ui := state.Get("ui").(packer.Ui)

	if step.Skip {
		return multistep.ActionContinue
	}

	if !step.DisableStopInstance {
		ui.Say("stopping source instance")
		err := common.Retry(10, 60, 6, func(i uint) (bool, error) {
			ui.Message(fmt.Sprintf("stopping instance '%s', attempt %d", instance.InstanceId, i+1))
			err := tc.StopInstances(&tcapi.StopInstancesRequest{
				InstanceIds: []string{instance.InstanceId},
			})
			if err == nil {
				return true, nil
			} else {
				return false, err
			}
		})
		if err != nil {
			state.Put("error", fmt.Errorf("could not stop instance: %s", err))
			return multistep.ActionHalt
		}
	} else {
		ui.Say(fmt.Sprintf("automatic instance stop disabled - please stop instance '%s' manually so execution can proceed", instance.InstanceId))
	}

	ui.Say(fmt.Sprintf("waiting for instance '%s' to stop", instance.InstanceId))
	stateChange := StateChangeConf{
		Pending:   []string{"RUNNING", "STOPPING"},
		Target:    "STOPPED",
		Refresh:   InstanceStateRefreshFunc(tc, instance.InstanceId),
		StepState: state,
	}

	if _, err := WaitForState(&stateChange); err != nil {
		state.Put("error", fmt.Errorf("error waiting for instance '%s' to stop: %s", instance.InstanceId, err))
		return multistep.ActionHalt
	}

	if tempKeyID, ok := state.GetOk("keyID"); ok {
		keyID := tempKeyID.(string)
		if keyID != "" {
			err := retry.Retry(0.2, 30, 11, func(_ uint) (bool, error) {
				ui.Say(fmt.Sprintf("disassociating key '%s' from instance '%s' before image creation", keyID, instance.InstanceId))
				err := tc.DisassociateInstancesKeyPairs(&tcapi.DisassociateInstancesKeyPairsRequest{
					InstanceIds: []string{
						instance.InstanceId,
					},
					KeyIds: []string{
						keyID,
					},
					ForceStop: true,
				})
				if err != nil {
					ui.Error(fmt.Sprintf("could not disassociate key: %s", err))
					return false, nil
				} else {
					return true, nil
				}
			})
			if err != nil {
				ui.Error(fmt.Sprintf("could not disassociate key: %s", err))
			}
		}
	}

	return multistep.ActionContinue
}

func (step *StepStopInstance) Cleanup(_ multistep.StateBag) {
	return
}
