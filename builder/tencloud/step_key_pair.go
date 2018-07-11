package tencloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/3van/tencloud-go"
	retry "github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type StepKeyPair struct {
	Debug                bool
	DebugKeyPath         string
	TemporaryKeyPairName string
	KeyPairName          string
	PrivateKeyFile       string

	doCleanup bool
}

func (step *StepKeyPair) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	if step.PrivateKeyFile != "" {
		ui.Say("using provided private key for SSH")
		privateKey, err := ioutil.ReadFile(step.PrivateKeyFile)
		if err != nil {
			state.Put("error", fmt.Errorf("could not load private key for SSH: %s", err))
			return multistep.ActionHalt
		}

		state.Put("keyPair", step.KeyPairName)
		state.Put("privateKey", string(privateKey))

		return multistep.ActionContinue
	} else if step.TemporaryKeyPairName == "" {
		ui.Say("no SSH keypair is being used")
		state.Put("keyPair", "")
		return multistep.ActionContinue
	}

	tc := state.Get("tc").(*tcapi.Client)
	config := state.Get("config").(Config)

	ui.Say(fmt.Sprintf("creating temporary keypair '%s'", step.TemporaryKeyPairName))
	resp, err := tc.CreateKeyPair(&tcapi.CreateKeyPairRequest{
		KeyName:   step.TemporaryKeyPairName,
		ProjectId: config.Project,
	})
	if err != nil {
		state.Put("error", fmt.Errorf("could not create temporary keypair: %s", err))
		return multistep.ActionHalt
	}

	step.doCleanup = true

	state.Put("keyPair", step.TemporaryKeyPairName)
	state.Put("privateKey", resp.KeyPair.PrivateKey)
	state.Put("keyID", resp.KeyPair.KeyId)

	if step.Debug {
		ui.Message(fmt.Sprintf("saving private key for '%s' to '%s'", step.TemporaryKeyPairName, step.DebugKeyPath))
		fh, err := os.Create(step.DebugKeyPath)
		if err != nil {
			state.Put("error", fmt.Errorf("could not save private key to disk: %s", err))
			return multistep.ActionHalt
		}
		defer fh.Close()

		if _, err := fh.Write([]byte(resp.KeyPair.PrivateKey)); err != nil {
			state.Put("error", fmt.Errorf("could not write private key to disk: %s", err))
			return multistep.ActionHalt
		}

		if err := fh.Chmod(0600); err != nil && runtime.GOOS != "windows" {
			state.Put("error", fmt.Errorf("could not set permissions on private key: %s", err))
			return multistep.ActionHalt
		}
	}

	return multistep.ActionContinue
}

func (step *StepKeyPair) Cleanup(state multistep.StateBag) {
	if !step.doCleanup {
		return
	}

	tc := state.Get("tc").(*tcapi.Client)
	ui := state.Get("ui").(packer.Ui)
	keyId := state.Get("keyID").(string)

	if keyId != "" {
		err := retry.Retry(0.2, 30, 11, func(_ uint) (bool, error) {
			ui.Say(fmt.Sprintf("removing temporary keypair '%s' (ID '%s')", step.TemporaryKeyPairName, keyId))
			err := tc.DeleteKeyPairs(&tcapi.DeleteKeyPairsRequest{
				KeyIds: []string{
					keyId,
				},
			})
			if err != nil {
				ui.Error(fmt.Sprintf("could not disassociate key from instance: %s", err))
				return false, nil
			}
			state.Put("keyID", "")
			return true, nil
		})
		if err != nil {
			ui.Error(fmt.Sprintf("could not disassociate key: %s", err))
		}
	}

	if step.Debug {
		if err := os.Remove(step.DebugKeyPath); err != nil {
			ui.Error(fmt.Sprintf("could not remove private key from disk: %s", err))
		}
	}
}
