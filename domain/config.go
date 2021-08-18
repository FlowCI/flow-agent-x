package domain

const (
	ConfigCategorySmtp = "SMTP"
	ConfigCategoryText = "TEXT"
)

type (
	Config interface {
		GetName() string
		GetCategory() string
		ToEnvs() map[string]string
		ConfigMarker()
	}

	ConfigBase struct {
		Name     string
		Category string
	}

	ConfigResponse struct {
		Response
		Data *ConfigBase `json:"data"`
	}
)

func (c *ConfigBase) GetName() string {
	return c.Name
}

func (c *ConfigBase) GetCategory() string {
	return c.Category
}

func (c *ConfigBase) ConfigMarker() {
	// placeholder
}
