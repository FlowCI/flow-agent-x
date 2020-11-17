package domain

const (
	ok = 200
)

type (
	ResponseMessage interface {
		IsOk() bool
		GetMessage() string
	}

	// Response the base response message struct
	Response struct {
		Code    int
		Message string
	}

	JobCacheResponse struct {
		Response
		Data *JobCache
	}
)

func (r *Response) IsOk() bool {
	return r.Code == ok
}

func (r *Response) GetMessage() string {
	return r.Message
}

func (r *JobCacheResponse) IsOk() bool {
	return r.Code == ok
}

func (r *JobCacheResponse) GetMessage() string {
	return r.Message
}
