package domain

type (
	SimpleAuthPair struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	AuthSecret struct {
		SecretBase
		Pair *SimpleAuthPair `json:"pair"`
	}
)

func (s *AuthSecret) ToEnvs() map[string]string {
	return map[string]string{
		s.GetName() + "_USERNAME": s.Pair.Username,
		s.GetName() + "_PASSWORD": s.Pair.Password,
	}
}
