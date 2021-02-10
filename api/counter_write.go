package api

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"strings"
)

type CounterWrite struct {
	Total uint64
}

func (cw *CounterWrite) Write(p []byte) (int, error) {
	n := len(p)
	cw.Total += uint64(n)
	cw.PrintProgress()
	return n, nil
}

func (cw *CounterWrite) PrintProgress() {
	fmt.Printf("\r%s", strings.Repeat(" ", 50))
	fmt.Printf("\rDownloading... %s complete", humanize.Bytes(cw.Total))
}
