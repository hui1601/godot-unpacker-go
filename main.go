package main

import (
	"crypto/md5"
	"fmt"
	"os"
	"strings"
)

const MAGIC = "GDPC"

const (
	PackDirEncrypted = 1 << iota
	PackRelFilebase  = 1 << iota
)

const PackFileEncrypted = 1 << iota

const (
	DataFormatImage = iota
	DataFormatPng
	DataFormatWebp
	DataFormatBasisUniversal
)

type File struct {
	path      string
	ofs       uint64
	size      uint64
	md5       []byte
	encrypted bool
}

func bytesToUint32(bytes []byte) uint32 {
	return uint32(bytes[0]) | uint32(bytes[1])<<8 | uint32(bytes[2])<<16 | uint32(bytes[3])<<24
}

func readUint32(file *os.File) uint32 {
	byteArr := make([]byte, 4)
	_, err := file.Read(byteArr)
	if err != nil {
		panic(err)
	}
	return bytesToUint32(byteArr)
}

func readUint64(file *os.File) uint64 {
	byteArr := make([]byte, 8)
	_, err := file.Read(byteArr)
	if err != nil {
		panic(err)
	}
	return uint64(byteArr[0]) | uint64(byteArr[1])<<8 | uint64(byteArr[2])<<16 | uint64(byteArr[3])<<24 | uint64(byteArr[4])<<32 | uint64(byteArr[5])<<40 | uint64(byteArr[6])<<48 | uint64(byteArr[7])<<56
}

func getPad(alignment int, n int) int64 {
	rest := n % alignment
	pad := 0
	if rest > 0 {
		pad = alignment - rest
	}
	return int64(pad)
}

func getMD5Hash(data []byte) []byte {
	hash := md5.New()
	hash.Write(data)
	return hash.Sum(nil)
}

