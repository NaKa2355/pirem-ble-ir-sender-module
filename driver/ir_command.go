package driver

type IrCommand interface {
	Match(IrCommandCaces)
}

type IrCommandCaces struct {
	SendIr     func(SendIrCommand)
	GetVersion func(GetVersionCommand)
}

type SendIrCommand struct {
	IrData []int16
	Result chan error
}

func (c SendIrCommand) Match(cases IrCommandCaces) {
	cases.SendIr(c)
}

type GetVersionCommandResult struct {
	Version string
	Err     error
}

type GetVersionCommand struct {
	Result chan GetVersionCommandResult
}

func (c GetVersionCommand) Match(cases IrCommandCaces) {
	cases.GetVersion(c)
}
