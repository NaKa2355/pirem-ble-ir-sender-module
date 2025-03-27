package driver

import "fmt"

const IrServiceUUID = "114b0000-0866-e4c2-fb93-7da2e0dfa398"
const FirmwareVersionUUID = "00002a26-0000-1000-8000-00805f9b34fb"
const IrDataSizeUUID = "114b00d0-0866-e4c2-fb93-7da2e0dfa398"
const IrSendUUID = "114b00d1-0866-e4c2-fb93-7da2e0dfa398"
const IrSendErrUUID = "114b00d2-0866-e4c2-fb93-7da2e0dfa398"

const (
	OK                   = 0
	ERR_INVAILD_DATA     = -1
	ERR_REQ_TIMEOUT      = -2
	ERR_DATA_TOO_LONG    = -3
	ERR_UNSUPPORTED_DATA = -4
)

var ErrInvaildData = fmt.Errorf("invaild data")
var ErrReqTimeout = fmt.Errorf("request time out")
var ErrDataTooLong = fmt.Errorf("data is too long")
var ErrUnsupportedData = fmt.Errorf("data is not supported")
var ErrNotConnected = fmt.Errorf("not connected")
var ErrorUnknown = fmt.Errorf("unknown error")

func convertErr(errorCode int8) error {
	switch errorCode {
	case OK:
		return nil
	case ERR_DATA_TOO_LONG:
		return ErrDataTooLong
	case ERR_INVAILD_DATA:
		return ErrInvaildData
	case ERR_REQ_TIMEOUT:
		return ErrReqTimeout
	case ERR_UNSUPPORTED_DATA:
		return ErrUnsupportedData
	default:
		return ErrorUnknown
	}
}
