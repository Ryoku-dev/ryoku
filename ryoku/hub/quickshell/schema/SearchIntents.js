.pragma library

// Task language -> the row that owns it. A row's label, description, keywords
// and option values answer what a control is CALLED; this answers what a user
// wants TO DO, in the words they use for it. VS Code needed Bing's
// "alternative words" pipeline for the same reason: no settings vocabulary
// enumerates every phrasing, and "my mouse pointer is too big" names no label.
//
// Every entry names keys that exist in the schemas -- SearchIntents.test.mjs
// asserts it -- so an intent can never navigate to a row nothing draws. The
// words are the searchable phrasings, in English and in the languages Ryoku
// ships; matching is order-independent and needs the WHOLE query, so one
// stray word cannot derail a phrase.
//
// The curation rules the test enforces, learned from the first batch:
//
//  1. a word earns its place only when it names this task and not another. A
//     generic word ("layout", "size", "shadow", "font") also names other
//     settings, so an intent carrying it reorders a query the plain search
//     already answered; those words belong to the row's own haystack.
//  2. an intent may not change the first plain result of any single-word query
//     it fires on, unless the plain search found nothing at all.
//  3. every destination is a row the schemas really carry, owned by exactly
//     one intent.
//
// Curation is deliberately in English and Spanish only (`langs` below): a
// promise about how a phrase reads cannot be made for a language nobody wrote,
// so outside them an intent does not fire at all and the plain ranking stands.
var langs = "en es";

var fillers = "el la los las un una de del al en para por como que cual mi tu su sus se le les lo me ya pero porque cuando cada todo toda cualquier esta este eso estoy tengo hacer poner quiero cambiar cambio cambios donde muy mucho demasiado tambien solo quitar desactivar activar enable disable turned turn the a an my to too very its no not dont is are it of";

var intents = [
    {
        keys: ["desktop.cursor.size"],
        words: "cursor grande puntero pointer too big large huge mouse"
    },
    {
        keys: ["desktop.input.sensitivity"],
        words: "raton lento mouse too slow fast sensitive pointer speed puntero"
    },
    {
        keys: ["desktop.input.kbLayout", "desktop.input.kbVariant"],
        words: "teclado distribucion azerty qwerty dvorak colemak keyboard"
    },
    {
        keys: ["desktop.input.repeatRate", "desktop.input.repeatDelay"],
        words: "tecla repetida repeat rate delay held key velocidad escritura demasiado rapido"
    },
    {
        keys: ["desktop.input.tapToClick", "desktop.input.naturalScroll"],
        words: "touchpad no haciendo nada tap clic scroll touchpad not working trackpad pad"
    },
    {
        keys: ["fontSize", "fontScale"],
        words: "letra pequena texto pequeno font small text legible demasiado interface scale"
    },
    {
        keys: ["language", "formatLocale"],
        words: "idioma espanol spanish english cambiar formato fecha hora numeros currency"
    },
    {
        keys: ["terminalFontFamily", "fontFamily"],
        words: "fuente tipografia typeface terminal monoespaciada consola"
    },
    {
        keys: ["desktop.appearance.activeOpacity", "desktop.appearance.inactiveOpacity",
               "desktop.appearance.fullscreenOpacity"],
        words: "ventana transparente translucida opacity see through no veo detras"
    },
    {
        keys: ["desktop.appearance.rounding", "desktop.appearance.roundingPower"],
        words: "esquinas redondas corner radius rounded softness puntiagudas"
    },
    {
        keys: ["desktop.appearance.gapsIn", "desktop.appearance.gapsOut",
               "desktop.appearance.gapsWorkspaces"],
        words: "separacion espacio entre ventanas gaps spacing inner outer margin hueco"
    },
    {
        keys: ["desktop.appearance.blurSize", "desktop.appearance.blurPasses"],
        words: "desenfoque borroso cristal blurred quitar frosting translucido glass"
    },
    {
        keys: ["desktop.appearance.layout"],
        words: "mosaico tiling dwindle espacial organizar ventanas como colocar"
    },
    {
        keys: ["desktop.appearance.borderSize", "desktop.appearance.activeBorder",
               "desktop.appearance.inactiveBorder"],
        words: "borde grosor thickness color colour foco activo linea grueso ancho fino delgado"
    },
    {
        keys: ["desktop.autostart"],
        words: "arranque al inicio autostart startup programa abre solo aplicacion lanzar"
    },
    {
        keys: ["desktop.env"],
        words: "variable de entorno environment variable export session ruta path"
    },
    {
        keys: ["clipboard.widthPercent", "clipboard.heightPercent"],
        words: "portapapeles clipboard tamano historial ancho alto"
    },
    {
        keys: ["gamebar.enabled", "gamebar.scale"],
        words: "barra juego gamebar overlay gaming mostrar"
    },
    {
        keys: ["idle.enabled", "idle.ac.dimSec", "idle.battery.dimSec"],
        words: "apaga pantalla solo sola solas screen idle timeout bloqueo suspende ahorro energia minutos"
    },
    {
        keys: ["sdrbrightness"],
        words: "brillo pantalla brightness muy oscuro subir bajar luz"
    },
    {
        keys: ["powerProfileEffects", "lowPowerMode"],
        words: "bateria dura poco power ahorro consumo rendimiento perfil bajo"
    },
    {
        keys: ["desktop.cursor.theme"],
        words: "cursor tema puntero theme oscuro claro colores"
    },
    {
        keys: ["liveWallpaper60"],
        words: "fondo animado fps video wallpaper 60 vivo consume"
    },
    {
        keys: ["timezone"],
        words: "zona horaria hora timezone reloj mal cambio de hora"
    }
];