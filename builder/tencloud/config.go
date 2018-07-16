package tencloud

import (
	"bytes"
	"fmt"
	"github.com/3van/tencloud-go"
	"github.com/hashicorp/packer/common/uuid"
	"github.com/hashicorp/packer/helper/communicator"
	"github.com/hashicorp/packer/template/interpolate"
	"math/rand"
	"os"
	"strings"
	"time"
)

// authentication configuration
type AuthConfig struct {
	KeyID   string `mapstructure:"key_id"`
	Key     string `mapstructure:"key"`
	Region  string `mapstructure:"region"`
	Project int    `mapstructure:"project"`

	client *tcapi.Client
}

func (c *AuthConfig) Client() (*tcapi.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	c.client = tcapi.New(c.KeyID, c.Key, c.Region, nil)

	return c.client, nil
}

func (c *AuthConfig) Prepare(ctx *interpolate.Context) []error {
	var errs []error
	if (len(c.Key) <= 0) || (len(c.KeyID) <= 0) {
		errs = append(errs,
			fmt.Errorf("'key_id' and 'key' must both be set"))
	}

	return errs
}

// image configuration
type ImageConfig struct {
	ImageName        string   `mapstructure:"image_name"`
	ImageDescription string   `mapstructure:"image_description"`
	ImageRegions     []string `mapstructure:"image_regions"`
	ForceDeregister  bool     `mapstructure:"force_deregister"`
	CleanImageName   bool     `mapstructure:"clean_image_name"`
}

func (c *ImageConfig) Prepare(ctx *interpolate.Context) []error {
	var errs []error
	if len(c.ImageName) > 20 {
		errs = append(errs, fmt.Errorf("image_name must be less than 20 characters"))
	}
	if len(c.ImageDescription) > 60 {
		errs = append(errs, fmt.Errorf("image_description must be less than 60 characters"))
	}
	if (c.ImageName != cleanImageName(c.ImageName)) && c.CleanImageName == false {
		errs = append(errs, fmt.Errorf("image_name can only contain alphanumerics and dashes"))
	} else {
		c.ImageName = cleanImageName(c.ImageName)
	}

	return errs
}

func cleanImageName(s string) string {
	allowed := []byte{'-'}
	b := []byte(s)
	clean := make([]byte, len(b))
	for idx, char := range b {
		if isAlphaNum(char) || bytes.IndexByte(allowed, char) != -1 {
			clean[idx] = char
		} else {
			clean[idx] = '-'
		}
	}
	return string(clean[:])
}

func isAlphaNum(b byte) bool {
	if ('0' <= b && b <= '9') || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') {
		return true
	}
	return false
}

// instance run configuration
type RunConfig struct {
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
	TemporaryKeyPairName    string   `mapstructure:"temporary_key_pair_name"`
	DisableStopInstance     bool     `mapstructure:"disable_stop_instance"`
	SSHKeyPairName          string   `mapstructure:"ssh_keypair_name"`
	SSHInterface            string   `mapstructure:"ssh_interface"`

	Comm communicator.Config `mapstructure:",squash"`
}

func (c *RunConfig) Prepare(ctx *interpolate.Context) []error {
	var errs []error
	c.Comm.Type = "ssh"
	c.Comm.SSHPort = 22

	if c.SSHKeyPairName == "" && c.TemporaryKeyPairName == "" && c.Comm.SSHPrivateKey == "" && c.Comm.SSHPassword == "" {
		keyName := fmt.Sprintf("packer_%s", uuid.TimeOrderedUUID())
		keyName = strings.Replace(keyName, "-", "", -1)
		c.TemporaryKeyPairName = keyName[:24]
	}

	if c.SourceImageId == "" {
		errs = append(errs, fmt.Errorf("source_image_id must be specified"))
	}

	if c.InstanceType == "" {
		errs = append(errs, fmt.Errorf("instance_type must be specified"))
	}

	if c.UserData != "" && c.UserDataFile != "" {
		errs = append(errs, fmt.Errorf("user_data and user_data_file cannot both be specified"))
	} else if c.UserDataFile != "" {
		if _, err := os.Stat(c.UserDataFile); err != nil {
			errs = append(errs, fmt.Errorf("user_data_file not found: %s", c.UserDataFile))
		}
	}

	if c.SubnetId == "" {
		errs = append(errs, fmt.Errorf("subnet_id must be specified"))
	}
	subnets := strings.Split(c.SubnetId, ",")
	rand.Seed(time.Now().Unix())
	c.SubnetId = subnets[rand.Intn(len(subnets))]

	return errs
}
