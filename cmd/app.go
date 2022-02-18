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
	mainWindow        *walk.MainWindow
	videoPlayer       *vlc.Player
	endReachedEventId vlc.EventID
	MediaPath         string
	Bounds            Rectangle
	Identifier        string
}

func (vmw *VideoWindowContext) getMedia() string {
	var files []fs.FileInfo
	files, err := ioutil.ReadDir(vmw.MediaPath)
	if err != nil {
		log.Panic(err)
	}

	var index = rand.Intn(len(files))

	return filepath.Join(vmw.MediaPath, files[index].Name())
}

type VlcVideoWidget struct {
	walk.WidgetBase
	videoWindowContext *VideoWindowContext
	cursorPos          win.POINT
}

const VlcVideoWidgetWindowClass = "VLC Video Widget Class"

func NewVlcVideoWidget(parent walk.Container, vwc *VideoWindowContext) (*VlcVideoWidget, error) {
	w := new(VlcVideoWidget)
	w.videoWindowContext = vwc

	if err := walk.InitWidget(
		w,
		parent,
		VlcVideoWidgetWindowClass,
		win.WS_VISIBLE,
		0); err != nil {
		return nil, err
	}

	bg, err := walk.NewSolidColorBrush(walk.RGB(0, 0, 0))
	if err != nil {
		return nil, err
	}
	w.SetBackground(bg)

	if !win.GetCursorPos(&w.cursorPos) {
		log.Panic("GetCursorPos failed")
	}
	win.SetCursor(0)

	return w, nil
}

func (*VlcVideoWidget) CreateLayoutItem(ctx *walk.LayoutContext) walk.LayoutItem {
	return &vlcVideoWidgetLayoutItem{idealSize: walk.SizeFrom96DPI(walk.Size{150, 150}, ctx.DPI())}
}

type vlcVideoWidgetLayoutItem struct {
	walk.LayoutItemBase
	idealSize walk.Size // in native pixels
}

func (li *vlcVideoWidgetLayoutItem) LayoutFlags() walk.LayoutFlags {
	return walk.ShrinkableHorz | walk.ShrinkableVert | walk.GrowableHorz | walk.GrowableVert | walk.GreedyHorz | walk.GreedyVert
}

func (li *vlcVideoWidgetLayoutItem) IdealSize() walk.Size {
	return li.idealSize
}

func (w *VlcVideoWidget) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_NCACTIVATE, win.WM_ACTIVATE, win.WM_ACTIVATEAPP:
		if wParam == 0 {
			w.videoWindowContext.mainWindow.Close()
		}
	case win.WM_LBUTTONDOWN, win.WM_RBUTTONDOWN, win.WM_MBUTTONDOWN, win.WM_XBUTTONDOWN, win.WM_KEYDOWN, win.WM_KEYUP, win.WM_SYSKEYDOWN:
		w.videoWindowContext.mainWindow.Close()
	case win.WM_MOUSEMOVE:
		var point = win.POINT{int32(win.GET_X_LPARAM(lParam)), int32(win.GET_Y_LPARAM(lParam))}
		if point.X != w.cursorPos.X || point.Y != w.cursorPos.Y {
			w.videoWindowContext.mainWindow.Close()
		}
	case win.WM_SETCURSOR:
		return 0
	}

	return w.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}

func (vmw *VideoWindowContext) Init() {
	// https://doxygen.reactos.org/d6/dc8/sdk_2lib_2scrnsave_2scrnsave_8c_source.html
	// see above for behaviour we need

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

	videoWidget, err := NewVlcVideoWidget(vmw.mainWindow, vmw)
	if err != nil {
		log.Panic(err)
	}

	vmw.videoPlayer, err = vlc.NewPlayer()
	if err != nil {
		log.Panic(err)
	}

	err = vmw.videoPlayer.SetHWND(uintptr(videoWidget.AsWindowBase().Handle()))
	if err != nil {
		log.Panic(err)
	}

	err = vmw.videoPlayer.SetKeyInput(false)
	if err != nil {
		log.Panic(err)
	}

	err = vmw.videoPlayer.SetMouseInput(false)
	if err != nil {
		log.Panic(err)
	}

	err = vmw.videoPlayer.SetAudioOutput("adummy")
	if err != nil {
		log.Print(err)
	}

	mediaFileName := vmw.getMedia()

	log.Printf("%s: playing file %s", vmw.Identifier, mediaFileName)

	_, err = vmw.videoPlayer.LoadMediaFromPath(mediaFileName)
	if err != nil {
		log.Panic(err)
	}

	err = vmw.videoPlayer.SetMute(true)
	if err != nil {
		log.Panic(err)
	}

	manager, err := vmw.videoPlayer.EventManager()
	if err != nil {
		log.Panic(err)
	}

	endReachedCallback := func(event vlc.Event, userData interface{}) {
		// This callback is called from a somewhat uncertain context. I don't think
		// we can safely call vlc functions in this state? (Maybe its not re-entrant?)
		vmw.mainWindow.Synchronize(func() {
			mediaFileName := vmw.getMedia()
			log.Printf("%s: playing file %s", vmw.Identifier, mediaFileName)
			vmw.videoPlayer.LoadMediaFromPath(mediaFileName)
			vmw.videoPlayer.Play()
		})
	}

	vmw.endReachedEventId, err = manager.Attach(vlc.MediaPlayerEndReached, endReachedCallback, nil)
	if err != nil {
		log.Panic(err)
	}

	vmw.videoPlayer.Play()
	vmw.mainWindow.SetFullscreen(true)
}

func (vmw *VideoWindowContext) Deinit() {
	if vmw.videoPlayer != nil {
		manager, err := vmw.videoPlayer.EventManager()
		if err != nil {
			log.Panic(err)
		}

		manager.Detach(vmw.endReachedEventId)

		if media, _ := vmw.videoPlayer.Media(); media != nil {
			media.Release()
		}

		vmw.videoPlayer.Stop()
		vmw.videoPlayer.Release()
	}
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
		log.Fatal("TODO: run screensaver as a child of 'parent'")
	}

	windows[0].mainWindow.Run()

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
	var command = Command{ctype: InvalidCommand}

	for index, word := range args {
		if index <= ignore {
			continue
		}

		switch word {
		case "-a", "/a":
			ignore = index + 1
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

func main() {
	initRand()
	loadRegistryEntries()

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
