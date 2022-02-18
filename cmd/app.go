package main

import "C"

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
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
	"golang.org/x/sys/windows/registry"

	// vlc "github.com/adrg/libvlc-go/v3"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	vlc "github.com/sammydre/golang-video-screensaver/vlcwrap"
)

var InstallPath string
var MediaPath string

type VideoWindowContext struct {
	mainWindow  *walk.MainWindow
	videoWidget *VlcVideoWidget
	MediaPath   string
	Bounds      Rectangle
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
		MainWindow{
			AssignTo: &vmw.mainWindow,
			Title:    "Video main window",
			Layout: VBox{
				MarginsZero: true,
				SpacingZero: true,
			},
			Bounds: vmw.Bounds,
			Background: SolidColorBrush{
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

	MainWindow{
		AssignTo: &mw,
		Title:    "Configure Video Screensaver",
		MinSize:  Size{300, 150},
		Size:     Size{400, 150},
		Layout:   VBox{},
		// Font:     Font{Family: "Arial"},
		Children: []Widget{
			Label{
				Text: "Use videos from:",
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					Label{
						Text: MediaPath,
					},
					HSpacer{},
					PushButton{
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
			VSpacer{},
			VSeparator{},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{
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

	newpath := os.Getenv("PATH") + ";" + InstallPath + "\\libvlc-3.0.16\\build\\x64"
	log.Print("Setting PATH to: ", newpath)
	os.Setenv("PATH", newpath)

	err := vlc.Init(InstallPath+"\\libvlc-3.0.16\\build\\x64", "--no-audio") // , "--verbose=2"
	if err != nil {
		log.Printf("win error was ", win.GetLastError())
		log.Panic(err)
	}

	// log.Print(vlc.AudioOutputList())

	var windows []*VideoWindowContext

	if parent == win.HWND(0) {
		for _, mon := range monitorRects {
			rect := mon.Rect
			var videoWindow *VideoWindowContext = &VideoWindowContext{
				MediaPath: MediaPath,
				Bounds: Rectangle{
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

		for {
			switch win.GetMessage(msg, 0, 0, 0) {
			case 0:
				break // int(msg.WParam)

			case -1:
				break // return -1
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
	var command = Command{ctype: RunScreenSaver}

	for index, word := range args {
		if index <= ignore {
			continue
		}

		switch word {
		case "-a", "/a":
			ignore = index + 1
			command.ctype = InvalidCommand
		case "-s", "/s":
			command.ctype = RunScreenSaver
		case "-p", "/p":
			command.ctype = PreviewScreenSaver
		case "-c", "/c":
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

	InstallPath, err = registryLoadString("Software\\sammydre\\golang-video-screensaver", "InstallPath")
	if err != nil {
		log.Printf("No install path, using the current working directory (error was: %v)", err)
		InstallPath, _ = os.Getwd()
	}
	log.Printf("Using install path of %v", InstallPath)

	MediaPath, err = registryLoadString("Software\\sammydre\\golang-video-screensaver", "MediaPath")
	if err != nil {
		log.Printf("No media path, using the current working directory (error was: %v)", err)
		MediaPath, _ = os.Getwd()
	}
	log.Printf("Using media path of %v", MediaPath)
}

func registrySaveString(subKeyPath string, valueName string, value string) error {
	// walk doesn't provide registry set/save functions. Nor even create key. So use the windows
	// package for that.
	key, openedExisting, err := registry.CreateKey(registry.CURRENT_USER, subKeyPath, registry.ALL_ACCESS)
	if err != nil {
		log.Panicf("RegCreateKeyEx: %v", windows.GetLastError())
	}

	defer key.Close()

	if openedExisting {
	}

	err = key.SetStringValue(valueName, value)
	if err != nil {
		log.Panicf("RegSetValueEx: %v", windows.GetLastError())
	}

	return nil
}

func registryLoadString(subKeyPath string, valueName string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, subKeyPath, registry.READ)
	if err != nil {
		return "", fmt.Errorf("%v: OpenKey() failed: %w", subKeyPath, err)
	}
	defer key.Close()

	val, valType, err := key.GetStringValue(valueName)
	if err != nil {
		return "", fmt.Errorf("%v: GetStringValue(%v) failed: %w", subKeyPath, valueName, err)
	}

	if valType != registry.SZ {
		return "", fmt.Errorf("%v: GetStringValue(%v) returned invalid type %v", subKeyPath, valueName, valType)
	}

	return val, nil
}

func setMediaPath(path string) {
	registrySaveString(
		"Software\\sammydre\\golang-video-screensaver",
		"MediaPath",
		path)
	MediaPath = path
}

func setInstallPath(path string) {
	registrySaveString(
		"Software\\sammydre\\golang-video-screensaver",
		"InstallPath",
		path)
	InstallPath = path
}

func setupLogging() {
	f, err := os.OpenFile(InstallPath+"\\log.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Panicf("Error opening log file ", err)
	}

	log.SetOutput(f)

	cwd, _ := os.Getwd()

	log.Printf("Logging to file initialised. InstallPath %v MediaPath %v Cwd %v",
		InstallPath, MediaPath, cwd)
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
