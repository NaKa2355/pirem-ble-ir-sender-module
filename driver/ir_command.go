package driver

type IrCommand interface {
	Match(IrCommandCaces) error
}

type IrCommandCaces struct {
	SendIr     func(SendIrCommand) error
	GetVersion func(GetVersionCommand) error
}

type SendIrCommand struct {
	IrData []int16
	Result chan error
}

func (c SendIrCommand) Match(cases IrCommandCaces) error {
	return cases.SendIr(c)
}

type GetVersionCommandResult struct {
	Version string
	Err     error
}

type GetVersionCommand struct {
	Result chan GetVersionCommandResult
}

func (c GetVersionCommand) Match(cases IrCommandCaces) error {
	return cases.GetVersion(c)
}
