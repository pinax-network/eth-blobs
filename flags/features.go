package flags

import (
	"golang.org/x/exp/slices"
)

type Feature string

// define available build features below
// const MyFeature Feature = "my_feature"

var features = []Feature{}

func GetEnabledFeatures() []Feature {
	return features
}

//nolint:golint,unused
func enableFeature(feature Feature) {
	if !slices.Contains(features, feature) {
		features = append(features, feature)
	}
}

func (f Feature) IsEnabled() bool {
	return slices.Contains(features, f)
}

func HasEnabledFeatures() bool {
	return len(features) > 0
}
