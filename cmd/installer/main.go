package main

import (
	"embed"
	"log"
	"os"
	"path"
	"strings"
	"sync/atomic"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	screensaver "github.com/sammydre/golang-video-screensaver"
	"github.com/sammydre/golang-video-screensaver/common"
)

func recursiveFsInstaller(fs embed.FS, trimPath string, dirPath string, destPath string, progress func()) error {
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
					err = recursiveFsInstaller(fs, trimPath, filePath, destPath, progress)
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
			err := os.MkdirAll(fileDestPath, 0)
			if err != nil {
				return err
			}

			err = recursiveFsInstaller(fs, trimPath, filePath, destPath, progress)
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
				log.Printf("Error writing file, %v", err)
				// TODO: fmt.Errorf() for a better desc
				return err
			}

			progress()
		}
	}

	return nil
}

func recursiveFsCount(fs embed.FS, dirPath string) int {
	dirEntries, err := fs.ReadDir(path.Clean(dirPath))
	if err != nil {
		return 0
	}

	var ret = 0

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			ret += recursiveFsCount(fs, path.Join(dirPath, dirEntry.Name()))
		} else {
			ret += 1
		}
	}

	return ret
}

type fsInstallDescripion struct {
	fs       embed.FS
	trimPath string
	addPath  string
}

func (fid *fsInstallDescripion) install(destPath string, progress func()) error {
	destPath = path.Join(destPath, fid.addPath)
	return recursiveFsInstaller(fid.fs, fid.trimPath, ".", destPath, progress)
}

func (fid *fsInstallDescripion) count() int {
	return recursiveFsCount(fid.fs, ".")
}

type fileInstallDescription struct {
	data    []byte
	name    string
	addPath string
}

func (fid *fileInstallDescription) install(destPath string, progress func()) error {
	destPath = path.Join(destPath, fid.addPath)

	err := os.MkdirAll(destPath, 0)
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(destPath, fid.name), fid.data, 0)
	if err != nil {
		return err
	}

	progress()

	return nil
}

func (rid *fileInstallDescription) count() int {
	return 1
}

type registryInstallDescription struct {
	subKeyPath string
	valueName  string
	value      string
}

func (rid *registryInstallDescription) install(destPath string, progress func()) error {
	value := strings.Replace(rid.value, "${InstallPath}", destPath, -1)

	ret := common.RegistrySaveString(rid.subKeyPath, rid.valueName, value)
	progress()
	return ret
}

func (rid *registryInstallDescription) count() int {
	return 1
}

type installInstance interface {
	install(string, func()) error
	count() int
}

type installDescription struct {
	instances []installInstance
}

func (desc *installDescription) install(installPath string, progress func()) error {
	for _, inst := range desc.instances {
		err := inst.install(installPath, progress)
		if err != nil {
			return err
		}
	}

	return nil
}

func (desc *installDescription) count() int {
	var ret = 0
	for _, inst := range desc.instances {
		ret += inst.count()
	}
	return ret
}

func installerGui(installDir string, desc installInstance) {
	var mw *walk.MainWindow
	var progress *walk.ProgressBar
	var topLabel *walk.Label

	numItemsToInstall := desc.count()

	win.CoInitializeEx(nil, win.COINIT_APARTMENTTHREADED)

	err := declarative.MainWindow{
		AssignTo: &mw,
		Title:    "Video Screensaver Installer",
		MinSize:  declarative.Size{Width: 300, Height: 150},
		Size:     declarative.Size{Width: 400, Height: 150},
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			declarative.Label{
				AssignTo: &topLabel,
				Text:     "Ready to install",
			},
			declarative.ProgressBar{
				MinValue: 0,
				MaxValue: numItemsToInstall,
				Value:    0,
				AssignTo: &progress,
			},
			declarative.VSpacer{},
			declarative.VSeparator{},
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true},
				Children: []declarative.Widget{
					declarative.HSpacer{},
					declarative.PushButton{
						Text: "Install",
						OnClicked: func() {
							topLabel.SetText("Installing...")

							var progressCount uint64 = 0
							go desc.install(installDir, func() {
								mw.Synchronize(func() {
									atomic.AddUint64(&progressCount, 1)
									progress.SetValue(int(atomic.LoadUint64(&progressCount)))
									log.Printf("progress now %d", progressCount)
								})
							})
						},
					},
				},
			},
		},
	}.Create()

	if err != nil {
		log.Panicf("Failed to create windows: %v", err)
	}

	mw.Run()
}

func main() {
	installDir, err := walk.LocalAppDataPath()
	if err != nil {
		log.Panicf("Failed to find local app data path")
	}

	installDir = path.Join(installDir, "sammydre", "golang-video-screensaver")

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

	installerGui(installDir, &installDesc)
}
