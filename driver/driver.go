package driver

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

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
	device   *bluetooth.Device
	buffer   bytes.Buffer
}

func newBleIrDevice() BleIrDevice {
	d := BleIrDevice{
		services: make(map[string]BLEService),
		device:   nil,
		buffer:   bytes.Buffer{},
	}
	return d
}

func (d *BleIrDevice) connect(device *bluetooth.Device) {
	d.device = device
}

func (d *BleIrDevice) scanService() error {
	srvcs, err := d.device.DiscoverServices(nil)
	if err != nil {
		fmt.Printf("%s\n", err)
		return err
	}

	for _, srvc := range srvcs {
		chars, err := srvc.DiscoverCharacteristics(nil)
		if err != nil {
			return err
		}
		service := make(BLEService)
		for _, char := range chars {
			service[char.UUID().String()] = char
		}
		d.services[srvc.UUID().String()] = service
	}

	return err
}

type scanAndConnectResult struct {
	dev BleIrDevice
	err error
}

func scanAndConnect(adapter *bluetooth.Adapter, strAddr string) (BleIrDevice, error) {
	dev := newBleIrDevice()
	ch := make(chan bluetooth.ScanResult, 1)
	err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		if strings.EqualFold(result.Address.String(), strAddr) {
			fmt.Println("connecting to... device:", result.Address.String(), result.RSSI, result.LocalName())
			ch <- result
			adapter.StopScan()
		}
	})

	if err != nil {
		return dev, err
	}

	scanResult := <-ch
	rawDev, err := adapter.Connect(scanResult.Address, bluetooth.ConnectionParams{})
	if err != nil {
		return dev, err
	}
	dev.connect(&rawDev)
	err = dev.scanService()
	return dev, err
}

func NewBleIrDriverWithContext(ctx context.Context, strAddr string) (*BleIrDriver, error) {
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

	ch := make(chan scanAndConnectResult, 1)

	go func() {
		println("start scan")
		dev, err := scanAndConnect(driver.adapter, strAddr)
		ch <- scanAndConnectResult{dev: dev, err: err}
		println("connected!")
	}()

	driver.wg.Add(1)
	go func() {
		defer driver.wg.Done()
		var dev *BleIrDevice = &BleIrDevice{}
		ticker := time.NewTicker(2 * time.Second)

		for {
			select {
			case scanResult := <-ch:
				if scanResult.err == nil {
					dev = &scanResult.dev
				}
			case <-driver.drop:
				return
			case command := <-driver.commandChan:
				dev.handleCommand(command)
			case <-ticker.C:
				if dev.device == nil {
					continue
				}
				rawDev, err := driver.adapter.Connect(dev.device.Address, bluetooth.ConnectionParams{})
				if err != nil {
					continue
				}
				dev.connect(&rawDev)
			}
		}
	}()
	return driver, err
}

func (d *BleIrDevice) handleCommand(command IrCommand) error {
	return command.Match(IrCommandCaces{
		SendIr: func(command SendIrCommand) error {
			err := d.sendIr(command.IrData)
			command.Result <- err
			return err
		},
		GetVersion: func(command GetVersionCommand) error {
			version, err := d.getVersion()
			command.Result <- GetVersionCommandResult{
				Version: version,
				Err:     err,
			}
			return err
		},
	})
}

const IRDATA_CHUNK_SIZE = 20

var buf = make([]byte, 512)

func (d *BleIrDevice) setIrData(irData []int16) error {
	if len(irData) > 600 {
		return ErrDataTooLong
	}
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
		uuid := fmt.Sprintf("114b00%02x-0866-e4c2-fb93-7da2e0dfa398", i)
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
	if d.device == nil {
		return ErrNotConnected
	}
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
	if d.device == nil {
		return "", ErrNotConnected
	}
	char := d.services[IrServiceUUID][FirmwareVersionUUID]
	n, err := char.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), err
}
