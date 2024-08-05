package server

import (
	"fmt"
	"net"
)

type FTPClient struct {
	conn net.Conn
	dir  string
}

func (C *FTPClient) sendResponse(statusCode int, msg string) error {
	_, err := fmt.Fprintf(C.conn, "[Status: %d] %s \n", statusCode, msg)
	return err
}
