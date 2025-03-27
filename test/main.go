package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NaKa2355/pirem-ble-ir-sender-module/driver"
)

func main() {
	dev, err := driver.NewBleIrDriverWithContext(context.Background(), "D4:4D:1F:FC:29:86")
	if err != nil {
		fmt.Println("error", err)
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		if input == "exit" {
			fmt.Println("shutting down...")
			dev.Drop()
			fmt.Println("bye")
			return
		}
		if input == "send" {
			fmt.Println("sending...")
			err = dev.SendIr(testIrData)
			if err != nil {
				fmt.Println("error", err)
				continue
			}
			fmt.Println("sended")
		}
		if input == "version" {
			version, err := dev.GetVersion()
			if err != nil {
				fmt.Println("error", err)
				continue
			}
			fmt.Println("version:", version)
		}
	}
}

var testIrData = []int16{
	3476,
	1752,
	424,
	472,
	388,
	480,
	384,
	1328,
	420,
	1316,
	412,
	460,
	416,
	1316,
	416,
	468,
	396,
	468,
	388,
	484,
	416,
	1292,
	444,
	440,
	396,
	476,
	388,
	1316,
	388,
	508,
	392,
	1316,
	420,
	472,
	388,
	1328,
	412,
	452,
	412,
	476,
	392,
	1324,
	416,
	472,
	388,
	476,
	388,
	452,
	412,
	480,
	396,
	444,
	416,
	1348,
	392,
	452,
	412,
	476,
	388,
	444,
	416,
	476,
	388,
	464,
	396,
	468,
	400,
	444,
	420,
	444,
	420,
	476,
	388,
	1340,
	400,
	476,
	360,
	496,
	400,
	468,
	400,
	468,
	388,
	484,
	412,
	1296,
	420,
	468,
	400,
	468,
	396,
	464,
	400,
	468,
	392,
	476,
	420,
	1296,
	412,
	30204,
	3472,
	1756,
	420,
	472,
	388,
	476,
	392,
	1324,
	412,
	1316,
	416,
	476,
	388,
	1324,
	412,
	472,
	388,
	452,
	412,
	476,
	388,
	1328,
	416,
	472,
	388,
	476,
	392,
	1324,
	420,
	468,
	396,
	1320,
	416,
	472,
	396,
	1320,
	444,
	444,
	388,
	476,
	396,
	1340,
	400,
	468,
	388,
	472,
	392,
	476,
	392,
	476,
	388,
	476,
	396,
	1316,
	420,
	468,
	392,
	472,
	388,
	476,
	384,
	476,
	388,
	476,
	388,
	452,
	412,
	476,
	392,
	448,
	412,
	476,
	392,
	1324,
	412,
	476,
	388,
	456,
	412,
	452,
	416,
	476,
	388,
	448,
	412,
	1328,
	412,
	452,
	412,
	476,
	412,
	428,
	436,
	452,
	392,
	448,
	412,
	1344,
	388,
}
