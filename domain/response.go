package domain

const (
	ok = 200
)

// Response the base response message struct
type Response struct {
	Code    int
	Message string
}

// IsOk check response code is equal to 200
func (r *Response) IsOk() bool {
	return r.Code == ok
}

type SettingsResponse struct {
	Response
	Data Settings
}
