package tencloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"

	"github.com/3van/tencloud-go"

	retry "github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/common/uuid"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepRunInstance struct {
	AvailabilityZone        string   `mapstructure:"availability_zone"`
	SourceImageId           string   `mapstructure:"source_image_id"`
	InstanceType            string   `mapstructure:"instance_type"`
	InstanceChargeType      string   `mapstructure:"instance_charge_type"`
	SystemDiskType          string   `mapstructure:"system_disk_type"`
	SystemDiskSize          int      `mapstructure:"system_disk_size"`
	VpcId                   string   `mapstructure:"vpc_id"`
	SubnetId                string   `mapstructure:"subnet_id"`
	InternetChargeType      string   `mapstructure:"internet_charge_type"`
	InternetMaxBandwidthOut int      `mapstructure:"internet_max_bandwidth_out"`
	PublicIpAssigned        bool     `mapstructure:"public_ip_assigned"`
	SecurityGroupIds        []string `mapstructure:"security_group_ids"`
	UserData                string   `mapstructure:"user_data"`
	UserDataFile            string   `mapstructure:"user_data_file"`

	instanceId   string
	instanceName string
}

func (step *StepRunInstance) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	tc := state.Get("tc").(*tcapi.Client)
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)
	var keyId string
	if tempId, ok := state.GetOk("keyID"); ok {
		keyId = tempId.(string)
	}

	userData := step.UserData
	if step.UserDataFile != "" {
		contents, err := ioutil.ReadFile(step.UserDataFile)
		if err != nil {
			state.Put("error", fmt.Errorf("could not read userdata file: %s", err))
			return multistep.ActionHalt
		}
		userData = string(contents)
	}
	if _, err := base64.StdEncoding.DecodeString(userData); err != nil {
		log.Printf("base64 encoding userdata")
		userData = base64.StdEncoding.EncodeToString([]byte(userData))
	}

	ui.Say("launching source instance")
	image, ok := state.Get("source_image").(tcapi.Image)
	if !ok {
		state.Put("error", fmt.Errorf("source_image failed type assert"))
		return multistep.ActionHalt
	}
	step.SourceImageId = image.ImageId
	step.instanceName = fmt.Sprintf("packer_%s", uuid.TimeOrderedUUID())
	step.instanceName = strings.Replace(step.instanceName, "-", "", -1)
	step.instanceName = step.instanceName[:24]

	req := &tcapi.RunInstancesRequest{
		Placement: tcapi.Placement{
			Zone:      step.AvailabilityZone,
			ProjectId: config.Project,
		},
		ImageId:            image.ImageId,
		InstanceChargeType: step.InstanceChargeType,
		InstanceType:       step.InstanceType,
		SystemDisk: tcapi.SystemDisk{
			DiskType: step.SystemDiskType,
			DiskSize: step.SystemDiskSize,
		},
		VirtualPrivateCloud: tcapi.VirtualPrivateCloud{
			VpcId:    step.VpcId,
			SubnetId: step.SubnetId,
		},
		InternetAccessible: tcapi.InternetAccessible{
			InternetChargeType:      step.InternetChargeType,
			InternetMaxBandwidthOut: step.InternetMaxBandwidthOut,
			PublicIpAssigned:        strconv.FormatBool(step.PublicIpAssigned),
		},
		InstanceCount: 1,
		InstanceName:  step.instanceName,
		LoginSettings: tcapi.LoginSettings{
			KeyIds: []string{
				keyId,
			},
		},
		SecurityGroupIds: step.SecurityGroupIds,
		UserData:         userData,
	}

	resp, err := tc.RunInstances(req)
	if err != nil {
		state.Put("error", fmt.Errorf("error launching source instance: %s", err))
		return multistep.ActionHalt
	}
	if resp.InstanceIdSet == nil || len(resp.InstanceIdSet) < 1 {
		state.Put("error", fmt.Errorf("unknown error launching source instance"))
		return multistep.ActionHalt
	}
	step.instanceId = resp.InstanceIdSet[0]

	ui.Message(fmt.Sprintf("spawned instance ID: %s", step.instanceId))
	ui.Message(fmt.Sprintf("spawned instance name: %s", step.instanceName))
	ui.Say(fmt.Sprintf("waiting for instance '%s' to become ready...", step.instanceId))

	stateChange := StateChangeConf{
		Pending:   []string{"PENDING"},
		Target:    "RUNNING",
		Refresh:   InstanceStateRefreshFunc(tc, step.instanceId),
		StepState: state,
	}

	if _, err := WaitForState(&stateChange); err != nil {
		state.Put("error", fmt.Errorf("error waiting for instance '%s' to become ready: %s", step.instanceId, err))
		return multistep.ActionHalt
	}

	describeResp, err := tc.DescribeInstances(&tcapi.DescribeInstancesRequest{
		InstanceIds: []string{
			step.instanceId,
		},
	})
	if err != nil || len(describeResp.InstanceSet) < 1 {
		state.Put("error", fmt.Errorf("could not query for spawned instance '%s': %s", step.instanceId, err))
		return multistep.ActionHalt
	}

	instance := describeResp.InstanceSet[0]
	if instance.PrivateIpAddresses != nil && len(instance.PrivateIpAddresses) > 0 {
		ui.Message(fmt.Sprintf("Private IP: %s", instance.PrivateIpAddresses[0]))
	}
	if instance.PublicIpAddresses != nil && len(instance.PublicIpAddresses) > 0 {
		ui.Message(fmt.Sprintf("Public IP: %s", instance.PublicIpAddresses[0]))
	}

	state.Put("instance", instance)
	return multistep.ActionContinue
}

