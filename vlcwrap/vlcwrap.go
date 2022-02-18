package vlcwrap

// The MIT License (MIT)

// Copyright (c) 2015 Adrian-George Bostan <adrg@epistack.com>

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// This exists purely so we can dynamically load libvlc.dll so it need not
// be system-installed or in the same path as our exe. If go had a linker
// with support for /DELAYLOAD, we could probably avoid this entire mess.
//
// Code copied verbatim from https://github.com/adrg/libvlc-go then modified
// for dynamic loading.

/*
#include <stdlib.h>

#include <vlc/vlc.h>

extern void eventDispatch(libvlc_event_t*, void*);
extern int load_vlc_library(const char *);

static inline int eventAttach(libvlc_event_manager_t* em, libvlc_event_type_t et, unsigned long userData) {
    return libvlc_event_attach(em, et, (void (*)(const libvlc_event_t*, void*))eventDispatch, (void*)(intptr_t)userData);
}

static inline int eventDetach(libvlc_event_manager_t* em, libvlc_event_type_t et, unsigned long userData) {
    libvlc_event_detach(em, et, (void (*)(const libvlc_event_t*, void*))eventDispatch, (void*)(intptr_t)userData);
}
*/
import "C"
import (
	"errors"
	"os"
	"sync"
	"unsafe"
)

type EventID uint64

type Player struct {
	player *C.libvlc_media_player_t
}

type Media struct {
	media *C.libvlc_media_t
}

// Event represents an event that can occur inside libvlc.
type Event int

// EventManager wraps a libvlc event manager.
type EventManager struct {
	manager *C.libvlc_event_manager_t
}

// EventCallback represents an event notification callback function.
type EventCallback func(Event, interface{})

type internalEventCallback func(*C.libvlc_event_t, interface{})

type eventContext struct {
	event            Event
	externalCallback EventCallback
	internalCallback internalEventCallback
	userData         interface{}
}

type eventRegistry struct {
	sync.RWMutex

	contexts map[EventID]*eventContext
	sequence EventID
}

type objectContext struct {
	refs uint
	data interface{}
}

type objectRegistry struct {
	sync.RWMutex

	contexts map[objectID]*objectContext
}

type instance struct {
	handle  *C.libvlc_instance_t
	events  *eventRegistry
	objects *objectRegistry
}

type mediaData struct {
	readerID objectID
	userData interface{}
}

type objectID = unsafe.Pointer

var (
	ErrPlayerCreate         = errors.New("could not create player")
	ErrPlayerNotInitialized = errors.New("player not initialized")
	ErrMediaNotInitialized  = errors.New("media not initialized")
	ErrMediaCreate          = errors.New("media TODO")
	ErrMissingEventManager  = errors.New("eventmanager TODO")
	ErrInvalidEventCallback = errors.New("event TODO")
	ErrModuleNotInitialized = errors.New("module not initialized")
	ErrModuleInitialize     = errors.New("could not initialize module")
	ErrLibraryLoad          = errors.New("could not load shared library")
	ErrAudioOutputSet       = errors.New("audio output TODO")
)

// Player events.
const (
	MediaPlayerMediaChanged Event = 0x100 + iota
	MediaPlayerNothingSpecial
	MediaPlayerOpening
	MediaPlayerBuffering
	MediaPlayerPlaying
	MediaPlayerPaused
	MediaPlayerStopped
	MediaPlayerForward
	MediaPlayerBackward
	MediaPlayerEndReached
	MediaPlayerEncounteredError
	MediaPlayerTimeChanged
	MediaPlayerPositionChanged
	MediaPlayerSeekableChanged
	MediaPlayerPausableChanged
	MediaPlayerTitleChanged
	MediaPlayerSnapshotTaken
	MediaPlayerLengthChanged
	MediaPlayerVout
	MediaPlayerScrambledChanged
	MediaPlayerESAdded
	MediaPlayerESDeleted
	MediaPlayerESSelected
	MediaPlayerCorked
	MediaPlayerUncorked
	MediaPlayerMuted
	MediaPlayerUnmuted
	MediaPlayerAudioVolume
	MediaPlayerAudioDevice
	MediaPlayerChapterChanged
)

var inst *instance = nil

func getError() error {
	msg := C.libvlc_errmsg()
	if msg == nil {
		return nil
	}

	err := errors.New(C.GoString(msg))
	C.libvlc_clearerr()
	return err
}

