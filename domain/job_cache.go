package domain

type JobCache struct {
	Id     string   `json:"id"`
	FlowId string   `json:"flowId"`
	JobId  string   `json:"jobId"`
	Key    string   `json:"key"`
	Os     string   `json:"os"`
	Files  []string `json:"files"`
}

type JobCacheResponse struct {
	Response
	Data *JobCache
}

func (r *JobCacheResponse) IsOk() bool {
	return r.Code == ok
}

func (r *JobCacheResponse) GetMessage() string {
	return r.Message
}
