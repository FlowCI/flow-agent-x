package domain

type (
	TokenSecret struct {
		SecretBase
		Token *SecretField `json:"token"`
	}
)

func (s *TokenSecret) ToEnvs() map[string]string {
	return map[string]string{
		s.GetName(): s.Token.Data,
	}
}