func errOrDefault(err, defaultErr error) error {
	if err != nil {
		return err
	}

	return defaultErr
}

func (i *instance) assertInit() error {
	if i == nil || i.handle == nil {
		return ErrModuleNotInitialized
	}

	return nil
}

// Init creates an instance of the libVLC module.
// Must be called only once and the module instance must be released using
// the Release function.
func Init(vlcPath string, args ...string) error {
	if inst != nil {
		return nil
	}

	argc := len(args)
	argv := make([]*C.char, argc)

	for i, arg := range args {
		argv[i] = C.CString(arg)
	}
	defer func() {
		for i := range argv {
			C.free(unsafe.Pointer(argv[i]))
		}
	}()

	// Hack: new code: add dynamic library load
	if C.load_vlc_library(C.CString(vlcPath+"\\libvlc.dll")) == 0 {
		// FIXME: error msg
		// log.Printf("Failed to load library %v", vlcPath+"\\libvlc.dll")
		return ErrLibraryLoad
	}
	// End: new code

	handle := C.libvlc_new(C.int(argc), *(***C.char)(unsafe.Pointer(&argv)))
	if handle == nil {
		return errOrDefault(getError(), ErrModuleInitialize)
	}

	inst = &instance{
		handle:  handle,
		events:  newEventRegistry(),
		objects: newObjectRegistry(),
	}

	return nil
}

// Release destroys the instance created by the Init function.
func Release() error {
	if inst == nil {
		return nil
	}

	C.libvlc_release(inst.handle)
	inst = nil

	return getError()
}

// NewPlayer creates an instance of a single-media player.
func NewPlayer() (*Player, error) {
	if err := inst.assertInit(); err != nil {
		return nil, err
	}

	player := C.libvlc_media_player_new(inst.handle)
	if player == nil {
		return nil, errOrDefault(getError(), ErrPlayerCreate)
	}

	return &Player{player: player}, nil
}

func (p *Player) assertInit() error {
	if p == nil || p.player == nil {
		return ErrPlayerNotInitialized
	}

	return nil
}

// SetHWND sets a Windows API window handle where the media player can render
// its video output. If libVLC was built without Win32/Win64 API output
// support, calling this method has no effect.
//   NOTE: By default, libVLC captures input events on the video rendering area.
//   Use the SetMouseInput and SetKeyInput methods if you want to handle input
//   events in your application.
func (p *Player) SetHWND(hwnd uintptr) error {
	if err := p.assertInit(); err != nil {
		return err
	}

	C.libvlc_media_player_set_hwnd(p.player, unsafe.Pointer(hwnd))
	return getError()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}

	return 0
}

// SetKeyInput enables or disables key press event handling, according to the
// libVLC hotkeys configuration. By default, keyboard events are handled by
// the libVLC video widget.
//   NOTE: This method works only for X11 and Win32 at the moment.
//   NOTE: On X11, there can be only one subscriber for key press and mouse
//   click events per window. If your application has subscribed to these
//   events for the X window ID of the video widget, then libVLC will not be
//   able to handle key presses and mouse clicks.
func (p *Player) SetKeyInput(enable bool) error {
	if err := p.assertInit(); err != nil {
		return err
	}

	C.libvlc_video_set_key_input(p.player, C.uint(boolToInt(enable)))
	return getError()
}

// SetMouseInput enables or disables mouse click event handling. By default,
// mouse events are handled by the libVLC video widget. This is needed for DVD
// menus to work, as well as for a few video filters, such as "puzzle".
//   NOTE: This method works only for X11 and Win32 at the moment.
//   NOTE: On X11, there can be only one subscriber for key press and mouse
//   click events per window. If your application has subscribed to these
//   events for the X window ID of the video widget, then libVLC will not be
//   able to handle key presses and mouse clicks.
func (p *Player) SetMouseInput(enable bool) error {
	if err := p.assertInit(); err != nil {
		return err
	}

	C.libvlc_video_set_mouse_input(p.player, C.uint(boolToInt(enable)))
	return getError()
}

// LoadMediaFromPath loads the media located at the specified path and sets
// it as the current media of the player.
func (p *Player) LoadMediaFromPath(path string) (*Media, error) {
	return p.loadMedia(path, true)
}

func (p *Player) loadMedia(path string, local bool) (*Media, error) {
	m, err := newMedia(path, local)
	if err != nil {
		return nil, err
	}

	if err = p.setMedia(m); err != nil {
		m.release()
		return nil, err
	}

	return m, nil
}

