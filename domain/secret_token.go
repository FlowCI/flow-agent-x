package domain

import "github.com/flowci/flow-agent-x/util"

type (
	TokenSecret struct {
		SecretBase
		Token *SecretField `json:"token"`
	}
)

func (s *TokenSecret) ToEnvs() map[string]string {
	util.PanicIfNil(s.Token, "secret token content")

	return map[string]string{
		s.GetName(): s.Token.Data,
	}
}
