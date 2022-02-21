#include <vlc/vlc.h>

#define STUB___0(func)                  \
    typedef void (*TYPE_##func)(void);  \
    void (*PTR_##func)(void);           \
    void func(void)                     \
    { PTR_##func(); }

#define STUB___1(func, arg1)            \
    typedef void (*TYPE_##func)(arg1);  \
    void (*PTR_##func)(arg1);           \
    void func(arg1 _a1)                 \
    { PTR_##func(_a1); }

#define STUB___2(func, arg1, arg2)      \
    typedef void (*TYPE_##func)(arg1, arg2); \
    void (*PTR_##func)(arg1, arg2);     \
    void func(arg1 _a1, arg2 _a2)       \
    { PTR_##func(_a1, _a2); }

#define STUB___4(func, arg1, arg2, arg3, arg4)              \
    typedef void (*TYPE_##func)(arg1, arg2, arg3, arg4);    \
    void (*PTR_##func)(arg1, arg2, arg3, arg4);             \
    void func(arg1 _a1, arg2 _a2, arg3 _a3, arg4 _a4)       \
    { PTR_##func(_a1, _a2, _a3, _a4); }

#define STUB_R_0(ret, func)             \
    typedef ret (*TYPE_##func)(void);   \
    ret (*PTR_##func)(void);            \
    ret func(void)                      \
    { return PTR_##func(); }

#define STUB_R_1(ret, func, arg1)       \
    typedef ret (*TYPE_##func)(arg1);   \
    ret (*PTR_##func)(arg1);            \
    ret func(arg1 _a1)                  \
    { return PTR_##func(_a1); }

#define STUB_R_2(ret, func, arg1, arg2) \
    typedef ret (*TYPE_##func)(arg1, arg2); \
    ret (*PTR_##func)(arg1, arg2);      \
    ret func(arg1 _a1, arg2 _a2)        \
    { return PTR_##func(_a1, _a2); }

#define STUB_R_4(ret, func, arg1, arg2, arg3, arg4)     \
    typedef ret (*TYPE_##func)(arg1, arg2, arg3, arg4); \
    ret (*PTR_##func)(arg1, arg2, arg3, arg4);          \
    ret func(arg1 _a1, arg2 _a2, arg3 _a3, arg4 _a4)    \
    { return PTR_##func(_a1, _a2, _a3, _a4); }

STUB_R_2(libvlc_instance_t *, libvlc_new, int, const char *const *);
STUB___1(libvlc_release, libvlc_instance_t *);
STUB_R_0(const char *, libvlc_errmsg);
STUB___0(libvlc_clearerr);
STUB___1(libvlc_media_release, libvlc_media_t *);
STUB_R_2(libvlc_media_t*, libvlc_media_new_path, libvlc_instance_t *, const char *);
STUB_R_2(libvlc_media_t*, libvlc_media_new_location, libvlc_instance_t *, const char *);
STUB_R_1(void*, libvlc_media_get_user_data, libvlc_media_t *);
STUB___2(libvlc_video_set_key_input, libvlc_media_player_t *, unsigned);
STUB___2(libvlc_video_set_mouse_input, libvlc_media_player_t *, unsigned);
STUB_R_4(int, libvlc_event_attach, libvlc_event_manager_t *, libvlc_event_type_t, libvlc_callback_t, void *);
STUB___4(libvlc_event_detach, libvlc_event_manager_t *, libvlc_event_type_t, libvlc_callback_t, void *);
STUB___2(libvlc_audio_set_mute, libvlc_media_player_t *, int);
STUB_R_1(libvlc_event_manager_t *, libvlc_media_player_event_manager, libvlc_media_player_t *);
STUB_R_1(libvlc_media_t *, libvlc_media_player_get_media, libvlc_media_player_t *);
STUB_R_1(int, libvlc_media_player_is_playing, libvlc_media_player_t *);
STUB_R_1(libvlc_media_player_t*, libvlc_media_player_new, libvlc_instance_t *);
STUB_R_1(int, libvlc_media_player_play, libvlc_media_player_t *);
STUB___1(libvlc_media_player_release, libvlc_media_player_t *);
STUB___2(libvlc_media_player_set_hwnd, libvlc_media_player_t *, void *);
STUB___2(libvlc_media_player_set_media,	libvlc_media_player_t *, libvlc_media_t *);
STUB___1(libvlc_media_player_stop, libvlc_media_player_t *);
STUB_R_1(libvlc_audio_output_t*, libvlc_audio_output_list_get,	libvlc_instance_t *);
STUB___1(libvlc_audio_output_list_release, libvlc_audio_output_t *);
STUB_R_2(int, libvlc_audio_output_set, libvlc_media_player_t *, const char *);


#define WIN32_LEAN_AND_MEAN
#include <windows.h>

int load_vlc_library(void)
{
    HMODULE lib = LoadLibrary("libvlc.dll");

    if (!lib)
        return 0;

#define LOAD(func) \
    PTR_##func = (TYPE_##func)GetProcAddress(lib, #func)

    LOAD(libvlc_new);
    LOAD(libvlc_release);
    LOAD(libvlc_errmsg);
    LOAD(libvlc_clearerr);
    LOAD(libvlc_media_release);
    LOAD(libvlc_media_new_path);
    LOAD(libvlc_media_new_location);
    LOAD(libvlc_media_get_user_data);
    LOAD(libvlc_video_set_key_input);
    LOAD(libvlc_video_set_mouse_input);
    LOAD(libvlc_event_attach);
    LOAD(libvlc_event_detach);
    LOAD(libvlc_audio_set_mute);
    LOAD(libvlc_media_player_event_manager);
    LOAD(libvlc_media_player_get_media);
    LOAD(libvlc_media_player_is_playing);
    LOAD(libvlc_media_player_new);
    LOAD(libvlc_media_player_play);
    LOAD(libvlc_media_player_release);
    LOAD(libvlc_media_player_set_hwnd);
    LOAD(libvlc_media_player_set_media);
    LOAD(libvlc_media_player_stop);
    LOAD(libvlc_audio_output_list_get);
    LOAD(libvlc_audio_output_list_release);
    LOAD(libvlc_audio_output_set);

#undef LOAD

    return 1;
}