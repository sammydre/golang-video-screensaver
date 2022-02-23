package main

import (
	"embed"
	"log"
	"os"
	"path"
	"strings"

	screensaver "github.com/sammydre/golang-video-screensaver"
	"github.com/sammydre/golang-video-screensaver/common"
)

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

type fsInstallDescripion struct {
	fs       embed.FS
	trimPath string
	addPath  string
}

func (fid *fsInstallDescripion) install(destPath string) error {
	destPath = path.Join(destPath, fid.addPath)
	return recursiveFsInstaller(fid.fs, fid.trimPath, ".", destPath)
}

type fileInstallDescription struct {
	data    []byte
	name    string
	addPath string
}

func (fid *fileInstallDescription) install(destPath string) error {
	destPath = path.Join(destPath, fid.addPath)

	err := os.MkdirAll(destPath, 0)
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(destPath, fid.name), fid.data, 0)
	if err != nil {
		return err
	}

	return nil
}

type registryInstallDescription struct {
	subKeyPath string
	valueName  string
	value      string
}

func (rid *registryInstallDescription) install(destPath string) error {
	return common.RegistrySaveString(rid.subKeyPath, rid.valueName, rid.value)
}

type installInstance interface {
	install(string) error
}

type installDescription struct {
	instances []installInstance
}

func install(desc *installDescription, installPath string) error {
	for _, inst := range desc.instances {
		err := inst.install(installPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	var installDesc = installDescription{
		instances: []installInstance{
			&fsInstallDescripion{
				fs:       screensaver.LibVlc,
				trimPath: "out/libvlc-3.0.16/build/x64",
				addPath:  "",
			},
			&fileInstallDescription{
				data:    screensaver.VideoGalleryExe,
				name:    "VideoGallery.scr",
				addPath: "",
			},
			&registryInstallDescription{
				subKeyPath: "Software\\sammydre\\golang-video-screensaver",
				valueName:  "InstallPath",
				value:      "${InstallPath}",
			},
		},
	}

	install(&installDesc, "C:\\Users\\sam\\AppData\\Local\\golang-video-screensaver")
}