func unpackImageResource(path string, data []byte) {
	if data[0] == 'G' && data[1] == 'S' && data[2] == 'T' && data[3] == '2' {
		// Unpack GST2
		fmt.Println("Unpacking GST2")
		version := int(data[4]) | int(data[5])<<8 | int(data[6])<<16 | int(data[7])<<24
		fmt.Println("Version:", version)
		if version != 1 {
			panic("Unsupported GST2 version")
		}
		width := int(data[8]) | int(data[9])<<8 | int(data[10])<<16 | int(data[11])<<24
		height := int(data[12]) | int(data[13])<<8 | int(data[14])<<16 | int(data[15])<<24
		fmt.Println("Width:", width)
		fmt.Println("Height:", height)
		format := int(data[36]) | int(data[37])<<8 | int(data[38])<<16 | int(data[39])<<24
		if format == DataFormatImage {
			fmt.Println("DATA_FORMAT_IMAGE")
		} else if format == DataFormatPng {
			fmt.Println("DATA_FORMAT_PNG")
		} else if format == DataFormatWebp {
			fmt.Println("DATA_FORMAT_WEBP")
		} else if format == DataFormatBasisUniversal {
			fmt.Println("DATA_FORMAT_BASIS_UNIVERSAL")
		}
		mipmaps := int(data[36]) | int(data[37])<<8 | int(data[38])<<16 | int(data[39])<<24
		fmt.Println("Mipmaps:", mipmaps)
		base := 0x34
		for i := 0; i < mipmaps; i++ {
			if base+4 > len(data) {
				break
			}
			size := int(data[base]) | int(data[base+1])<<8 | int(data[base+2])<<16 | int(data[base+3])<<24
			fmt.Printf("Mipmap %d size: %d\n", i, size)
			// write to file
			err := os.WriteFile(fmt.Sprintf("%s_%d", path, i), data[base+4:base+4+size], 0644)
			if err != nil {
				panic(err)
			}
			base += 4 + size
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: pck-info <pck file>")
		os.Exit(1)
	}
	pckFilePath := os.Args[1]
	file, err := os.Open(pckFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Read the file header
	headerByte := make([]byte, 4)
	_, err = file.Read(headerByte)
	if err != nil {
		panic(err)
	}
	if string(headerByte) != MAGIC {
		panic("Invalid file header")
	}

	// Read the file version
	formatVersion := readUint32(file)
	fmt.Println("PCK file version:", formatVersion)

	if formatVersion != 2 {
		panic("Unsupported PCK file version")
	}

	// Read the engine version
	engineMajorVersion := readUint32(file)
	engineMinorVersion := readUint32(file)
	enginePatchVersion := readUint32(file)

	fmt.Printf("Engine version: %d.%d.%d\n", engineMajorVersion, engineMinorVersion, enginePatchVersion)

	// Read the flags
	flags := readUint32(file)
	fmt.Print("Flags: ")
	if flags&PackDirEncrypted != 0 {
		fmt.Print("PACK_DIR_ENCRYPTED ")
	}
	if flags&PackRelFilebase != 0 {
		fmt.Print("PACK_REL_FILEBASE ")
	}
	if flags == 0 {
		fmt.Print("None")
	} else {
		fmt.Println()
		panic("Unsupported flags")
	}
	fmt.Println()

	filesBase := readUint64(file)
	for i := 0; i < 16; i++ {
		_ = readUint32(file) // Skip reserved
	}
	// Read the number of files
	numFiles := readUint32(file)
	fmt.Println("Number of files:", numFiles)
	fmt.Println()

	files := make([]File, numFiles)
	// Read the files
	for i := 0; i < int(numFiles); i++ {
		// Read the file path length
		pathLen := readUint32(file)
		pathByte := make([]byte, pathLen)
		_, err = file.Read(pathByte)
		if err != nil {
			panic(err)
		}
		path := string(pathByte)
		path = strings.Trim(path, "\x00")
		fmt.Println("File path:", path)

		// Skip the padding
		padding := 4 - (pathLen % 4)
		if padding != 4 {
			_, err = file.Seek(int64(padding), 1)
			if err != nil {
				panic(err)
			}
		}

		// Read the file offset
		fileOffset := readUint64(file)
		fmt.Println("File offset:", fileOffset)

		// Read the file size
		fileSize := readUint64(file)
		fmt.Println("File size:", fileSize)

		// Read the MD5 hash
		md5Byte := make([]byte, 16)
		_, err = file.Read(md5Byte)
		if err != nil {
			panic(err)
		}
		fmt.Printf("MD5 hash: %x\n", md5Byte)

		// Read the flags
		fileFlags := readUint32(file)
		fmt.Print("File flags: ")
		if fileFlags&PackFileEncrypted != 0 {
			fmt.Print("PACK_FILE_ENCRYPTED ")
		}
		if fileFlags == 0 {
			fmt.Print("None")
		} else {
			fmt.Println()
			panic("Unsupported file flags")
		}
		fmt.Println()
		fmt.Println()

		files[i] = File{
			path:      path,
			ofs:       fileOffset,
			size:      fileSize,
			md5:       md5Byte,
			encrypted: fileFlags&PackFileEncrypted != 0,
		}
	}
	// seek file base
	fmt.Printf("Files base: %x\n", filesBase)
	// Read the file data
	for i, f := range files {
		fmt.Printf("[%d/%d] File path: %s\n", i+1, numFiles, f.path)
		_, err = file.Seek(int64(f.ofs+filesBase), 0)
		if err != nil {
			panic(err)
		}
		data := make([]byte, f.size)
		_, err = file.Read(data)
		if err != nil {
			panic(err)
		}
		// remove res://
		if f.path[:6] == "res://" {
			f.path = f.path[6:]
		}
		f.path = "export/" + f.path
		// write to file
		if strings.Contains(f.path, "/") {
			err = os.MkdirAll(f.path[:strings.LastIndex(f.path, "/")], 0755)
		}
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(f.path, data, 0644)
		if err != nil {
			panic(err)
		}
		// check md5
		if fmt.Sprintf("%x", getMD5Hash(data)) != fmt.Sprintf("%x", f.md5) {
			fmt.Printf("Warning: MD5 hash mismatch for file %s\n", f.path)
		}
		if strings.HasSuffix(f.path, ".ctex") {
			unpackImageResource(f.path, data)
		}
	}
}
