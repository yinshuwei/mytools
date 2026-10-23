package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

func main() {
	mygo.App.WhenReady(func() {
		defer mygo.App.Quit()
		fmt.Println("formats:", mygo.Clipboard.Formats())
		data, err := mygo.Clipboard.Read()
		fmt.Println("read:", err)
		for _, item := range data.Items() {
			for _, f := range item.Formats() {
				b, err := item.Read(f)
				fmt.Printf("%s: %d bytes, %v\n", f, len(b), err)
				if f == transfer.PNG && err == nil {
					_ = os.WriteFile("/tmp/opencode/clip.png", b, 0o644)
				}
			}
		}
	})
	if err := mygo.App.Run(); err != nil {
		panic(err)
	}
}
