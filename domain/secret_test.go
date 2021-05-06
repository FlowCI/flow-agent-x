package domain

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestShouldGetVarsOfAuthSecret(t *testing.T) {
	assert := assert.New(t)

	secret := &AuthSecret{
		SecretBase: SecretBase{
			Name:     "MyAuth",
			Category: SecretCategoryAuth,
		},
		Pair: &SimpleAuthPair{
			Username: "admin",
			Password: "12345",
		},
	}

	vars := secret.ToEnvs()
	assert.NotNil(vars)
	assert.Equal(2, len(vars))

	assert.Equal("admin", vars["MyAuth_USERNAME"])
	assert.Equal("12345", vars["MyAuth_PASSWORD"])
}

func TestShouldGetVarsOfRsaSecret(t *testing.T) {
	assert := assert.New(t)

	secret := &RSASecret{
		SecretBase: SecretBase{
			Name:     "MyRSA",
			Category: SecretCategorySshRsa,
		},
		Pair: &SimpleKeyPair{
			PublicKey:  "publicAdmin",
			PrivateKey: "privateAdmin",
		},
	}

	vars := secret.ToEnvs()
	assert.NotNil(vars)
	assert.Equal(2, len(vars))

	assert.Equal("publicAdmin", vars["MyRSA_PUBLIC_KEY"])
	assert.Equal("privateAdmin", vars["MyRSA_PRIVATE_KEY"])
}

func TestShouldGetVarsOfTokenSecret(t *testing.T) {
	assert := assert.New(t)

	secret := &TokenSecret{
		SecretBase: SecretBase{
			Name:     "MyToken",
			Category: SecretCategoryToken,
		},
		Token: &SecretField{
			Data: "mytoken",
		},
	}

	vars := secret.ToEnvs()
	assert.NotNil(vars)
	assert.Equal(1, len(vars))
	assert.Equal("mytoken", vars["MyToken"])
}
