package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FTPClient struct {
	conn         net.Conn
	dataConn     net.Conn
	dataConnPort int
	cwd          string
}

func (C *FTPClient) sendResponse(statusCode int, msg string) error {
	if C.dataConn != nil {
		_, err := fmt.Fprintf(C.dataConn, "[Status: %d] > %s \n", statusCode, msg)
		return err
	}
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

// related to data connection
func (C *FTPClient) passiveInput(S *FTPServer) error {
	if C.dataConn == nil {
		fmt.Println("[Passive] > Passive connection has not been established")
		return fmt.Errorf("data connection not established")
	}

	defer func() {
		C.resetDataConnection(S)
	}()
	fmt.Println("[PASSIVE] > Listening on data connection at :>>", C.dataConn.RemoteAddr())
	pasv := bufio.NewReader(C.dataConn)
	for {
		line, err := pasv.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Printf("Closing connection with client running at: %s \n", C.dataConn.RemoteAddr())
				C.sendResponse(201, "Closing data connection with FTP server \n")
				return err
			} else if netError, ok := err.(net.Error); ok && netError.Timeout() {
				fmt.Printf("Network timeout with data connection with server: %s \n", C.dataConn.RemoteAddr())
				C.sendResponse(201, "Closing data connection with FTP server with unexpected error. \n")
				return err
			} else {
				fmt.Println("[PASSIVE] > Err: ", err.Error())
				return err
			}
		}

		// handle command here
		fmt.Printf("[PASSIVE | CMD] > Passive command: %s \n", line)
		cmd := parseCommand(line)
		C.handlePassiveCommand(cmd, S)

	}
}

func (C *FTPClient) handlePassiveCommand(cmd FTPCommand, S *FTPServer) error {
	fmt.Println(cmd.name, cmd.argument)

	switch cmd.name {
	case "ADD":
		if C.dataConn == nil && C.dataConnPort == 0 {
			return C.sendResponse(404, "Activate passive mode before running ADD command")
		}
		return C.add(cmd, S)
	case "DOWNLOAD":
		if C.dataConn == nil && C.dataConnPort == 0 {
			return C.sendResponse(404, "Activate passive mode before running DOWNLOAD command")
		}

		return C.get(cmd, S)
	case "QUIT":
		return C.resetDataConnection(S)
	default:
		return C.sendResponse(404, "Invalid passive command")
	}
}

func (C *FTPClient) get(cmd FTPCommand, S *FTPServer) error {
	// implement, validate if file exist on server
	if cmd.argument == "" {
		return C.sendResponse(404, "Please provide valid file name to download")
	}

	fullPath := path.Join(C.cwd, cmd.argument)
	fmt.Printf("Download file name with path: %s \n", fullPath)

	if !strings.HasPrefix(fullPath, S.rootDir) {
		return C.sendResponse(404, "unathorized path")
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return C.sendResponse(404, "File doesn't exist")
		}
		C.sendResponse(201, "Invalid file name provided")
	}

	if fileInfo.IsDir() {
		return C.sendResponse(404, "Cannot download a directory")
	}

	file, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("[GET] Error while reading file content at: %s \n", fullPath)
		return C.sendResponse(404, "unable to read file's content...")
	}

	defer file.Close()

	bufSize := 32 * 1024 // 32KB
	buffer := make([]byte, bufSize)
	writer := bufio.NewWriter(C.dataConn)
	reader := bufio.NewReader(file)
	byteTransferred := 0

	for {
		n, err := reader.Read(buffer)
		if err != nil && err != io.EOF {
			return C.sendResponse(550, "error reading file")
		}

		if n == 0 {
			break
		}

		_, err = writer.Write(buffer[:n])
		if err != nil {
			fmt.Println("[GET | WRITER]", err.Error())
			return C.sendResponse(426, "Transfer aborted...")
		}

		byteTransferred += n

		if byteTransferred%(bufSize*10) == 0 {
			err = writer.Flush()
			if err != nil {
				fmt.Println("[GET | WRITER | FLUSH]", err.Error())
				return C.sendResponse(404, "Transfer aborted...")
			}
		}
	}

	err = writer.Flush()
	if err != nil {
		fmt.Println("[GET | WRITER | FLUSH]", err.Error())
		return C.sendResponse(404, "Transfer aborted...")
	}

	return C.sendResponse(226, fmt.Sprintf("File transfer completed, total number of bytes send: [%d] \n", byteTransferred))
}

func (C *FTPClient) add(cmd FTPCommand, S *FTPServer) error {
	if cmd.argument == "" {
		return C.sendResponse(404, "Please provide file to upload to FTP server")
	}

	localPath := strings.TrimSpace(cmd.argument)
	fileName := filepath.Base(localPath)
	fullPath := path.Join(C.cwd, fileName)
	fmt.Printf("Uploading file: [%s] to given path: { %s }", fileName, fullPath)
	if !strings.HasPrefix(fullPath, S.rootDir) {
		return C.sendResponse(404, "unauthorized path")
	}
	C.sendResponse(201, "Ok to send file...")

	file, err := os.Create(fullPath)
	if err != nil {
		fmt.Println("Error while creating path in server >> ", err.Error())
		return C.sendResponse(404, "unable to create file")
	}
	defer file.Close()

	written, err := io.Copy(file, C.dataConn)
	if err != nil {
		fmt.Println("Error while writing client's data content to server's", err.Error())
		return C.sendResponse(404, "unable to read")
	}

	return C.sendResponse(201, fmt.Sprintf("Total number of bytes send to server: %d \n", written))
}

func (C *FTPClient) resetDataConnection(S *FTPServer) error {
	if C.dataConn != nil {
		C.dataConn.Close()
		C.dataConn = nil
	}

	if C.dataConnPort != 0 {
		S.availableDataPorts <- C.dataConnPort
	}

	return C.sendResponse(201, "[DATA-CONN] > connection closed")
}
