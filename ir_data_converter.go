package Module

import (
	"math"
	"time"
)

func convertToDriverIrRawData(irData []uint32) []int16 {
	driverIrData := make([]int16, len(irData))
	for i, pluse := range irData {
		pluse = pluse / uint32(time.Microsecond)
		if pluse > math.MaxInt16 {
			driverIrData[i] = int16(pluse/1000) * -1
		} else {
			driverIrData[i] = int16(pluse)
		}
	}
	return driverIrData
}
