package server

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type FTPCommand struct {
	name     string
	argument string
}

var helpMenuHolder = map[string]string{
	"CD":      "Navigate directory, { cd x, where x -> directory name }",
	"TOUCH":   "Create new file,  { touch x.ext, where x -> file name, ext -> file extension } ",
	"GET":     "------Currently in development------",
	"CWD":     "------Currently in development------",
	"PASSIVE": "Enter passive mode for file transfer and download",
	"QUIT":    "Disconnect client connection with server.",
	"HELP":    "------Currently in development------",
}

func parseCommand(str string) FTPCommand {
	cmd := strings.SplitN(strings.TrimSpace(str), " ", 2)
	named := FTPCommand{name: strings.ToUpper(cmd[0])}
	if len(cmd) > 1 {
		named.argument = cmd[1]
	}

	return named
}

func (S *FTPServer) handleClientCommand(c *FTPClient, cmd FTPCommand) error {
	fmt.Println(cmd.name, cmd.argument)
	switch cmd.name {
	case "CD":
		return S.handleNavigationCommand(c, cmd)
	case "LS":
		return S.listContentOfDir(c)
	case "MKDIR":
		return S.handleMKDIR(c, cmd.argument)
	case "PASSIVE":
		return S.PassiveConn(c)
	case "TOUCH":
		return S.handleTOUCH(c, cmd.argument)
	case "DOWNLOAD":
		return c.sendResponse(404, "In-progress")
	case "UPLOAD":
		return c.sendResponse(404, "In-progress...")
	case "CWD":
		return S.handleCWD(c)
	case "HELP":
		return c.handleHelpCommand()
	case "QUIT":
		return S.closeClientConnection(c)
	default:
		return nil
	}
}

func (S *FTPServer) handleNavigationCommand(c *FTPClient, cmd FTPCommand) error {
	S.navMu.RLock()
	rootDir := S.rootDir
	S.navMu.RUnlock()

	S.mu.Lock()
	defer S.mu.Unlock()

	switch cmd.argument {
	case "":
		return c.sendResponse(301, "Empty path given")
	case "/":
		c.cwd = S.rootDir
		fmt.Printf("[Navigation] > Current working directory: %s \n", c.cwd)
		return c.sendResponse(250, "Navigated to root directory")
	case "..":
		if c.cwd == S.rootDir {
			return c.sendResponse(450, "Restrict directory cannot be accessed")
		}

		c.cwd = filepath.Dir(c.cwd)
		return c.sendResponse(250, fmt.Sprintf("Directory changed, current working directory: { %s } \n", c.cwd))
	}

	newPath := filepath.Join(c.cwd, cmd.argument)
	newPath = filepath.Clean(newPath)
	if !strings.HasPrefix(newPath, "/") {
		newPath = "/" + newPath
		fmt.Println(rootDir)
	}

	exist := S.doesDirExist(newPath)
	if !exist {
		fmt.Println("[Navigation] > Directory doesn't exist")
		return c.sendResponse(404, fmt.Sprintf("[Navigation] > Directory: { %s } does not exist in server \n", newPath))
	}

	c.cwd = newPath
	return c.sendResponse(450, fmt.Sprintf("[Navigation] > Current working directory: { %s } \n", c.cwd))
}

func (S *FTPServer) listContentOfDir(c *FTPClient) error {
	var strBuilder strings.Builder
	fmt.Println("[DIR] > Current working directory for client :> ", c.cwd)
	contents, err := ioutil.ReadDir(c.cwd)
	if err != nil {
		fmt.Printf("[DIR] > Error while displaying content inside of directory: { %s } \n", c.cwd)
		return err
	}

	if len(contents) < 1 {
		strBuilder.WriteString("[Empty] > No content available inside of given directory")
	}

	for i, content := range contents {
		strBuilder.WriteString(fmt.Sprintf("\n [%d] > %-15s | %-10d | %-5s | %-1s \n", i+1, content.Mode(), content.Size(), content.ModTime(), content.Name()))
	}

	return c.sendResponse(201, strBuilder.String())
}

func (S *FTPServer) doesDirExist(dir string) bool {
	S.navMu.RLock()
	defer S.navMu.RUnlock()

	info, err := os.Stat(dir)
	if err != nil {
		fmt.Println("[NAVIGATION] > Error while validating directory", err.Error())
		return false
	}

	return info.IsDir()
}
