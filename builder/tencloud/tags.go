package tencloud

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/3van/tencloud-go"
)

// TagMap is a helper type for a string=>string map
type TagMap map[string]string

// TagFilterOptions holds all of the potential filters for describing a tc image
type TagFilterOptions struct {
	Filters        map[string]string `mapstructure:"filters"`
	TagFilters     map[string]string `mapstructure:"tag_filters"`
	TagFilterDelim string            `mapstructure:"tag_filter_delimiter"`
	MostRecent     bool              `mapstructure:"most_recent"`
}

// IsSet determines if the TagMap is set or not
func (t TagMap) IsSet() bool {
	return len(t) > 0
}

// Flatten well... flattens the tag map with the supplied delimiter
func (t TagMap) Flatten(delim string) string {
	var res string
	for k, v := range t {
		res = fmt.Sprintf("%s:%s=%s", res, k, v)
	}

	return res
}

// Empty determines if the TagFilterOptions is set or not
func (t TagFilterOptions) Empty() bool {
	return len(t.Filters) == 0 && len(t.TagFilters) == 0
}

// IsDelimSet checks if there is a delimeter set for tag filters or not
func (t TagFilterOptions) IsDelimSet() bool {
	return t.TagFilterDelim != ""
}

// FindImage finds a source image id given the provided filters
func (t TagFilterOptions) FindImage(client *tcapi.Client) (*tcapi.Image, error) {
	images := []tcapi.Image{}

	req := &tcapi.DescribeImagesRequest{
		Filters: t.tcFilters(),
		Limit:   100,
		Offset:  0,
	}

	for {
		resp, err := client.DescribeImages(req)
		if err != nil {
			return nil, err
		}
		if resp.TotalCount == 0 || len(resp.ImageSet) == 0 {
			break
		}

		// if we have no tag filters to process, add images to list
		if len(t.TagFilters) == 0 {
			images = append(images, resp.ImageSet...)
		} else {
			for i := range resp.ImageSet {
				// Process description tags
				if resp.ImageSet[i].ImageDescription == "" {
					continue
				}

				// if tags match, add to list
				if t.matchImageDesc(resp.ImageSet[i].ImageDescription) {
					images = append(images, resp.ImageSet[i])
				}
			}
		}

		if resp.TotalCount > req.Limit {
			req.Offset += req.Limit
			continue
		} else {
			break
		}
	}

	log.Printf("Found %d images", len(images))

	if len(images) == 0 {
		return nil, fmt.Errorf("no image found matching supplied filters")
	}

	if len(images) == 1 {
		log.Printf("Using ImageID: %s", &images[0].ImageId)
		return &images[0], nil
	}

	if !t.MostRecent {
		return nil, fmt.Errorf("most_recent was not selected, and more than one image matching filters was found")
	}

	// Find latest image, return that ID
	latest := images[0]
	latestTime, err := time.Parse(time.RFC3339, images[0].CreatedTime)
	if err != nil {
		return nil, fmt.Errorf("getting images created time: %v", err)
	}

	for i := 1; i < len(images); i++ {
		// parse time
		t, err := time.Parse(time.RFC3339, images[i].CreatedTime)
		if err != nil {
			return nil, fmt.Errorf("getting image (%s) created time: %v", images[i].ImageName, err)
		}

		// if image is newer, sub
		if latestTime.Before(t) {
			latestTime = t
			latest = images[i]
		}
	}

	return &latest, nil
}

func (t TagFilterOptions) tcFilters() []tcapi.Filter {
	resp := []tcapi.Filter{}
	for k, v := range t.Filters {
		resp = append(resp, tcapi.Filter{
			Name:   k,
			Values: []string{v},
		})
	}
	return resp
}

func (t TagFilterOptions) matchImageDesc(iDesc string) bool {
	// First, convert image description to map, checking length along the way.
	iTags := map[string]string{}
	parts := strings.Split(iDesc, t.TagFilterDelim)
	if len(parts) == 0 {
		return false
	}
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		p := strings.Split(parts[i], "=")
		if len(p) != 2 {
			continue
		}
		iTags[p[0]] = p[1]
	}

	if len(iTags) == 0 {
		return false
	}

	// now we just have to compare the two maps
	// we check against specified tag filters. If the image has more tags, we don't care
	for k, v := range t.TagFilters {
		if w, ok := iTags[k]; !ok || v != w {
			return false
		}
	}

	return true
}
