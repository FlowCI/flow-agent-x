package domain

type (
	SimpleKeyPair struct {
		PublicKey  string `json:"publicKey"`
		PrivateKey string `json:"privateKey"`
	}

	RSASecret struct {
		SecretBase
		Pair           *SimpleKeyPair `json:"pair"`
		MD5FingerPrint string         `json:"md5Fingerprint"`
	}
)

func (s *RSASecret) ToEnvs() map[string]string {
	return map[string]string{
		s.GetName() + "_PUBLIC_KEY":  s.Pair.PublicKey,
		s.GetName() + "_PRIVATE_KEY":  s.Pair.PrivateKey,
	}
}
