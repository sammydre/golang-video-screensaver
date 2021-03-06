package main

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"io/fs"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"github.com/sammydre/golang-video-screensaver/common"
	vlc "github.com/sammydre/golang-video-screensaver/vlcwrap"
)

var InstallPath string
var MediaPath string

type VideoWindowContext struct {
	mainWindow  *walk.MainWindow
	videoWidget *VlcVideoWidget
	MediaPath   string
	Bounds      declarative.Rectangle
	Identifier  string
	Parent      win.HWND
}

func (vmw *VideoWindowContext) getMedia() string {
	var files []fs.FileInfo
	files, err := ioutil.ReadDir(vmw.MediaPath)
	if err != nil {
		log.Panic(err)
	}

	var index = rand.Intn(len(files))
	var ret = filepath.Join(vmw.MediaPath, files[index].Name())

	log.Printf("%s: playing file %s", vmw.Identifier, ret)

	return ret
}

func (vmw *VideoWindowContext) Init() {
	// https://doxygen.reactos.org/d6/dc8/sdk_2lib_2scrnsave_2scrnsave_8c_source.html
	// see above for behaviour we need

	var videoWidget *VlcVideoWidget
	var err error

	if vmw.Parent == win.HWND(0) {
		declarative.MainWindow{
			AssignTo: &vmw.mainWindow,
			Title:    "Video main window",
			Layout: declarative.VBox{
				MarginsZero: true,
				SpacingZero: true,
			},
			Bounds: vmw.Bounds,
			Background: declarative.SolidColorBrush{
				Color: walk.RGB(0, 0, 0),
			},
		}.Create()

		videoWidget, err = NewVlcVideoWidget(
			vmw.mainWindow,
			func() {
				vmw.mainWindow.Close()
			},
			func() string {
				return vmw.getMedia()
			},
			vmw.mainWindow.Synchronize)
		if err != nil {
			log.Panic(err)
		}
	} else {
		videoWidget, err = NewPreviewVlcVideoWidget(
			vmw.Parent,
			vmw.getMedia,
			func(func()) {})
		if err != nil {
			log.Panic(err)
		}
	}

	vmw.videoWidget = videoWidget

	if vmw.mainWindow != nil {
		vmw.mainWindow.SetFullscreen(true)
	}

	vmw.videoWidget.SetupVlcPlayer()
}

func (vmw *VideoWindowContext) Deinit() {
	vmw.videoWidget.Deinit()
}

type Monitor struct {
	Rect win.RECT
	Name string
}

func listMonitors() []Monitor {
	var ret []Monitor
	var libuser32 *windows.LazyDLL
	var enumDisplayMonitors *windows.LazyProc

	// Unfortunately the "win" module doesn't wrap EnumDisplayMonitors for us,
	// so we have to do it ourselves instead.
	libuser32 = windows.NewLazySystemDLL("user32.dll")
	enumDisplayMonitors = libuser32.NewProc("EnumDisplayMonitors")

	EnumDisplayMonitors := func(hDc win.HDC, rect *win.RECT, lpfnEnum func(hmon win.HMONITOR, hDc win.HDC, rect *win.RECT, lParam uintptr) uintptr, dwData uintptr) bool {
		ret, _, _ := syscall.Syscall6(enumDisplayMonitors.Addr(), 4,
			uintptr(hDc),
			uintptr(unsafe.Pointer(rect)),
			syscall.NewCallback(lpfnEnum),
			dwData,
			0,
			0)
		return ret != 0
	}

	enumCallback := func(hmon win.HMONITOR, hDc win.HDC, rect *win.RECT, lParam uintptr) uintptr {
		// win.MONITORINFO exists, but it lacks the SzDevice at the end, which is
		// useful for debugging purposes.
		type MONITORINFOEX struct {
			CbSize    uint32
			RcMonitor win.RECT
			RcWork    win.RECT
			DwFlags   uint32
			SzDevice  [win.CCHDEVICENAME]uint16
		}

		var monitorInfo MONITORINFOEX
		monitorInfo.CbSize = uint32(unsafe.Sizeof(monitorInfo))

		if !win.GetMonitorInfo(hmon, (*win.MONITORINFO)(unsafe.Pointer(&monitorInfo))) {
			log.Panicf("GetMonitorInfo: %d", win.GetLastError())
		}

		var monitor = Monitor{
			Rect: monitorInfo.RcWork,
			Name: win.UTF16PtrToString(&monitorInfo.SzDevice[0])}
		ret = append(ret, monitor)

		log.Printf("Found monitor %d: %v", len(ret), monitor)

		// Must return true to keep iterating
		return win.TRUE
	}

	err := EnumDisplayMonitors(0, nil, enumCallback, 0)
	if !err {
		log.Panic("EnumDisplayMonitors")
	}

	return ret
}

