package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	screensaver "github.com/sammydre/golang-video-screensaver"
)

type installTracker interface {
	trackFile(string)
	trackDirectory(string)
	trackRegistryEntry(string)
}

type installConfiguration interface {
	install(*installTracker, string) error
}

type fileSystemInstallConfiguration struct {
	fs       embed.FS
	trimPath string
}

type singleFileInstallConfiguration struct {
	file []byte
}

func recursiveFsInstaller(fs embed.FS, trimPath string, dirPath string, destPath string) error {
	dirEntries, err := fs.ReadDir(path.Clean(dirPath))
	if err != nil {
		return err
	}

	for _, dirEntry := range dirEntries {
		filePath := path.Join(dirPath, dirEntry.Name())
		var fileDestPath string

		if len(trimPath) > 0 {
			destPathToAppend := path.Clean(filePath)
			if strings.HasPrefix(destPathToAppend, trimPath) {
				destPathToAppend = string(([]rune(destPathToAppend))[len(trimPath):])
			} else if len(destPathToAppend) < len(trimPath) && destPathToAppend == trimPath[:len(destPathToAppend)] {
				destPathToAppend = ""
			}

			if len(destPathToAppend) == 0 {
				log.Printf("Skipping %v due to match again trim %v", filePath, trimPath)

				if dirEntry.IsDir() {
					err = recursiveFsInstaller(fs, trimPath, filePath, destPath)
					if err != nil {
						return err
					}
				}
				continue
			}

			fileDestPath = path.Join(destPath, destPathToAppend)
		} else {
			fileDestPath = path.Join(destPath, filePath)
		}

		log.Print(filePath, " -> ", fileDestPath)

		if dirEntry.IsDir() {
			err := os.Mkdir(fileDestPath, 0)
			if err != nil {
				return err
			}
			err = recursiveFsInstaller(fs, trimPath, filePath, destPath)
			if err != nil {
				return err
			}
			// TODO: track
		} else {
			data, err := fs.ReadFile(filePath)
			if err != nil {
				// Unexpected, this should just work
				return err
			}

			log.Printf("Writing source file %v -> %v of size %v", filePath, fileDestPath, len(data))

			// The last argument here is a fs.FileMode, which we could obtain from dirEntry.Info().
			// But it shouldn't be relevant to us here, especially on Windows.
			err = os.WriteFile(fileDestPath, data, 0)
			if err != nil {
				// TODO: fmt.Errorf() for a better desc
				return err
			}
		}
	}

	return nil
}

func (cfg *fileSystemInstallConfiguration) install(tracker *installTracker, destPath string) error {
	err := recursiveFsInstaller(cfg.fs, cfg.trimPath, ".", destPath)
	if err != nil {
		return fmt.Errorf("installing files to %v: %w", destPath, err)
	}

	return nil
}

func (cfg *singleFileInstallConfiguration) install(tracker *installTracker, destPath string) error {
	return nil
}

func main() {
	// log.Print("Hello world ", len(screensaver.VideoGalleryExe))

	var libVlcInstallConfig = fileSystemInstallConfiguration{
		fs:       screensaver.LibVlc,
		trimPath: "out/libvlc-3.0.16/build/x64",
	}

	err := libVlcInstallConfig.install(nil, "C:\\Users\\sam\\AppData\\Local\\Temp\\SamSam")
	if err != nil {
		log.Print(err)
	}
}
