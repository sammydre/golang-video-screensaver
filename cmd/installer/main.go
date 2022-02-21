package main

import (
	"embed"
	"log"
	"path"

	screensaver "github.com/sammydre/golang-video-screensaver"
)

func recursePrint(fs embed.FS, dirPath string) {
	dirEntries, err := fs.ReadDir(path.Clean(dirPath))
	if err != nil {
		log.Panicf("Failed to list libVlc: %v", err)
	}

	for _, dirEntry := range dirEntries {
		filePath := path.Clean(dirPath + "/" + dirEntry.Name())

		if dirEntry.IsDir() {
			log.Print("dir  ", filePath)
			recursePrint(fs, filePath)
		} else {
			log.Print("file ", filePath)
		}
	}
}

func main() {
	log.Print("Hello world ", len(screensaver.VideoGalleryExe))

	recursePrint(screensaver.LibVlc, ".")
}