func initRand() {
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package with cryptographically secure random number generator")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

func showConfigureWindow() {
	var mw *walk.MainWindow
	var mediaPathTextEdit *walk.TextEdit

	win.CoInitializeEx(nil, win.COINIT_APARTMENTTHREADED)

	declarative.MainWindow{
		AssignTo: &mw,
		Title:    "Configure Video Screensaver",
		MinSize:  declarative.Size{Width: 300, Height: 150},
		Size:     declarative.Size{Width: 400, Height: 150},
		Layout:   declarative.VBox{},
		// Font:     Font{Family: "Arial"},
		Children: []declarative.Widget{
			declarative.Label{
				Text: "Use videos from:",
			},
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true},
				Children: []declarative.Widget{
					declarative.HSpacer{},
					declarative.Label{
						Text: MediaPath,
					},
					declarative.HSpacer{},
					declarative.PushButton{
						Text: "Browse",
						OnClicked: func() {
							log.Print("Button clicked")
							dlg := new(walk.FileDialog)

							dlg.Title = "Select a media path"
							if ok, err := dlg.ShowBrowseFolder(mw); err != nil {
								log.Fatalf("err is %v", err)
								return
							} else if !ok {
								log.Print("not ok - user cancelled")
								return
							}

							log.Printf("User selected media path %v", dlg.FilePath)

							setMediaPath(dlg.FilePath)
							mediaPathTextEdit.SetText(dlg.FilePath)
						},
					},
				},
			},
			declarative.VSpacer{},
			declarative.VSeparator{},
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true},
				Children: []declarative.Widget{
					declarative.HSpacer{},
					declarative.PushButton{
						Text: "Ok",
						OnClicked: func() {
							mw.Close()
						},
					},
				},
			},
		},
	}.Run()
}

func runScreenSaver(parent win.HWND) {
	win.CoInitializeEx(nil, win.COINIT_MULTITHREADED)

	monitorRects := listMonitors()

	newpath := os.Getenv("PATH") + ";" + InstallPath
	log.Print("Setting PATH to: ", newpath)
	os.Setenv("PATH", newpath)

	err := vlc.Init("--no-audio") // , "--verbose=2"
	if err != nil {
		log.Panic(err)
	}

	// log.Print(vlc.AudioOutputList())

	var windows []*VideoWindowContext

	if parent == win.HWND(0) {
		for _, mon := range monitorRects {
			rect := mon.Rect
			var videoWindow *VideoWindowContext = &VideoWindowContext{
				MediaPath: MediaPath,
				Bounds: declarative.Rectangle{
					X:      int(rect.Left),
					Y:      int(rect.Top),
					Width:  int(rect.Right - rect.Left),
					Height: int(rect.Bottom - rect.Top),
				},
				Identifier: mon.Name,
			}
			videoWindow.Init()

			windows = append(windows, videoWindow)
		}
	} else {
		// rect := mon.Rect
		var videoWindow *VideoWindowContext = &VideoWindowContext{
			MediaPath:  MediaPath,
			Identifier: "Preview",
			Parent:     parent,
		}
		videoWindow.Init()

		windows = append(windows, videoWindow)
	}

	if windows[0].mainWindow != nil {
		windows[0].mainWindow.Run()
	} else {
		msg := (*win.MSG)(unsafe.Pointer(win.GlobalAlloc(0, unsafe.Sizeof(win.MSG{}))))
		defer win.GlobalFree(win.HGLOBAL(unsafe.Pointer(msg)))

		var keepGoing = true

		for keepGoing {
			switch win.GetMessage(msg, 0, 0, 0) {
			case 0:
				keepGoing = false
				continue
				// break // int(msg.WParam)

			case -1:
				keepGoing = false
				continue
				// break // return -1
			}

			win.TranslateMessage(msg)
			win.DispatchMessage(msg)
		}
	}

	for _, vmw := range windows {
		vmw.Deinit()
	}

	vlc.Release()
}

