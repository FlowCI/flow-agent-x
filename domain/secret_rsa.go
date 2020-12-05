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
