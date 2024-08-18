package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type FTPServer struct {
	addr        string
	port        int
	listener    net.Listener
	clients     sync.Map
	rootDir     string
	workerPool  chan chan *FTPClient
	jobQueue    chan *FTPClient
	maxWorker   int
	minWorker   int
	workerCount int
	mu          sync.Mutex
	wg          sync.WaitGroup
	dispatchWg  sync.WaitGroup
	quit        chan bool

	workerDone chan struct{}
	navMu      sync.RWMutex

	dataPorts struct {
		max int
		min int
	}
	availableDataPorts chan int
}

func NewServer(
	addr string,
	port int,
	maxWorker int,
	minWorker int,
	maxDataPort int,
	minDataPort int,
) *FTPServer {
	server := &FTPServer{
		addr:       addr,
		port:       port,
		rootDir:    "/ftp/server/downstream/directory",
		maxWorker:  maxWorker,
		minWorker:  minWorker,
		workerPool: make(chan chan *FTPClient),
		jobQueue:   make(chan *FTPClient),
		workerDone: make(chan struct{}, maxWorker),

		dataPorts: struct {
			max int
			min int
		}{
			max: maxDataPort,
			min: minDataPort,
		},
		availableDataPorts: make(chan int, maxDataPort-minDataPort+1),
	}

	for port := minDataPort; port < maxDataPort; port++ {
		server.availableDataPorts <- port
	}

	return server
}

func (S *FTPServer) PassiveConn(c *FTPClient) error {
	port, err := S.getAvailableDataPorts()
	if err != nil {
		fmt.Println("[PASSIVE] > Error while trying to access port for Passive connection >", err.Error())
		return err
	}

	dataConnAddr := fmt.Sprintf("%s:%d", S.addr, port)
	fmt.Println("[PASSIVE] > Passive connection address >>: ", dataConnAddr)
	listener, err := net.Listen("tcp", dataConnAddr)
	if err != nil {
		fmt.Println("[PASSIVE] > Error while listening for passive connection >", err.Error())
		S.availableDataPorts <- port
		return err
	}

	go func() {
		defer listener.Close()
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("[PASSIVE] > Error while accpeting data connection >", err.Error())
			return
		}

		c.dataConn = conn
		c.passiveInput(S)
	}()

	return c.sendResponse(201, fmt.Sprintf("Entering passive mode at: [%s], CWD is: %s \n", dataConnAddr, c.cwd))
}

func (S *FTPServer) Start() error {
	formattedAddr := fmt.Sprintf("%s:%d", S.addr, S.port)
	listener, err := net.Listen("tcp", formattedAddr)
	if err != nil {
		fmt.Printf("Error while listening on addr %s, check subsequent logs for more information... \n", formattedAddr)
		fmt.Println(err.Error())
		return err
	}

	S.listener = listener
	fmt.Printf("[Started] > Server is up and running at address: %s \n", formattedAddr)

	for i := 0; i < S.minWorker; i++ {
		// allocating workers for later work
		// this run concurrently with other aspect of server
		// as for each worker, it creates new go routine
		S.allocateWorkers()
	}

	// pushing workers for work
	// this go routine runs concurrently with Acception of client request
	go S.dispatchClientToWorkers()
	go S.runAdjustWorkerPool()

	for {
		conn, err := S.listener.Accept()
		if err != nil {
			select {
			case <-S.quit:
				return nil
			default:
				fmt.Printf("Error encountered while accepting connection %s \n", err.Error())
				continue
			}
		}

		client := &FTPClient{conn: conn, cwd: S.rootDir}
		S.clients.Store(client.conn.RemoteAddr(), client)
		S.jobQueue <- client
		fmt.Printf("Connection accepted: [%s] \n", client.conn.RemoteAddr())
	}
}

func (S *FTPServer) runAdjustWorkerPool() {
	ticker := time.NewTicker(5 * time.Second)

	defer ticker.Stop()
	select {
	case <-ticker.C:
		S.adjustWorkerPool()
	case <-S.workerDone:
		S.reduceWorkerSize()
	case <-S.quit:
		return
	}
}

