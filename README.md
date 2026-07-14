# Spark - Windows to Windows File Transfer

<img width="1408" height="768" alt="spark_Logo" src="https://github.com/user-attachments/assets/dc029205-c0c5-4240-8af0-397a59964dea" />




Spark is a lightweight tool written in Go that allows you to transfer one or multiple files from one Windows machine to another. Because it is written in Go, it compiles into a single, self-contained binary that runs in most Windows environments with absolutely no external dependencies.

---

## The Problem

At some point, especially if you are into cybersecurity CTFs (Capture The Flag) or Active Directory labs, you may have come across the situation where you need to transfer one or multiple files from one Windows machine to another. 

* In **Linux**, it is usually as simple as spinning up a Python HTTP server, using SSH/SCP, FTP, or `impacket-smbserver`.
* On **Windows**, however, you won't always have a built-in HTTP server, SMB outbound/inbound traffic might be disabled or restricted by Group Policy (GPO), and finding a quick, reliable native solution can be frustrating. 
* While **Netcat** (`nc.exe`) is an option, it can sometimes mangle bytes, corrupt file integrity, or struggle with multiple files.

---

## The Solution

**Spark** solves this by providing a reliable, lightweight client-server mechanism specifically designed for fast and safe file transfers.

### 1. On the Server Side (Host)

Bind Spark to an IP address and port to serve files. By default, it will serve the directory from which it is executed:

```cmd
spark.exe -b 192.168.1.136 8080
```

#### Optional Flags:
* Use `-v` to enable **verbose mode**, which displays the list of served files.
* You can optionally specify a custom directory to serve as a positional argument at the end:

```cmd
spark.exe -b 192.168.1.136 8080 C:\Users\user\Desktop -v
```

---

### 2. On the Client Side (Receiver)

You can request all available files hosted by the server using the `-l` flag:

```cmd
spark.exe -l 192.168.1.136 8080
```

**Output Example:**
```text
1: file1.txt
2: anotherfile.iso
3: music.mp3
4: credentials.txt
```

#### Downloading Files:
To download a file, use the `-d` flag followed by the file index number:

```cmd
# Download a single file (index 6)
spark.exe -d 192.168.1.136 8080 6

# Download multiple files at once (comma-separated list)
spark.exe -d 192.168.1.136 8080 2,3,4
```

#### Optional Flags:
* Specify a custom output directory to save the downloaded files using the `-o` flag:

```cmd
spark.exe -d 192.168.1.136 8080 6 -o C:\Users\user\MyFolder
```

During the download, you will see a clean, real-time progress bar:

```text
File 6 (abc.txt): 16.70 GiB / 23.50 GiB [================================================================>---------------------------] 71 %
```

---

## How to Build

If you want to build the executable yourself:

1. Download or clone this repository.
2. Ensure you have the Go programming language installed on your system (download it from the [Official Go website](https://go.dev/doc/install)).
3. Open your terminal (Git Bash, Command Prompt, or PowerShell), navigate to the project directory, and execute:

```bash
go build
```

Once the compilation finishes, you will find your own `spark.exe` binary ready to use in the same folder.
