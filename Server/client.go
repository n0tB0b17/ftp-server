package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type FTPClient struct {
	conn         net.Conn
	dataListener net.Listener
	dataConn     net.Conn
	dataConnPort int
	cwd          string
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

func (S *FTPServer) handleMKDIR(c *FTPClient, dirName string) error {
	if len(dirName) < 1 {
		return c.sendResponse(201, fmt.Sprintf("Invalid directory name provided: [%s] \n", dirName))
	}

	S.navMu.Lock()
	filePath := filepath.Join(c.cwd, dirName)
	S.navMu.Unlock()

	if !strings.HasPrefix(filePath, S.rootDir) {
		fmt.Println("[MKDIR] > Invalid filePath...")
		return c.sendResponse(501, "Access denied")
	}

	err := os.MkdirAll(filePath, 0755)
	if err != nil {
		fmt.Println("[MKDIR] > Error while creating new directory named >> ", dirName)
		return c.sendResponse(501, fmt.Sprintf("Unable to create new directory in cwd:> %s \n", c.cwd))
	}

	return c.sendResponse(201, fmt.Sprintf("Successfully created new directory named: %s at path: %s \n", dirName, filePath))
}

func (S *FTPServer) handleTOUCH(c *FTPClient, fileName string) error {
	if len(fileName) < 1 {
		return c.sendResponse(201, fmt.Sprintf("Invalid file name provided: [%s]", fileName))
	}

	S.navMu.Lock()
	filePath := filepath.Join(c.cwd, fileName)
	S.navMu.Unlock()

	if !strings.HasPrefix(filePath, S.rootDir) {
		fmt.Println("[TOUCH] > Invalid filePath...")
		return c.sendResponse(501, "Access denied")
	}

	f, err := os.OpenFile(filePath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("[TOUCH] > Error while trying to create a new file", err.Error())
		return c.sendResponse(550, fmt.Sprintf("[ERROR] > While trying to create a new file named: [%s] at cwd: {%s} \n", fileName, c.cwd))
	}

	f.Close()
	return c.sendResponse(201, fmt.Sprintf("Successfully created new file named: [%s] at path: { %s } \n", fileName, filePath))
}

func (S *FTPServer) handleCWD(c *FTPClient) error {
	return c.sendResponse(201, fmt.Sprintf("Current Working Directory |> %s", c.cwd))
}
