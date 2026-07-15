package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// usage prints the correct command-line usage instructions for the application,
// detailing flags for both server and client modes, and terminates the process with an error code.
func usage() {
	fmt.Println("Usage:")
	fmt.Println("  spark.exe -b <ip> <port> [path] [-v]       (server mode, optional path, optional verbose)")
	fmt.Println("  spark.exe -l <ip> <port>                  (list remote files)")
	fmt.Println("  spark.exe -d <ip> <port> <nums> [-o path] (download files, optional output path)")
	os.Exit(1)
}

// listFiles reads the specified directory path and returns a slice of strings
// containing only the names of the files present, excluding directories.
func listFiles(basePath string) []string {
	files := []string{}
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return files
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files
}

// handleConnection manages the lifecycle of an incoming network connection to the server.
// It reads the initial command from the client: if 'L', it sends a numbered list of files;
// if 'D', it reads the requested file index, validates it, and transmits the filename, size, and file payload.
func handleConnection(conn net.Conn, basePath string) {
	defer conn.Close()
	
	remoteAddr := conn.RemoteAddr().String()
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remoteAddr = tcpAddr.IP.String()
	}

	reader := bufio.NewReader(conn)
	cmd, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	switch cmd[:1] {
	case "L":
		fmt.Printf("Incoming: %s ; List\n", remoteAddr)
		files := listFiles(basePath)
		for i, name := range files {
			fmt.Fprintf(conn, "%d: %s\n", i+1, name)
		}
	case "D":
		indexStr, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(conn, "ERR: invalid index\n")
			return
		}
		index, err := strconv.Atoi(strings.TrimSpace(indexStr))
		if err != nil {
			fmt.Fprintf(conn, "ERR: invalid index\n")
			return
		}
		files := listFiles(basePath)
		if index < 1 || index > len(files) {
			fmt.Fprintf(conn, "ERR: index out of range\n")
			return
		}
		fname := files[index-1]
		
		fmt.Printf("Incoming: %s ; Download %s\n", remoteAddr, fname)

		fullPath := filepath.Join(basePath, fname)
		file, err := os.Open(fullPath)
		if err != nil {
			fmt.Fprintf(conn, "ERR: failed to open file\n")
			return
		}
		defer file.Close()

		fmt.Fprintln(conn, fname)
		info, _ := file.Stat()
		fmt.Fprintln(conn, info.Size())
		io.Copy(conn, file)
	default:
		fmt.Printf("Incoming: %s ; Unknown Command (%s)\n", remoteAddr, cmd[:1])
		fmt.Fprintf(conn, "ERR: unknown command\n")
	}
}

// runServer starts the TCP server listening on the provided IP and port.
// If verbose mode is enabled, it prints the local list of available files.
// It keeps accepting new incoming connections in an infinite loop, routing them to concurrent goroutines.
func runServer(ip, port, basePath string, verbose bool) {
	addr := ip + ":" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println("Failed to bind:", err)
		os.Exit(1)
	}
	fmt.Printf("Listening on %s (Serving: %s)\n", addr, basePath)

	if verbose {
		fmt.Println("\nAvailable Files:")
		files := listFiles(basePath)
		for i, name := range files {
			fmt.Printf("  %d: %s\n", i+1, name)
		}
		fmt.Println()
	}

	for {
		conn, err := ln.Accept()
		if err == nil {
			go handleConnection(conn, basePath)
		}
	}
}

// runList acts as a client by connecting to the server and sending a list command ('L').
// It prints the received numbered catalog of files line by line onto the local console.
func runList(ip, port string) {
	conn, err := net.Dial("tcp", ip+":"+port)
	if err != nil {
		fmt.Println("Failed to connect:", err)
		return
	}
	defer conn.Close()
	fmt.Fprint(conn, "L\n")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

// runDownload manages the process of downloading a single file by its list index.
// It establishes a connection, requests the file index, automatically creates the target output directory if missing,
// creates the local file, and updates a visual progress bar as the byte stream is written.
func runDownload(ip, port, num, outputPath string, wg *sync.WaitGroup, p *mpb.Progress) {
	defer wg.Done()

	conn, err := net.Dial("tcp", ip+":"+port)
	if err != nil {
		fmt.Printf("Connection error (File %s): %v\n", num, err)
		return
	}
	defer conn.Close()

	fmt.Fprint(conn, "D\n")
	fmt.Fprint(conn, num+"\n")

	reader := bufio.NewReader(conn)

	fname, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading filename (File %s): %v\n", num, err)
		return
	}
	fname = strings.TrimSpace(fname)

	sizeStr, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading file size (File %s): %v\n", num, err)
		return
	}
	sizeStr = strings.TrimSpace(sizeStr)
	totalSize, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		fmt.Printf("Error processing file size (File %s): %v\n", num, err)
		return
	}

	if outputPath != "." {
		err = os.MkdirAll(outputPath, os.ModePerm)
		if err != nil {
			fmt.Printf("Error creating directory (%s): %v\n", outputPath, err)
			return
		}
	}

	destPath := filepath.Join(outputPath, fname)
	out, err := os.Create(destPath)
	if err != nil {
		fmt.Printf("Error creating destination file (%s): %v\n", destPath, err)
		return
	}
	defer out.Close()

	fmt.Printf("\nFile: %s (ID: %s)\nTotal size: %s bytes\n\n", fname, num, sizeStr)

	// Initialize the progress bar
	bar := p.AddBar(totalSize,
		mpb.PrependDecorators(
			decor.Name(fmt.Sprintf("[ID: %s] Downloading: ", num)),
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.Percentage(),
		),
	)
	// ---------------------

	proxyReader := bar.ProxyReader(reader)
	defer proxyReader.Close()

	_, err = io.Copy(out, proxyReader)
	if err != nil {
		fmt.Printf("Error during data stream download (%s): %v\n", fname, err)
	}
}

// runDownloadMulti splits the comma-separated string of file indices into individual values
// and triggers concurrent runDownload operations using a WaitGroup to synchronize their completion.
func runDownloadMulti(ip, port, nums, outputPath string) {
	numList := strings.Split(nums, ",")
	var wg sync.WaitGroup

	p := mpb.New()

	for _, num := range numList {
		wg.Add(1)
		go runDownload(ip, port, strings.TrimSpace(num), outputPath, &wg, p)
	}

	wg.Wait()
	p.Wait()
	
	fmt.Println("\nAll downloads finished.")
}

// main serves as the entry point of the application. It parses command-line arguments,
// validates the execution structure, and routes the control flow to server mode (-b), list mode (-l), or download mode (-d).
func main() {
	args := os.Args
	if len(args) < 4 {
		usage()
	}

	switch args[1] {
	case "-b":
		basePath := "."
		verbose := false
		remainingArgs := args[4:]
		for _, arg := range remainingArgs {
			if arg == "-v" {
				verbose = true
			} else {
				basePath = arg
			}
		}
		runServer(args[2], args[3], basePath, verbose)

	case "-l":
		runList(args[2], args[3])

	case "-d":
		if len(args) < 5 {
			usage()
		}

		outputPath := "."
		
		for i := 5; i < len(args); i++ {
			if args[i] == "-o" && i+1 < len(args) {
				outputPath = args[i+1]
				break
			}
		}

		runDownloadMulti(args[2], args[3], args[4], outputPath)

	default:
		usage()
	}
}