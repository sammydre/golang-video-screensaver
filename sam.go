package main

import (
	"C"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"flag"
	"io/fs"
	"io/ioutil"
	"log"
	"math/rand"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	vlc "github.com/adrg/libvlc-go/v3"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

//export thisIsALongFunctionName
func thisIsALongFunctionName() {

}

// possible we'll need a C shim if we need to fix up calling convention?

//-export ScreenSaverProc
func ScreenSaverProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	return 0
}

//-export ScreenSaverConfigureDialog
func ScreenSaverConfigureDialog(hdlg win.HWND, msg uint32, wParam, lParam uintptr) {
}

type VideoMainWindow struct {
	mainWindow        *walk.MainWindow
	videoPlayer       *vlc.Player
	endReachedEventId vlc.EventID
	MediaPath         string
	Bounds            Rectangle
	Identifier        string
}

func (vmw *VideoMainWindow) getMedia() string {
	var files []fs.FileInfo
	files, err := ioutil.ReadDir(vmw.MediaPath)
	if err != nil {
		log.Panic(err)
	}

	var index = rand.Intn(len(files))

	return filepath.Join(vmw.MediaPath, files[index].Name())
}

func (vmw *VideoMainWindow) Init() {
	MainWindow{
		AssignTo: &vmw.mainWindow,
		Title:    "Video main window",
		Layout:   VBox{},
		Bounds:   vmw.Bounds,
		Background: SolidColorBrush{
			Color: walk.RGB(0, 0, 0),
		},
		OnMouseDown: func(x, y int, button walk.MouseButton) {
			vmw.mainWindow.SetFullscreen(!vmw.mainWindow.Fullscreen())
		},
	}.Create()

	var err error

	vmw.videoPlayer, err = vlc.NewPlayer()
	if err != nil {
		log.Panic(err)
	}

	err = vmw.videoPlayer.SetHWND(uintptr(vmw.mainWindow.AsWindowBase().Handle()))
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

func (vmw *VideoMainWindow) Deinit() {
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

func main() {
	initRand()

	win.CoInitializeEx(nil, win.COINIT_MULTITHREADED)

	mediaPathPtr := flag.String("video-path", "", "Path to find videos in")
	flag.Parse()

	if *mediaPathPtr == "" {
		log.Fatal("No video path provided")
	}

	monitorRects := listMonitors()

	err := vlc.Init()
	if err != nil {
		log.Panic(err)
	}

	var windows []*VideoMainWindow

	for _, mon := range monitorRects {
		rect := mon.Rect
		var videoWindow *VideoMainWindow = &VideoMainWindow{
			MediaPath: *mediaPathPtr,
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

	windows[0].mainWindow.Run()

	for _, vmw := range windows {
		vmw.Deinit()
	}

	vlc.Release()
}