func (step *StepRunInstance) Cleanup(state multistep.StateBag) {
	tc := state.Get("tc").(*tcapi.Client)
	ui := state.Get("ui").(packer.Ui)

	if tempKeyID, ok := state.GetOk("keyID"); ok {
		keyID := tempKeyID.(string)
		if keyID != "" {
			err := retry.Retry(0.2, 30, 11, func(_ uint) (bool, error) {
				ui.Say(fmt.Sprintf("disassociating key '%s' from instance '%s' before termination", keyID, step.instanceId))
				err := tc.DisassociateInstancesKeyPairs(&tcapi.DisassociateInstancesKeyPairsRequest{
					InstanceIds: []string{
						step.instanceId,
					},
					KeyIds: []string{
						keyID,
					},
					ForceStop: true,
				})
				if err != nil {
					ui.Error(fmt.Sprintf("could not disassociate key from instance: %s", err))
					return false, nil
				} else {
					state.Put("keyID", "")
					return true, nil
				}
			})
			if err != nil {
				ui.Error(fmt.Sprintf("could not disassociate key: %s", err))
			}
		}
	}

	if step.instanceId != "" {
		err := retry.Retry(0.2, 30, 11, func(_ uint) (bool, error) {
			ui.Say(fmt.Sprintf("trying to terminate source instance '%s'", step.instanceId))
			err := tc.TerminateInstances(&tcapi.TerminateInstancesRequest{InstanceIds: []string{step.instanceId}})
			if err == nil {
				return true, nil
			} else {
				return false, nil
			}
		})

		if err != nil {
			ui.Error(fmt.Sprintf("could not terminate instance: %s", err))
			return
		}
	}

	ui.Say(fmt.Sprintf("waiting for instance '%s' to cease existence...", step.instanceId))

	stateChange := StateChangeConf{
		Pending:   []string{"TERMINATING"},
		Target:    "TERMINATED",
		Refresh:   InstanceStateRefreshFunc(tc, step.instanceId),
		StepState: state,
	}

	if _, err := WaitForDoesNotExist(&stateChange); err != nil {
		ui.Error(fmt.Sprintf("error waiting for instance '%s' to cease existence: %s", step.instanceId, err))
	}

	return
}