func newMedia(path string, local bool) (*Media, error) {
	if err := inst.assertInit(); err != nil {
		return nil, err
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var media *C.libvlc_media_t
	if local {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}

		media = C.libvlc_media_new_path(inst.handle, cPath)
	} else {
		media = C.libvlc_media_new_location(inst.handle, cPath)
	}

	if media == nil {
		return nil, errOrDefault(getError(), ErrMediaCreate)
	}

	return &Media{media: media}, nil
}

func (p *Player) setMedia(m *Media) error {
	if err := p.assertInit(); err != nil {
		return err
	}
	if err := m.assertInit(); err != nil {
		return err
	}

	C.libvlc_media_player_set_media(p.player, m.media)
	return getError()
}

// SetAudioOutput sets the audio output to be used by the player. Any change
// will take effect only after playback is stopped and restarted. The audio
// output cannot be changed while playing.
func (p *Player) SetAudioOutput(output string) error {
	if err := p.assertInit(); err != nil {
		return err
	}

	cOutput := C.CString(output)
	defer C.free(unsafe.Pointer(cOutput))

	if C.libvlc_audio_output_set(p.player, cOutput) != 0 {
		return errOrDefault(getError(), ErrAudioOutputSet)
	}

	return nil
}

func (m *Media) assertInit() error {
	if m == nil || m.media == nil {
		return ErrMediaNotInitialized
	}

	return nil
}

// SetMute mutes or unmutes the audio output of the player.
//   NOTE: If there is no active audio playback stream, the mute status might
//   not be available. If digital pass-through (S/PDIF, HDMI, etc.) is in use,
//   muting may not be applicable.
//   Some audio output plugins do not support muting.
func (p *Player) SetMute(mute bool) error {
	if err := p.assertInit(); err != nil {
		return err
	}

	C.libvlc_audio_set_mute(p.player, C.int(boolToInt(mute)))
	return getError()
}

// Play plays the current media.
func (p *Player) Play() error {
	if err := p.assertInit(); err != nil {
		return err
	}
	if p.IsPlaying() {
		return nil
	}

	if C.libvlc_media_player_play(p.player) < 0 {
		return getError()
	}

	return nil
}

// IsPlaying returns a boolean value specifying if the player is currently
// playing.
func (p *Player) IsPlaying() bool {
	if err := p.assertInit(); err != nil {
		return false
	}

	return C.libvlc_media_player_is_playing(p.player) != 0
}

// newEventManager returns a new event manager instance.
func newEventManager(manager *C.libvlc_event_manager_t) *EventManager {
	return &EventManager{
		manager: manager,
	}
}

// EventManager returns the event manager responsible for the media player.
func (p *Player) EventManager() (*EventManager, error) {
	if err := p.assertInit(); err != nil {
		return nil, err
	}

	manager := C.libvlc_media_player_event_manager(p.player)
	if manager == nil {
		return nil, ErrMissingEventManager
	}

	return newEventManager(manager), nil
}

// Attach registers a callback for an event notification.
func (em *EventManager) Attach(event Event, callback EventCallback, userData interface{}) (EventID, error) {
	return em.attach(event, callback, nil, userData)
}

// Detach unregisters the specified event notification.
func (em *EventManager) Detach(eventIDs ...EventID) {
	if err := inst.assertInit(); err != nil {
		return
	}

	for _, eventID := range eventIDs {
		ctx, ok := inst.events.get(eventID)
		if !ok {
			continue
		}

		inst.events.remove(eventID)
		C.eventDetach(em.manager, C.libvlc_event_type_t(ctx.event), C.ulong(eventID))
	}
}

// attach registers callbacks for an event notification.
func (em *EventManager) attach(event Event, externalCallback EventCallback,
	internalCallback internalEventCallback, userData interface{}) (EventID, error) {
	if err := inst.assertInit(); err != nil {
		return 0, err
	}
	if externalCallback == nil && internalCallback == nil {
		return 0, ErrInvalidEventCallback
	}

	id := inst.events.add(event, externalCallback, internalCallback, userData)
	if C.eventAttach(em.manager, C.libvlc_event_type_t(event), C.ulong(id)) != 0 {
		return 0, getError()
	}

	return id, nil
}

func newEventRegistry() *eventRegistry {
	return &eventRegistry{
		contexts: map[EventID]*eventContext{},
	}
}

