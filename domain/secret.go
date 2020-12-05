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
	}

	SecretBase struct {
		Name     string `json:"name"`
		Category string `json:"category"`
	}
)

func (s *SecretBase) GetName() string {
	return s.Name
}

func (s *SecretBase) GetCategory() string {
	return s.Category
}