package domain

const (
	ok = 200
)

type (
	// Response the base response message struct
	Response struct {
		Code    int
		Message string
	}
)

// IsOk check response code is equal to 200
func (r *Response) IsOk() bool {
	return r.Code == ok
}
