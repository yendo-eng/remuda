package cli

// ConfigValidateCmd validates the resolved config file.
type ConfigValidateCmd struct{}

func (c *ConfigValidateCmd) Run(kctx Context) error {
	_, _, err := loadConfigV1(kctx)
	return err
}