func (S *FTPServer) adjustWorkerPool() {
	S.mu.Lock()
	defer S.mu.Unlock()

	queueLength := len(S.jobQueue)
	switch {
	case queueLength > 10 && S.workerCount < S.maxWorker:
		for i := 0; i < 5 && S.workerCount < S.maxWorker; i++ {
			S.allocateWorkers()
		}
	case queueLength == 10 && S.workerCount > S.minWorker:
		diff := S.minWorker - S.workerCount
		for i := 0; i < diff && i < 5; i++ {
			S.quit <- true
			S.workerCount--
		}
	}
}

func (S *FTPServer) allocateWorkers() {
	S.mu.Lock()
	S.workerCount++
	S.mu.Unlock()

	S.wg.Add(1)
	go func() {
		defer S.wg.Done()
		clients := make(chan *FTPClient)
		for {
			// appends client to worker pool
			S.workerPool <- clients
			select {
			// use that client as a single object, one at a time
			case c := <-clients:
				// when a new clients joins, this case will execute, as it contains newly connected client, in a single object
				S.handleClientConn(c)
			case <-S.quit:
				return
			}
		}
	}()
}

func (S *FTPServer) handleClientConn(c *FTPClient) error {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Println("[PANIC] > Recovering from panic inside of handleClientConn(c *FTPClient) function of FTPServer")
			fmt.Printf("[PANIC] > %v \n", rec)
		}

		S.gracefullyDisconnect(c)
	}()

	c.sendResponse(201, "Successfully connected to FTP server, welcome...")
	wreader := bufio.NewReader(c.conn)
	for {
		// connection timeout
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Minute))
		str, err := wreader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Printf("[READ] > Client %v closed connection \n", c.conn.RemoteAddr())
				c.sendResponse(401, "Closing connection, thank you for using...")
				return err
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Printf("[READ] > Client %v timeout, connection closing \n", c.conn.RemoteAddr())
				c.sendResponse(401, "Closing connection, thank you for using...")
				return err
			} else {
				fmt.Printf("[READ] > Error: %v \n", err)
				return err
			}
		}

		cmd := parseCommand(str)
		S.handleClientCommand(c, cmd)
	}
}

func (S *FTPServer) dispatchClientToWorkers() {
	// best example of shared resource with threads
	S.dispatchWg.Add(1)

	defer S.dispatchWg.Done()
	for {
		select {
		case client := <-S.jobQueue:
			worker := <-S.workerPool

			S.dispatchWg.Add(1)
			go func() {
				defer S.dispatchWg.Done()
				worker <- client
			}()
		case <-S.quit:
			return
		}
	}
}

func (S *FTPServer) gracefullyDisconnect(C *FTPClient) {
	S.mu.Lock()
	defer S.mu.Unlock()

	if C.dataConn != nil {
		C.dataConn.Close()
	}

	if C.dataConnPort != 0 {
		S.availableDataPorts <- C.dataConnPort
	}

	C.conn.Close()
	S.clients.Delete(C.conn.RemoteAddr())

	fmt.Printf("[DISCONNECT] > Disconnected client: %s \n", C.conn.RemoteAddr())
	select {
	case S.workerDone <- struct{}{}:
		fmt.Printf("[Disconnect] > Client discconected \n")
	default:
		fmt.Println("Channel is full")
	}
}

func (S *FTPServer) reduceWorkerSize() {
	S.mu.Lock()
	S.workerCount--
	S.mu.Unlock()

	if len(S.jobQueue) == 0 && S.workerCount < S.minWorker {
		S.workerCount--

		select {
		case S.quit <- true:
		default:
		}
	}
}

func (S *FTPServer) ShutDown(ctx context.Context) error {
	close(S.quit)
	S.listener.Close()

	done := make(chan struct{})
	go func() {
		S.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (S *FTPServer) getAvailableDataPorts() (int, error) {
	select {
	case port := <-S.availableDataPorts:
		return port, nil
	default:
		return 0, fmt.Errorf("port not available")
	}
}
