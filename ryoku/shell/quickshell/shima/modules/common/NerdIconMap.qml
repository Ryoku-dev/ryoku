pragma Singleton
import QtQuick

QtObject {
    // Material icon name -> Nerd Font glyph
    readonly property var map: ({
        // Navigation
        "home": "󰋜", "settings": "󰒓", "search": "󰍉", "menu": "󰍜", "close": "󰅖",
        "arrow_back": "󰁍", "arrow_forward": "󰁔", "expand_more": "󰁅", "expand_less": "󰁝",
        "chevron_left": "󰁍", "chevron_right": "󰁔", "refresh": "󰑓", "more_vert": "󰇙",
        "more_horiz": "󰇘", "apps": "󰀻", "dashboard": "󰕮", "grid_view": "󰕰",
        
        // Media
        "play_arrow": "󰐊", "pause": "󰏤", "stop": "󰓛", "skip_next": "󰒭", "skip_previous": "󰒮",
        "fast_forward": "󰒍", "fast_rewind": "󰒊", "shuffle": "󰒟", "repeat": "󰑖", "repeat_one": "󰑘",
        "volume_up": "󰕾", "volume_down": "󰖀", "volume_mute": "󰖁", "volume_off": "󰖁",
        "music_note": "󰎈", "album": "󰀥", "queue_music": "󰲸", "playlist_play": "󰲲",
        "mic": "󰍬", "mic_off": "󰍭", "headphones": "󰋋", "speaker": "󰓃",
        
        // System
        "memory": "󰍛", "storage": "󰋊", "thermostat": "󰔏", "device_thermostat": "󰔏",
        "battery_full": "󰁹", "battery_charging_full": "󰂄", "battery_0_bar": "󰂎",
        "battery_android_full": "󰁹",
        "wifi": "󰖩", "wifi_off": "󰖪", "signal_wifi_4_bar": "󰖩",
        "bluetooth": "󰂯", "bluetooth_disabled": "󰂲", "bluetooth_connected": "󰂱",
        "brightness_high": "󰃟", "brightness_medium": "󰃝", "brightness_low": "󰃞",
        "power_settings_new": "󰐥", "restart_alt": "󰜉", "logout": "󰍃",
        "lock": "󰌾", "lock_open": "󰌿", "vpn_key": "󰌋",
        "computer": "󰇄", "desktop_windows": "󰇄", "laptop": "󰌢", "smartphone": "󰄜",
        "monitor": "󰍹", "keyboard": "󰌌", "mouse": "󰍽",
        
        // Files & Folders
        "folder": "󰉋", "folder_open": "󰝰", "create_new_folder": "󰉗",
        "file_present": "󰈔", "description": "󰈙", "article": "󰈙",
        "image": "󰋩", "photo": "󰋩", "photo_library": "󰉏", "collections": "󰉏",
        "video_file": "󰕧", "audio_file": "󰎈",
        "picture_as_pdf": "󰈦", "code": "󰅩", "terminal": "󰆍",
        "download": "󰇚", "upload": "󰕒", "cloud_download": "󰇚", "cloud_upload": "󰕒",
        "save": "󰆓", "save_as": "󰆔",
        
        // Edit & Actions  
        "edit": "󰏫", "delete": "󰩹", "content_copy": "󰆏", "content_paste": "󰆒",
        "content_cut": "󰆐", "undo": "󰕌", "redo": "󰑎", "select_all": "󰒉",
        "add": "󰐕", "remove": "󰍴", "add_circle": "󰐕", "remove_circle": "󰍴",
        "check": "󰄬", "check_circle": "󰄬", "cancel": "󰅖", "block": "󰂭",
        "link": "󰌷", "link_off": "󰌸", "share": "󰒪", "send": "󰒊",
        "print": "󰐪", "attach_file": "󰏢",
        
        // Status & Alerts
        "error": "󰅚", "warning": "󰀦", "info": "󰋽", "help": "󰋗",
        "notifications": "󰂞", "notifications_off": "󰂛", "notifications_active": "󰂟",
        "do_not_disturb_on": "󰂛", "do_not_disturb_off": "󰂞",
        "check_box": "󰄵", "check_box_outline_blank": "󰄱",
        "radio_button_checked": "󰐾", "radio_button_unchecked": "󰐽",
        "star": "󰓎", "star_border": "󰓏", "star_half": "󰓐",
        "favorite": "󰋑", "favorite_border": "󰋕",
        "bookmark": "󰃀", "bookmark_border": "󰃃", "flag": "󰈻",
        "label": "󰓹", "sell": "󰓹",
        
        // Time & Calendar
        "schedule": "󰥔", "access_time": "󰥔", "timer": "󱎫", "alarm": "󰀠",
        "calendar_today": "󰃭", "calendar_month": "󰃭", "event": "󰃭", "event_note": "󰃭",
        
        // Anime/TV
        "live_tv": "󰔂", "tv": "󰔂", "movie": "󰎁", "theaters": "󰎁",
        "today": "󰃶", "date_range": "󰃭",
        "hourglass_empty": "󰞌", "hourglass_full": "󰞎",
        
        // Weather
        "wb_sunny": "󰖙", "light_mode": "󰖙", "dark_mode": "󰖔", "nightlight": "󰖔",
        "cloud": "󰖐", "cloud_queue": "󰖐", "water_drop": "󰖗", "rainy": "󰖗",
        "ac_unit": "󰼶", "thunderstorm": "󰖓", "air": "󰖝",
        
        // People & Social
        "person": "󰀄", "people": "󰡉", "group": "󰡉", "groups": "󰡉",
        "account_circle": "󰀄", "face": "󰀄",
        "chat": "󰍡", "forum": "󰍩", "message": "󰍦", "mail": "󰇮", "email": "󰇮",
        "call": "󰏲", "phone": "󰏲", "videocam": "󰕧", "camera": "󰄀",
        
        // Media controls
        "fullscreen": "󰊓", "fullscreen_exit": "󰊔", "picture_in_picture": "󰐜",
        "crop": "󰆞", "zoom_in": "󰐭", "zoom_out": "󰐯",
        
        // Development
        "bug_report": "󰃤", "build": "󰣖", "extension": "󰏗", "api": "󰘦",
        "developer_mode": "󰀼",
        
        // Gaming
        "sports_esports": "󰊴", "gamepad": "󰊴", "videogame_asset": "󰊴",
        "casino": "󰣎", "emoji_events": "󰸏",
        
        // Misc UI
        "visibility": "󰈈", "visibility_off": "󰈉",
        "palette": "󰏘", "color_lens": "󰏘", "colorize": "󰴱",
        "tune": "󰔎", "equalizer": "󰕪", "graphic_eq": "󰕪",
        "sort": "󰒺", "filter_list": "󰈲", "filter_alt": "󰈲",
        "view_list": "󰕮", "view_module": "󰕰",
        "drag_indicator": "󰍜", "drag_handle": "󰍜",
        "open_in_new": "󰏌", "launch": "󰏌", "exit_to_app": "󰍃",
        "screenshot_region": "󰹑", "screenshot": "󰹑", "screen_share": "󰹑",
        "edit_note": "󰏫", "note_add": "󰎞", "sticky_note_2": "󰎞",
        "inventory_2": "󰆼", "category": "󰆼",
        "bolt": "󰂄", "flash_on": "󰂄", "electric_bolt": "󰂄",
        "eco": "󰌪", "park": "󰌪",
        "public": "󰖟", "language": "󰖟", "translate": "󰗊",
        "lightbulb": "󰛨", "tips_and_updates": "󰛨",
        "auto_awesome": "󰓒", "verified": "󰄬",
        "speed": "󰓅", "rocket_launch": "󰑣", "explore": "󰆋",
        "map": "󰆋", "place": "󰍎", "location_on": "󰍎", "my_location": "󰆋",
        "navigation": "󰆌", "directions": "󰆌", "near_me": "󰆌",
        "move_item": "󰁔", "swap_horiz": "󰓡", "swap_vert": "󰓢",
        "arrow_upward": "󰁝", "arrow_downward": "󰁅",
        "north": "󰁝", "south": "󰁅", "east": "󰁔", "west": "󰁍",
        "first_page": "󰁍", "last_page": "󰁔",
        "keyboard_arrow_up": "󰁝", "keyboard_arrow_down": "󰁅",
        "keyboard_arrow_left": "󰁍", "keyboard_arrow_right": "󰁔",
        "unfold_more": "󰁅", "unfold_less": "󰁝",
        "open_in_full": "󰊓", "close_fullscreen": "󰊔",
        "push_pin": "󰐃", "keep": "󰐃",
        "history": "󰋚", "update": "󰚰", "sync": "󰑓",
        "cached": "󰑓", "autorenew": "󰑓", "replay": "󰑓",
        "done": "󰄬", "done_all": "󰄬", "task_alt": "󰄬",
        "pending": "󰥔", "hourglass_top": "󰞌", "hourglass_bottom": "󰞎",
        "timelapse": "󱎫", "checklist": "󰄵", "checklist_rtl": "󰄵",
        "circle": "󰝥", "square": "󰝤", "hexagon": "󰝣",
        "change_history": "󰔶", "details": "󰔶",
        "format_list_bulleted": "󰉹", "format_list_numbered": "󰉼",
        "list": "󰉹", "toc": "󰉹", "segment": "󰉹",
        "text_fields": "󰊄", "title": "󰊄", "format_size": "󰊄",
        "format_bold": "󰂓", "format_italic": "󰂑", "format_underlined": "󰂖",
        "format_quote": "󰂘", "format_align_left": "󰂔", "format_align_center": "󰂕",
        "format_align_right": "󰂗", "format_align_justify": "󰂓",
        "text_snippet": "󰈙", "short_text": "󰈙", "notes": "󰈙",
        "insert_emoticon": "󰞅", "mood": "󰞅", "sentiment_satisfied": "󰞅",
        "thumb_up": "󰔢", "thumb_down": "󰔡",
        "grade": "󰓎", "workspace_premium": "󰸏",
        
        // Additional icons
        "arrow_drop_down": "󰁅", "arrow_drop_up": "󰁝",
        "backspace": "󰁮", "delete_forever": "󰩺", "delete_outline": "󰩹",
        "bedtime": "󰖔", "wb_twilight": "󰖛",
        "dangerous": "󰀦", "security": "󰒃", "fingerprint": "󰈷", "key": "󰌋",
        "experiment": "󰂓", "deployed_code": "󰅩",
        "drive_file_move": "󰈔", "drive_folder_upload": "󰉍",
        "image_search": "󰋩", "preview": "󰈈",
        "keyboard_hide": "󰌌", "linked_services": "󰌷",
        "music_cast": "󰎈", "travel_explore": "󰖟",
        "right_panel_open": "󰘖", "search_off": "󰍉",
        "stacks": "󰕮", "widgets": "󰘓", "window": "󰖲",
    })
    
    function get(materialIcon) {
        return map[materialIcon] ?? ""
    }
}