type CommandType int

const (
	InvalidCommand CommandType = iota
	RunScreenSaver
	PreviewScreenSaver
	ConfigureScreenSaver
)

type Command struct {
	ctype CommandType
	hwnd  win.HWND
}

func parseCommandLineArgs(args []string) Command {
	// https://docs.microsoft.com/en-us/troubleshoot/windows/win32/screen-saver-command-line

	// The ReactOS sources suggest the configure case can additionally have an
	// argument. But the MS documentation does not mention that. So I've not
	// implemented that here.

	var ignore int = -1
	var command = Command{ctype: ConfigureScreenSaver}

	for index, word := range args {
		if index <= ignore {
			continue
		}

		switch word {
		case "-a", "/a", "/A":
			ignore = index + 1
			command.ctype = InvalidCommand
		case "-s", "/s", "/S":
			command.ctype = RunScreenSaver
		case "-p", "/p", "/P":
			command.ctype = PreviewScreenSaver
		case "-c", "/c", "/C":
			command.ctype = ConfigureScreenSaver
		default:
			switch command.ctype {
			case PreviewScreenSaver:
				parsedInt, err := strconv.ParseInt(word, 0, 64)
				if err != nil {
					return Command{ctype: InvalidCommand}
				}
				command.hwnd = win.HWND(parsedInt)
			}
		}
	}

	return command
}

func init() {
	walk.AppendToWalkInit(func() {
		walk.MustRegisterWindowClass(VlcVideoWidgetWindowClass)
	})
}

func loadRegistryEntries() {
	var err error

	InstallPath, err = common.RegistryLoadString("Software\\sammydre\\golang-video-screensaver", "InstallPath")
	if err != nil {
		log.Printf("No install path, using the current working directory (error was: %v)", err)
		InstallPath, _ = os.Getwd()
	}
	log.Printf("Using install path of %v", InstallPath)

	MediaPath, err = common.RegistryLoadString("Software\\sammydre\\golang-video-screensaver", "MediaPath")
	if err != nil {
		log.Printf("No media path, using the current working directory (error was: %v)", err)
		MediaPath, _ = os.Getwd()
	}
	log.Printf("Using media path of %v", MediaPath)
}

func setMediaPath(path string) {
	common.RegistrySaveString(
		"Software\\sammydre\\golang-video-screensaver",
		"MediaPath",
		path)
	MediaPath = path
}

func setupLogging() {
	f, err := os.OpenFile(InstallPath+"\\log.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Panicf("Error opening log file %v", err)
	}

	log.SetOutput(f)

	cwd, _ := os.Getwd()

	log.Printf("Logging to file initialised. InstallPath %v MediaPath %v Cwd %v Args %v",
		InstallPath, MediaPath, cwd, os.Args)
}

func main() {
	initRand()
	loadRegistryEntries()
	setupLogging()

	cmd := parseCommandLineArgs(os.Args[1:])

	switch cmd.ctype {
	case RunScreenSaver:
		runScreenSaver(win.HWND(0))
	case PreviewScreenSaver:
		runScreenSaver(cmd.hwnd)
	case ConfigureScreenSaver:
		showConfigureWindow()
	}
}
