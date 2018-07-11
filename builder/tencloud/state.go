package tencloud

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/3van/tencloud-go"
	"github.com/hashicorp/packer/helper/multistep"
)

type StateRefreshFunc func() (result interface{}, state string, err error)

type StateChangeConf struct {
	Pending   []string
	Refresh   StateRefreshFunc
	StepState multistep.StateBag
	Target    string
}

func ImageStateRefreshFunc(tc *tcapi.Client, imageId string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := tc.DescribeImages(&tcapi.DescribeImagesRequest{
			ImageIds: []string{imageId},
		})
		if err != nil {
			return nil, "", nil
		}

		if resp == nil || len(resp.ImageSet) == 0 {
			return nil, "", nil
		}

		return resp.ImageSet[0], resp.ImageSet[0].ImageState, nil
	}
}

func ImageExistsRefreshFunc(tc *tcapi.Client, imageName string) StateRefreshFunc {
	return func() (interface{}, string, error) {
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
						imageName,
					},
				},
			},
			Limit: 1,
		}
		resp, err := tc.DescribeImages(req)
		if err != nil {
			return nil, "", nil
		}

		if resp == nil || len(resp.ImageSet) == 0 {
			return nil, "", nil
		}

		return resp.ImageSet[0], resp.ImageSet[0].ImageState, nil
	}
}

func InstanceStateRefreshFunc(tc *tcapi.Client, instanceId string) StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := tc.DescribeInstances(&tcapi.DescribeInstancesRequest{
			InstanceIds: []string{instanceId},
		})
		if err != nil {
			return nil, "", nil
		}

		if resp == nil || len(resp.InstanceSet) == 0 {
			return nil, "", nil
		}

		return resp.InstanceSet[0], resp.InstanceSet[0].InstanceState, nil
	}
}

func WaitForState(conf *StateChangeConf) (i interface{}, err error) {
	log.Printf("Waiting for state to become: %s", conf.Target)

	sleepSeconds := SleepSeconds()
	maxTicks := TimeoutSeconds()/sleepSeconds + 1
	notfoundTick := 0

	for {
		var currentState string
		i, currentState, err = conf.Refresh()
		if err != nil {
			return
		}

		if i == nil {
			notfoundTick += 1
			if notfoundTick > maxTicks {
				return nil, errors.New("couldn't find resource")
			}
		} else {
			notfoundTick = 0

			if currentState == conf.Target {
				return
			}

			if conf.StepState != nil {
				if _, ok := conf.StepState.GetOk(multistep.StateCancelled); ok {
					return nil, errors.New("interrupted")
				}
			}

			found := false
			for _, allowed := range conf.Pending {
				if currentState == allowed {
					found = true
					break
				}
			}

			if !found {
				err := fmt.Errorf("unexpected state '%s', wanted target '%s'", currentState, conf.Target)
				return nil, err
			}
		}

		time.Sleep(time.Duration(sleepSeconds) * time.Second)
	}
}

func WaitForExists(conf *StateChangeConf) (i interface{}, err error) {
	log.Printf("Waiting for resource to exist")

	sleepSeconds := SleepSeconds()
	maxTicks := TimeoutSeconds()/sleepSeconds + 1
	notfoundTick := 0

	for {
		i, _, err = conf.Refresh()
		if err != nil {
			return nil, errors.New("couldn't find resource")
		}

		if i != nil {
			return
		} else {
			notfoundTick += 1
			if notfoundTick > maxTicks {
				return nil, errors.New("couldn't find resource")
			}
		}

		time.Sleep(time.Duration(sleepSeconds) * time.Second)
	}
}

func WaitForDoesNotExist(conf *StateChangeConf) (i interface{}, err error) {
	log.Printf("Waiting for resource to cease to exist")

	sleepSeconds := SleepSeconds()
	maxTicks := TimeoutSeconds()/sleepSeconds + 1
	foundTick := 0

	for {
		i, _, err = conf.Refresh()
		if err != nil {
			return nil, errors.New("couldn't find resource")
		}

		if i == nil {
			return
		} else {
			foundTick += 1
			if foundTick > maxTicks {
				return nil, errors.New("resource still exists after timeout")
			}
		}

		time.Sleep(time.Duration(sleepSeconds) * time.Second)
	}
}

func TimeoutSeconds() (seconds int) {
	seconds = 300

	override := os.Getenv("TC_TIMEOUT_SECONDS")
	if override != "" {
		n, err := strconv.Atoi(override)
		if err != nil {
			log.Printf("Invalid timeout seconds '%s', using default", override)
		} else {
			seconds = n
		}
	}

	log.Printf("Allowing %ds to complete (change with TC_TIMEOUT_SECONDS)", seconds)
	return seconds
}

func SleepSeconds() (seconds int) {
	seconds = 2

	override := os.Getenv("TC_POLL_DELAY_SECONDS")
	if override != "" {
		n, err := strconv.Atoi(override)
		if err != nil {
			log.Printf("Invalid sleep seconds '%s', using default", override)
		} else {
			seconds = n
		}
	}

	log.Printf("Using %ds as polling delay (change with TC_POLL_DELAY_SECONDS)", seconds)
	return seconds
}
