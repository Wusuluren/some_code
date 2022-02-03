package signal

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

type CleanupHandler func()

func OnSignal(handlers ...CleanupHandler) {
	exit := make(chan os.Signal)
	signal.Notify(exit, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	signal := <-exit
	fmt.Printf("receive signal: %s\n", signal)
	for _, handler := range handlers {
		if handler != nil {
			handler()
		}
	}
}
