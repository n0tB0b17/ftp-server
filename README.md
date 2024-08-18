# Dummy FTP Server

FTP server is completely based on Golang, created to learn about network programming, concurrency in Golang. 

Server creates two connection.
1. **Control connection** (for command exchange)
2. **Data connection** (for actual data transfer)

These two connection runs on different ports.

Control connection uses concurrency to handle multiple client connection without having any performance issue, beside that, it also has option to specifying minimum and maximum number of workers to handle client connection, tune worker's number properly to achieve optimal performance.  

## Running server
As this server do not contains any third party libraries, server can be started very easily.

1. Inside of project directory
```bash
go build
```

2. Running the actual server's binary, which was just generated from above command.
```bash

./name #linux
.\name.exe #windows

```

## Usages
Many of server's parameters can be customized according to users own requirements.
like:

1. Changing server's address and port.
2. Changing FTP server's downstream directory.
3. Changing minimum and maximum workers number.

After server is up and running, Enter command ```help``` for more information.

##