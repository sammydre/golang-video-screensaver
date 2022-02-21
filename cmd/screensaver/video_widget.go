package main

import (
	"log"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	vlc "github.com/sammydre/golang-video-screensaver/vlcwrap"
)

type VlcVideoWidget struct {
	walk.WidgetBase

	screenSaverFinishCallback func()
	nextMediaFileCallback     func() string
	synchroniseCallback       func(func())
	cursorPos                 win.POINT
	videoPlayer               *vlc.Player
	endReachedEventId         vlc.EventID
	hwndForVlc                win.HWND
}

const VlcVideoWidgetWindowClass = "VLC Video Widget Class"

func NewVlcVideoWidget(parent walk.Container, finishCallback func(), mediaPathCallback func() string, synchroniseCallback func(func())) (*VlcVideoWidget, error) {
	w := new(VlcVideoWidget)
	w.screenSaverFinishCallback = finishCallback
	w.nextMediaFileCallback = mediaPathCallback
	w.synchroniseCallback = synchroniseCallback

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

	w.hwndForVlc = w.AsWindowBase().Handle()

	return w, nil
}

func NewPreviewVlcVideoWidget(parent win.HWND, mediaPathCallback func() string, synchroniseCallback func(func())) (*VlcVideoWidget, error) {
	w := new(VlcVideoWidget)
	w.screenSaverFinishCallback = func() {}
	w.nextMediaFileCallback = mediaPathCallback
	w.synchroniseCallback = synchroniseCallback
	w.hwndForVlc = parent

	return w, nil
}

func (vvw *VlcVideoWidget) SetupVlcPlayer() {
	var err error

	log.Print("Creating and initialising VLC player...")

	vvw.videoPlayer, err = vlc.NewPlayer()
	if err != nil {
		log.Panic(err)
	}

	err = vvw.videoPlayer.SetHWND(uintptr(vvw.hwndForVlc))
	if err != nil {
		log.Panic(err)
	}

	err = vvw.videoPlayer.SetKeyInput(false)
	if err != nil {
		log.Panic(err)
	}

	err = vvw.videoPlayer.SetMouseInput(false)
	if err != nil {
		log.Panic(err)
	}

	err = vvw.videoPlayer.SetAudioOutput("adummy")
	if err != nil {
		log.Print(err)
	}

	err = vvw.videoPlayer.SetMute(true)
	if err != nil {
		log.Panic(err)
	}

	mediaFileName := vvw.nextMediaFileCallback()

	_, err = vvw.videoPlayer.LoadMediaFromPath(mediaFileName)
	if err != nil {
		log.Panic(err)
	}

	manager, err := vvw.videoPlayer.EventManager()
	if err != nil {
		log.Panic(err)
	}

	endReachedCallback := func(event vlc.Event, userData interface{}) {
		// This callback is called from a somewhat uncertain context. I don't think
		// we can safely call vlc functions in this state? (Maybe its not re-entrant?)
		vvw.synchroniseCallback(func() {
			mediaFileName := vvw.nextMediaFileCallback()
			vvw.videoPlayer.LoadMediaFromPath(mediaFileName)
			vvw.videoPlayer.Play()
		})
	}

	vvw.endReachedEventId, err = manager.Attach(vlc.MediaPlayerEndReached, endReachedCallback, nil)
	if err != nil {
		log.Panic(err)
	}

	log.Print("VLC player initialised, playing")
	vvw.videoPlayer.Play()
}

func (vvw *VlcVideoWidget) Deinit() {
	if vvw.videoPlayer != nil {
		manager, err := vvw.videoPlayer.EventManager()
		if err != nil {
			log.Panic(err)
		}

		manager.Detach(vvw.endReachedEventId)

		if media, _ := vvw.videoPlayer.Media(); media != nil {
			media.Release()
		}

		vvw.videoPlayer.Stop()
		vvw.videoPlayer.Release()
		vvw.videoPlayer = nil
	}
}

func (*VlcVideoWidget) CreateLayoutItem(ctx *walk.LayoutContext) walk.LayoutItem {
	return &vlcVideoWidgetLayoutItem{idealSize: walk.SizeFrom96DPI(walk.Size{Width: 150, Height: 150}, ctx.DPI())}
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
			w.screenSaverFinishCallback()
		}
	case win.WM_LBUTTONDOWN, win.WM_RBUTTONDOWN, win.WM_MBUTTONDOWN, win.WM_XBUTTONDOWN, win.WM_KEYDOWN, win.WM_KEYUP, win.WM_SYSKEYDOWN:
		w.screenSaverFinishCallback()
	case win.WM_MOUSEMOVE:
		var point = win.POINT{
			X: int32(win.GET_X_LPARAM(lParam)),
			Y: int32(win.GET_Y_LPARAM(lParam))}
		if point.X != w.cursorPos.X || point.Y != w.cursorPos.Y {
			w.screenSaverFinishCallback()
		}
	case win.WM_SETCURSOR:
		return 1
	}

	return w.WidgetBase.WndProc(hwnd, msg, wParam, lParam)
}
