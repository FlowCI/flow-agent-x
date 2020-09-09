package domain

import (
	"fmt"
	"github/flowci/flow-agent-x/util"
	"strings"
)

type (
	// DockerVolume volume will mount to step docker
	DockerVolume struct {
		Name   string // volume name
		Dest   string // dest path
		Script string // script file name to execute
		Image  string // image contain volume
		Init   string // init script /ws/{init} in image that will copy required data to /target
	}
)

func (v *DockerVolume) HasImage() bool {
	return v.Image != ""
}

func (v *DockerVolume) InitScriptInImage() string {
	return fmt.Sprintf("/ws/%s", v.Init)
}

func (v *DockerVolume) DefaultTargetInImage() string {
	return "/target"
}

func (v *DockerVolume) ScriptPath() string {
	return fmt.Sprintf("%s/%s", v.Dest, v.Script)
}

func (v *DockerVolume) ToBindStr() string {
	return fmt.Sprintf("%s:%s", v.Name, v.Dest)
}

// NewFromString parse string name=xxx,dest=xxx,script=xxx;name=xxx,dest=xxx,script=xxx;...
func NewVolumesFromString(val string) []*DockerVolume {
	var volumes []*DockerVolume

	if util.IsEmptyString(val) {
		return volumes
	}

	tokens := strings.Split(val, ";")
	if len(tokens) == 0 {
		return volumes
	}

	getValue := func(val string) string {
		pair := strings.Split(val, "=")

		if len(pair) != 2 {
			panic(fmt.Errorf("'%s' is invalid volume string, must be key=value pair", val))
		}

		return pair[1]
	}

	for _, token := range tokens {
		if util.IsEmptyString(token) {
			continue
		}

		fields := strings.Split(token, ",")
		if len(fields) != 5 {
			panic(fmt.Errorf("'%s' is invalid volume string, fields must contain name, dest, script", token))
		}

		name := fields[0]
		dest := fields[1]
		script := fields[2]
		image := fields[3]
		initScript := fields[4]

		volumes = append(volumes, &DockerVolume{
			Name:   getValue(name),
			Dest:   getValue(dest),
			Script: getValue(script),
			Image:  getValue(image),
			Init:   getValue(initScript),
		})
	}

	return volumes
}