func (er *eventRegistry) add(event Event, externalCallback EventCallback,
	internalCallback internalEventCallback, userData interface{}) EventID {
	er.Lock()

	er.sequence++
	id := er.sequence

	er.contexts[id] = &eventContext{
		event:            event,
		externalCallback: externalCallback,
		internalCallback: internalCallback,
		userData:         userData,
	}

	er.Unlock()
	return id
}

//export eventDispatch
func eventDispatch(event *C.libvlc_event_t, userData unsafe.Pointer) {
	if err := inst.assertInit(); err != nil {
		return
	}

	ctx, ok := inst.events.get(EventID(uintptr(userData)))
	if !ok {
		return
	}

	// Execute external callback.
	if ctx.externalCallback != nil {
		ctx.externalCallback(ctx.event, ctx.userData)
	}

	// Execute internal callback.
	if ctx.internalCallback != nil {
		ctx.internalCallback(event, ctx.userData)
	}
}

func (er *eventRegistry) get(id EventID) (*eventContext, bool) {
	if id == 0 {
		return nil, false
	}

	er.RLock()
	ctx, ok := er.contexts[id]
	er.RUnlock()

	return ctx, ok
}

func (er *eventRegistry) remove(id EventID) {
	if id == 0 {
		return
	}

	er.Lock()
	delete(er.contexts, id)
	er.Unlock()
}

// Media returns the current media of the player, if one exists.
func (p *Player) Media() (*Media, error) {
	if err := p.assertInit(); err != nil {
		return nil, err
	}

	media := C.libvlc_media_player_get_media(p.player)
	if media == nil {
		return nil, nil
	}

	// This call will not release the media. Instead, it will decrement
	// the reference count increased by libvlc_media_player_get_media.
	C.libvlc_media_release(media)

	return &Media{media}, nil
}

// Stop cancels the currently playing media, if there is one.
func (p *Player) Stop() error {
	if err := p.assertInit(); err != nil {
		return err
	}

	C.libvlc_media_player_stop(p.player)
	return getError()
}

// Release destroys the media player instance.
func (p *Player) Release() error {
	if err := p.assertInit(); err != nil {
		return nil
	}

	C.libvlc_media_player_release(p.player)
	p.player = nil

	return getError()
}

func (m *Media) getUserData() (objectID, *mediaData) {
	if err := inst.assertInit(); err != nil {
		return nil, nil
	}
	id := C.libvlc_media_get_user_data(m.media)

	obj, ok := inst.objects.get(id)
	if !ok {
		return nil, nil
	}

	data, ok := obj.(*mediaData)
	if !ok {
		return nil, nil
	}

	return id, data
}

func (m *Media) deleteUserData() {
	id, data := m.getUserData()
	if data == nil {
		return
	}

	inst.objects.decRefs(data.readerID)
	inst.objects.decRefs(id)
}

func (m *Media) release() {
	// Delete user data.
	m.deleteUserData()

	// Delete media.
	C.libvlc_media_release(m.media)
	m.media = nil
}

// Release destroys the media instance.
func (m *Media) Release() error {
	if err := m.assertInit(); err != nil {
		return nil
	}

	m.release()
	return getError()
}

func newObjectRegistry() *objectRegistry {
	return &objectRegistry{
		contexts: map[objectID]*objectContext{},
	}
}

func (or *objectRegistry) get(id objectID) (interface{}, bool) {
	if id == nil {
		return nil, false
	}

	or.RLock()
	ctx, ok := or.contexts[id]
	or.RUnlock()

	if !ok {
		return nil, false
	}
	return ctx.data, ok
}

func (or *objectRegistry) decRefs(id objectID) {
	if id == nil {
		return
	}

	or.Lock()

	ctx, ok := or.contexts[id]
	if ok {
		ctx.refs--
		if ctx.refs == 0 {
			delete(or.contexts, id)
			C.free(id)
		}
	}

	or.Unlock()
}

//////////////////////////////////////////////////////////////////////////////
// Sam was here

func AudioOutputList() []string {
	audioOutputList := C.libvlc_audio_output_list_get(inst.handle)
	defer C.libvlc_audio_output_list_release(audioOutputList)

	var ret []string

	var iter = audioOutputList
	for iter != nil {
		ret = append(ret, C.GoString(iter.psz_name))
		iter = iter.p_next
	}

	return ret
}
