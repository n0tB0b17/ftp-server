package server

import (
	"fmt"
	"net"
	"strings"
)

type FTPClient struct {
	conn net.Conn
	dir  string
}

func (C *FTPClient) sendResponse(statusCode int, msg string) error {
	_, err := fmt.Fprintf(C.conn, "[Status: %d] %s \n", statusCode, msg)
	return err
}

// when quit command is send.
func (S *FTPServer) closeClientConnection(C *FTPClient) error {
	C.sendResponse(400, "Good bye, connection closed")

	S.gracefullyDisconnect(C)
	return nil
}

func (C *FTPClient) handleHelpCommand() error {
	var strBuilder strings.Builder
	strBuilder.WriteString("Following is help manual for FTP server \n")
	for cmd, desp := range helpMenuHolder {
		strBuilder.WriteString(fmt.Sprintf("%-15s- %s \n", cmd, desp))
	}

	return C.sendResponse(201, strBuilder.String())
}
