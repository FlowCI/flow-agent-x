package domain

const (
	SecretCategoryAuth        = "AUTH"
	SecretCategorySshRsa      = "SSH_RSA"
	SecretCategoryToken       = "TOKEN"
	SecretCategoryAndroidSign = "ANDROID_SIGN"
	SecretCategoryKubeConfig  = "KUBE_CONFIG"
)

type (
	Secret interface {
		GetName() string
		GetCategory() string
		ToEnvs() map[string]string
		SecretMarker()
	}

	SecretBase struct {
		Name     string `json:"name"`
		Category string `json:"category"`
	}

	SecretField struct {
		Data string `json:"data"`
	}

	SecretResponse struct {
		Response
		Data *SecretBase `json:"data"`
	}
)

func (s *SecretBase) GetName() string {
	return s.Name
}

func (s *SecretBase) GetCategory() string {
	return s.Category
}

func (s *SecretBase) SecretMarker() {
	// placeholder
}
