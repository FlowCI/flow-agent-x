package api

func buildMessage(event string, body []byte) (out []byte) {
	out = append([]byte(event), '\n')
	out = append(out, body...)
	return
}
