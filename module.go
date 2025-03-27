package Module

import (
	"context"
	"encoding/json"

	"github.com/NaKa2355/pirem-ble-ir-sender-module/driver"
	plugin "github.com/NaKa2355/pirem/pkg/driver_module/v1"
)

type DeviceConfig struct {
	MacAddress string `json:"mac_address"`
}

func convertErr(err error) error {
	if err == nil {
		return nil
	}
	switch err {
	case driver.ErrDataTooLong:
		return plugin.WrapErr(plugin.CodeInvaildInput, err)
	case driver.ErrInvaildData:
		return plugin.WrapErr(plugin.CodeInvaildInput, err)
	case driver.ErrReqTimeout:
		return plugin.WrapErr(plugin.CodeTimeout, err)
	case driver.ErrUnsupportedData:
		return plugin.WrapErr(plugin.CodeInvaildInput, err)
	default:
		return plugin.WrapErr(plugin.CodeUnknown, err)
	}
}

type Device struct {
	driver *driver.BleIrDriver
	info   plugin.DeviceInfo
}

var _ plugin.Device = &Device{}
var _ plugin.Sender = &Device{}

func newDevice(jsonConf json.RawMessage) (dev *Device, err error) {
	dev = &Device{}
	conf := DeviceConfig{}
	err = json.Unmarshal(jsonConf, &conf)
	if err != nil {
		return dev, plugin.WrapErr(plugin.CodeInvaildInput, err)
	}

	d, err := driver.NewBleIrDriverWithContext(context.Background(), conf.MacAddress)
	if err != nil {
		return dev, err
	}

	dev.driver = d
	dev.info.FirmwareVersion = "0.1.0"
	dev.info.DriverVersion = "0.1.0"

	return dev, nil
}

func (dev *Device) GetInfo(ctx context.Context) (*plugin.DeviceInfo, error) {
	return &dev.info, nil
}

func (dev *Device) Drop() error {
	dev.driver.Drop()
	return nil
}

func (dev *Device) SendIR(ctx context.Context, irData *plugin.IRData) error {
	err := dev.driver.SendIr(convertToDriverIrRawData((irData.PluseNanoSec)))
	return convertErr(err)
}

type Module struct{}

var _ plugin.DriverModule = &Module{}

func (p *Module) LoadDevice(conf json.RawMessage) (plugin.Device, error) {
	dev, err := newDevice(conf)
	return dev, err
}
