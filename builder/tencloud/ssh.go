package tencloud

import (
	"errors"
	"fmt"
	"time"

	"github.com/3van/tencloud-go"
	packerssh "github.com/hashicorp/packer/communicator/ssh"
	"github.com/hashicorp/packer/helper/multistep"
	"golang.org/x/crypto/ssh"
)

func SSHHost(tc *tcapi.Client, sshInterface string) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		const tries = 2
		for j := 0; j <= tries; j++ {
			host := ""
			i := state.Get("instance").(tcapi.Instance)
			switch sshInterface {
			case "public_ip":
				if i.PublicIpAddresses != nil && len(i.PublicIpAddresses) > 0 {
					host = i.PublicIpAddresses[0]
				}
			case "private_ip":
				if i.PrivateIpAddresses != nil && len(i.PrivateIpAddresses) > 0 {
					host = i.PrivateIpAddresses[0]
				}
			default:
				panic(fmt.Sprintf("unknown SSH interface type '%s'", sshInterface))
			}

			if host != "" {
				return host, nil
			}

			req := &tcapi.DescribeInstancesRequest{
				InstanceIds: []string{
					i.InstanceId,
				},
			}
			resp, err := tc.DescribeInstances(req)
			if err != nil {
				return "", err
			}

			if len(resp.InstanceSet) == 0 {
				return "", fmt.Errorf("instance not found: %s", i.InstanceId)
			}

			state.Put("instance", resp.InstanceSet[0])
			time.Sleep(time.Second * 2)
		}

		return "", errors.New("could not determine IP address for instance")
	}
}

func SSHConfig(username, password string) func(multistep.StateBag) (*ssh.ClientConfig, error) {
	return func(state multistep.StateBag) (*ssh.ClientConfig, error) {
		privateKey, hasKey := state.GetOk("privateKey")
		if hasKey {
			signer, err := ssh.ParsePrivateKey([]byte(privateKey.(string)))
			if err != nil {
				return nil, fmt.Errorf("error establishing SSH configuration: %s", err)
			}
			return &ssh.ClientConfig{
				User: username,
				Auth: []ssh.AuthMethod{
					ssh.PublicKeys(signer),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}, nil
		} else {
			return &ssh.ClientConfig{
				User: username,
				Auth: []ssh.AuthMethod{
					ssh.Password(password),
					ssh.KeyboardInteractive(
						packerssh.PasswordKeyboardInteractive(password)),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}, nil
		}
	}
}
