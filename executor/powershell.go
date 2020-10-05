// +build windows

package executor

type (
	shellExecutor struct {
		BaseExecutor
		workDir string
		binDir  string
		envFile string
	}
)

func (b *shellExecutor) Init() (err error) {
	return nil
}

func (b *shellExecutor) Start() (out error) {
	return nil
}

func (b *shellExecutor) StartTty(ttyId string, onStarted func(ttyId string)) (out error) {
	return nil
}

func (b *shellExecutor) StopTty() {

}




