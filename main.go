package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	Server "github.com/bob17/ftpver/Server"
)

func main() {
	var addr string = "127.0.0.1"
	var port int = 3636
	var maxWorker int = 1000
	var minWorker int = 10
	var maxDataPort int = 6669
	var minDataPort int = 6420

	serverInstance := Server.NewServer(
		addr,
		port,
		maxWorker,
		minWorker,
		maxDataPort,
		minDataPort,
	)

	err := serverInstance.Start()
	if err != nil {
		fmt.Println("Unable to start FTP server, try again")
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := serverInstance.ShutDown(ctx); err != nil {
		fmt.Printf("[SHUTDOWN] Error while shutting down server, %v \n", err)
	}
}
