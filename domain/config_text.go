package domain

type TextConfig struct {
	ConfigBase
	Text string
}

func (c *TextConfig) ToEnvs() map[string]string {
	return map[string]string{
		c.GetName(): c.Text,
	}
}
