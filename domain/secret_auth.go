package domain

type (
	SimpleAuthPair struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	AuthSecret struct {
		SecretBase
		Pair *SimpleAuthPair
	}
)