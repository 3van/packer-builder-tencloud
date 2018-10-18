package tencloud

import (
	"fmt"
	"log"

	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/communicator"
	"github.com/hashicorp/packer/helper/config"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/template/interpolate"
)

const BuilderID = "epicgames.tencloud"

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	AuthConfig          `mapstructure:",squash"`
	ImageConfig         `mapstructure:",squash"`
	RunConfig           `mapstructure:",squash"`

	ctx interpolate.Context
}

type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) Prepare(rawVars ...interface{}) ([]string, error) {
	err := config.Decode(&b.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &b.config.ctx,
	}, rawVars...)
	if err != nil {
		return nil, err
	}

	if b.config.PackerConfig.PackerForce {
		b.config.ForceDeregister = true
	}

	var errs *packer.MultiError
	errs = packer.MultiErrorAppend(errs, b.config.AuthConfig.Prepare(&b.config.ctx)...)
	errs = packer.MultiErrorAppend(errs, b.config.ImageConfig.Prepare(&b.config.ctx)...)
	errs = packer.MultiErrorAppend(errs, b.config.RunConfig.Prepare(&b.config.ctx)...)

	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	log.Println(common.ScrubConfig(b.config, b.config.Key, b.config.KeyID))
	return nil, nil
}

func (b *Builder) Run(ui packer.Ui, hook packer.Hook, cache packer.Cache) (packer.Artifact, error) {
	tc, err := b.config.Client()
	if err != nil {
		return nil, err
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", b.config)
	state.Put("tc", tc)
	state.Put("hook", hook)
	state.Put("ui", ui)

	steps := []multistep.Step{
		&StepPreValidate{
			DestImageName:   b.config.ImageName,
			ForceDeregister: b.config.ForceDeregister,
		},
		&StepSourceImageInfo{
			SourceImage:       b.config.SourceImageId,
			SourceImageFilter: b.config.SourceImageFilter,
		},
		&StepKeyPair{
			Debug:                b.config.PackerDebug,
			DebugKeyPath:         fmt.Sprintf("tc_%s.pem", b.config.PackerBuildName),
			KeyPairName:          b.config.SSHKeyPairName,
			TemporaryKeyPairName: b.config.TemporaryKeyPairName,
			PrivateKeyFile:       b.config.RunConfig.Comm.SSHPrivateKey,
		},
		&StepRunInstance{
			AvailabilityZone:        b.config.AvailabilityZone,
			SourceImageId:           b.config.SourceImageId,
			SourceImageFilter:       b.config.SourceImageFilter,
			InstanceType:            b.config.InstanceType,
			InstanceChargeType:      b.config.InstanceChargeType,
			SystemDiskType:          b.config.SystemDiskType,
			SystemDiskSize:          b.config.SystemDiskSize,
			VpcId:                   b.config.VpcId,
			SubnetId:                b.config.SubnetId,
			InternetChargeType:      b.config.InternetChargeType,
			InternetMaxBandwidthOut: b.config.InternetMaxBandwidthOut,
			PublicIpAssigned:        b.config.PublicIpAssigned,
			SecurityGroupIds:        b.config.SecurityGroupIds,
			UserData:                b.config.UserData,
			UserDataFile:            b.config.UserDataFile,
		},
		&communicator.StepConnect{
			Config:    &b.config.RunConfig.Comm,
			Host:      SSHHost(tc, b.config.SSHInterface),
			SSHConfig: SSHConfig(b.config.RunConfig.Comm.SSHUsername, b.config.RunConfig.Comm.SSHPassword),
		},
		&common.StepProvision{},
		&StepStopInstance{
			Skip:                false,
			DisableStopInstance: b.config.DisableStopInstance,
		},
		&StepDeregisterImage{
			ForceDeregister: b.config.ForceDeregister,
			ImageName:       b.config.ImageName,
			Regions:         b.config.ImageRegions,
		},
		&StepCreateImage{},
		&StepImageRegionCopy{
			Regions: b.config.ImageRegions,
			Name:    b.config.ImageName,
		},
	}

	b.runner = common.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}
	if _, ok := state.GetOk("images"); !ok {
		return nil, nil
	}
	artifact := Artifact{
		Images:         state.Get("images").(map[string]string),
		BuilderIdValue: BuilderID,
		Session:        tc,
	}

	return artifact, nil
}

func (b *Builder) Cancel() {
	if b.runner != nil {
		log.Println("cancelling run...")
		b.runner.Cancel()
	}
}
