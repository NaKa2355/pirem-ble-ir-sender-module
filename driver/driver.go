package driver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"tinygo.org/x/bluetooth"
)

type BLEService = map[string]bluetooth.DeviceCharacteristic

type BleIrDriver struct {
	adapter     *bluetooth.Adapter
	commandChan chan IrCommand
	wg          sync.WaitGroup
	drop        chan struct{}
}

func (d *BleIrDriver) SendIr(irdata []int16) error {
	resultChan := make(chan error)
	d.commandChan <- SendIrCommand{
		IrData: irdata,
		Result: resultChan,
	}
	// read errrorCode
	return <-resultChan
}

func (d *BleIrDriver) GetVersion() (string, error) {
	resultChan := make(chan GetVersionCommandResult)
	d.commandChan <- GetVersionCommand{
		Result: resultChan,
	}

	result := <-resultChan
	return result.Version, result.Err
}

func (d *BleIrDriver) Drop() {
	close(d.drop)
	d.wg.Wait()
}

type BleIrDevice struct {
	services map[string]BLEService
	device   bluetooth.Device
	buffer   bytes.Buffer
}

func NewBleIrDevice(device bluetooth.Device) (BleIrDevice, error) {
	d := BleIrDevice{
		services: make(map[string]BLEService),
		device:   device,
		buffer:   bytes.Buffer{},
	}
	srvcs, err := d.device.DiscoverServices(nil)
	if err != nil {
		fmt.Printf("%s\n", err)
		return d, err
	}

	for _, srvc := range srvcs {
		chars, err := srvc.DiscoverCharacteristics(nil)
		if err != nil {
			return d, err
		}
		service := make(BLEService)
		for _, char := range chars {
			service[char.UUID().String()] = char
		}
		d.services[srvc.UUID().String()] = service
	}

	return d, err
}

func NewBleIrDriverWithContext(ctx context.Context, address string) (*BleIrDriver, error) {
	var err error = nil
	driver := &BleIrDriver{
		adapter:     bluetooth.DefaultAdapter,
		commandChan: make(chan IrCommand),
		wg:          sync.WaitGroup{},
		drop:        make(chan struct{}),
	}

	driver.adapter.Enable()
	if err != nil {
		return driver, err
	}

	ch := make(chan struct {
		dev bluetooth.Device
		err error
	})

	err = driver.adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		if strings.EqualFold(result.Address.String(), address) {
			fmt.Println("connecting to... device:", result.Address.String(), result.RSSI, result.LocalName())
			dev, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
			if err == nil {
				ch <- struct {
					dev bluetooth.Device
					err error
				}{dev: dev, err: err}
			}
		}
	})

	if err != nil {
		return driver, err
	}

	rawDev := <-ch
	if rawDev.err != nil {
		return driver, rawDev.err
	}
	loadedDev, err := NewBleIrDevice(rawDev.dev)
	if err != nil {
		return driver, err
	}
	fmt.Println("device connected! dev: ", rawDev.dev.Address.String())

	go func() {
		dev := loadedDev
		for {
			select {
			case <-driver.drop:
				return
			case <-ch:
			case command := <-driver.commandChan:
				dev.handleCommand(command)
			}
		}
	}()
	return driver, err
}

func (d *BleIrDevice) handleCommand(command IrCommand) {
	command.Match(IrCommandCaces{
		SendIr: func(command SendIrCommand) {
			command.Result <- d.sendIr(command.IrData)
		},
		GetVersion: func(command GetVersionCommand) {
			version, err := d.getVersion()
			command.Result <- GetVersionCommandResult{
				Version: version,
				Err:     err,
			}
		},
	})
}

const IRDATA_CHUNK_SIZE = 20

var buf = make([]byte, 512)

func (d *BleIrDevice) setIrData(irData []int16) error {
	buffer := d.buffer
	buffer.Reset()
	if err := binary.Write(&buffer, binary.LittleEndian, irData); err != nil {
		fmt.Printf("err: %s\n", err.Error())
		return err
	}
	chunkCount := buffer.Len() / IRDATA_CHUNK_SIZE

	// チャンクサイズで割り切れない中途半端なデータ数の時
	if buffer.Len()%IRDATA_CHUNK_SIZE != 0 {
		chunkCount += 1
		buffer.Write(make([]byte, chunkCount*IRDATA_CHUNK_SIZE-buffer.Len()))
	}

	binaryData := buffer.Bytes()
	for i := 0; i < chunkCount; i++ {
		startIndex := i * IRDATA_CHUNK_SIZE
		endIndex := (i + 1) * IRDATA_CHUNK_SIZE
		uuid := fmt.Sprintf("114b00%02d-0866-e4c2-fb93-7da2e0dfa398", i)
		data := binaryData[startIndex:endIndex]
		_, err := d.services[IrServiceUUID][uuid].WriteWithoutResponse(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *BleIrDevice) setIrDataSize(irData []int16) error {
	buffer := d.buffer
	buffer.Reset()

	dataSize := int16(len(irData))
	err := binary.Write(&buffer, binary.LittleEndian, dataSize)
	if err != nil {
		fmt.Printf("err: %s\n", err.Error())
		return err
	}

	if _, err = d.services[IrServiceUUID][IrDataSizeUUID].WriteWithoutResponse(buffer.Bytes()); err != nil {
		return err
	}

	_, err = d.services[IrServiceUUID][IrSendUUID].WriteWithoutResponse([]byte{0})
	if err != nil {
		return err
	}

	return nil
}

func (d *BleIrDevice) readErrCode() error {
	var errCode int8 = 0
	buffer := d.buffer
	buffer.Reset()
	n, err := d.services[IrServiceUUID][IrSendErrUUID].Read(buf)
	if err != nil {
		return err
	}
	if _, err := buffer.Write(buf[:n]); err != nil {
		return err
	}
	if err = binary.Read(&buffer, binary.LittleEndian, &errCode); err != nil {
		return err
	}
	return convertErr(errCode)
}

func (d *BleIrDevice) sendIr(irdata []int16) error {
	if err := d.setIrData(irdata); err != nil {
		return err
	}

	if err := d.setIrDataSize(irdata); err != nil {
		return err
	}
	// read errrorCode
	return d.readErrCode()
}

func (d *BleIrDevice) getVersion() (string, error) {
	char := d.services[IrServiceUUID][FirmwareVersionUUID]
	fmt.Printf("%v", char)
	n, err := char.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), err
}
