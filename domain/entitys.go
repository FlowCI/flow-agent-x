package domain

type JobCache struct {
	Id     string   `json:"id"`
	FlowId string   `json:"flowId"`
	JobId  string   `json:"jobId"`
	Key    string   `json:"key"`
	Os     string   `json:"os"`
	Files  []string `json:"files"`
}
