package main

import "C"

import (
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

	// vlc "github.com/adrg/libvlc-go/v3"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	vlc "github.com/sammydre/golang-video-screensaver/vlcwrap"
)

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

func main() {
	initRand()

	win.CoInitializeEx(nil, win.COINIT_MULTITHREADED)

	// Standard arguments screensavers need to accept
	_ = flag.Int64("a", 0, "Change the password; only for Win9x, unused on WinNT")
	_ = flag.Bool("s", false, "Run the screensaver")
	_ = flag.Int64("p", 0, "Preview")
	_ = flag.Int64("c", 0, "Configure")

	// Own own additional arguments
	mediaPathPtr := flag.String("video-path", "", "Path to find videos in")

	flag.Parse()

	if *mediaPathPtr == "" {
		log.Fatal("No video path provided")
		// *mediaPathPtr = ""
	}

	monitorRects := listMonitors()

	err := vlc.Init()
	if err != nil {
		log.Panic(err)
	}

	walk.AppendToWalkInit(func() {
		walk.MustRegisterWindowClass(VlcVideoWidgetWindowClass)
	})

	var windows []*VideoWindowContext

	for _, mon := range monitorRects {
		rect := mon.Rect
		var videoWindow *VideoWindowContext = &VideoWindowContext{
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
