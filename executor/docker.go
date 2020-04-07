package executor

import "fmt"

type (
	// DockerVolume volume will mount to step docker
	DockerVolume struct {
		Name   string
		Dest   string
		Script string
	}
)

func (v *DockerVolume) scriptPath() string {
	return fmt.Sprintf("%s/%s", v.Dest, v.Script)
}

func (v *DockerVolume) toBindStr() string {
	return fmt.Sprintf("%s:%s", v.Name, v.Dest)
}
