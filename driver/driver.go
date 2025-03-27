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

func connect(ctx context.Context, adapter *bluetooth.Adapter, address string) (bluetooth.Device, error) {
	ch := make(chan bluetooth.ScanResult, 1)
	var device bluetooth.Device

	err := adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		if strings.EqualFold(result.Address.String(), address) {
			fmt.Println("connecting to... device:", result.Address.String(), result.RSSI, result.LocalName())
			adapter.StopScan()
			ch <- result
		}
	})
	if err != nil {
		return device, err
	}
	select {
	case <-ctx.Done():
		return device, ctx.Err()
	case result := <-ch:
		return adapter.Connect(result.Address, bluetooth.ConnectionParams{})
	}
}

type BleIrDriver struct {
	adapter     *bluetooth.Adapter
	services    map[string]BLEService
	device      bluetooth.Device
	buffer      bytes.Buffer
	commandChan chan IrCommand
	wg          sync.WaitGroup
	drop        chan struct{}
}

func NewBleIrDriverWithContext(ctx context.Context, address string) (*BleIrDriver, error) {
	var err error = nil
	driver := &BleIrDriver{
		adapter:     bluetooth.DefaultAdapter,
		services:    make(map[string]BLEService),
		buffer:      bytes.Buffer{},
		commandChan: make(chan IrCommand),
		wg:          sync.WaitGroup{},
		drop:        make(chan struct{}),
	}

	driver.adapter.Enable()
	if err != nil {
		return driver, err
	}

	reConnectChan := make(chan struct{})

	driver.adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		driver.wg.Add(1)
		go func() {
			defer driver.wg.Done()
			if !connected {
				fmt.Println("Device Disconnected: ", device.Address.String())
				timeout := bluetooth.NewDuration(time.Second)
				reConnectChan <- struct{}{}
				for {
					select {
					case <-driver.drop:
						return
					default:
						_, err := driver.adapter.Connect(device.Address, bluetooth.ConnectionParams{Timeout: timeout})
						if err == nil {
							reConnectChan <- struct{}{}
							return
						}
					}
				}
			}
			fmt.Println("Device Connected: ", device.Address.String())

			for {
				select {
				case <-driver.drop:
					return
				case <-reConnectChan:
					return
				case command := <-driver.commandChan:
					driver.handleCommand(command)
				}
			}
		}()
	})

	driver.device, err = connect(ctx, driver.adapter, address)
	if err != nil {
		return driver, err
	}

	srvcs, err := driver.device.DiscoverServices(nil)
	if err != nil {
		fmt.Printf("%s\n", err)
		return driver, err
	}

	for _, srvc := range srvcs {
		chars, err := srvc.DiscoverCharacteristics(nil)
		if err != nil {
			return driver, err
		}
		service := make(BLEService)
		for _, char := range chars {
			service[char.UUID().String()] = char
		}
		driver.services[srvc.UUID().String()] = service
	}
	return driver, err
}

func (d *BleIrDriver) handleCommand(command IrCommand) {
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

func (d *BleIrDriver) setIrData(irData []int16) error {
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

func (d *BleIrDriver) setIrDataSize(irData []int16) error {
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

func (d *BleIrDriver) readErrCode() error {
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

func (d *BleIrDriver) sendIr(irdata []int16) error {
	if err := d.setIrData(irdata); err != nil {
		return err
	}

	if err := d.setIrDataSize(irdata); err != nil {
		return err
	}
	// read errrorCode
	return d.readErrCode()
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

func (d *BleIrDriver) getVersion() (string, error) {
	char := d.services[IrServiceUUID][FirmwareVersionUUID]
	fmt.Printf("%v", char)
	n, err := char.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), err
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
