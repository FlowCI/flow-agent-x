package domain

import "strconv"

type SmtpConfig struct {
	ConfigBase
	Server     string
	Port       int
	SecureType string `json:"secure"`
	Auth       *SimpleAuthPair
}

func (c *SmtpConfig) ToEnvs() map[string]string {
	return map[string]string{
		c.GetName() + "_SERVER":        c.Server,
		c.GetName() + "_PORT":          strconv.Itoa(c.Port),
		c.GetName() + "_SECURE_TYPE":   c.SecureType,
		c.GetName() + "_AUTH_USERNAME": c.Auth.Username,
		c.GetName() + "_AUTH_PASSWORD": c.Auth.Password,
	}
}
