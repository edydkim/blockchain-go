package utils

import (
	"github.com/magiconair/properties"
)

var PropertyFiles = []string{"./resources/resource.properties"}

var Props, _ = properties.LoadFiles(PropertyFiles, properties.UTF8, true)

type ResourceManager struct{}

func (r ResourceManager) GetProperty(propertyName string) string {
	value, ok := Props.Get(propertyName)
	if !ok {
		return Props.MustGet("Not Found")
	} else {
		return value
	}
}
