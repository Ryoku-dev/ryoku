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
// Curation is deliberate, not generated: an intent whose words are a common
// single word ("big", "size") would fire on every query that contains it, so
// a word earns its place only when it names this task and not another.
//
// Filler words ride along with a task phrase ("el puntero esta demasiado
// grande"), so an intent lists them beside its nouns. This is not a stop-word
// list for the search engine -- the engine and its haystacks are untouched --
// only the phrases this table answers, and only in the languages it is
// curated for: a query in a language nobody curated degrades to today's
// ranking rather than matching something wrong.
var fillers = "el la los las un una de del al en para por como que cual mi tu su sus esta este eso estoy tengo hacer poner quiero cambiar cambio cambios donde muy mucho demasiado tambien solo quitar desactivar activar enable disable turned turn the a an my to too very its no not dont is are it of";
var intents = [
    {
        keys: ["desktop.cursor.size"],
        words: "cursor grande puntero pointer too big large huge mouse pointer"
    },
    {
        keys: ["desktop.input.sensitivity"],
        words: "raton lento mouse too slow fast sensitive pointer speed puntero"
    },
    {
        keys: ["desktop.input.kbLayout", "desktop.input.kbVariant"],
        words: "teclado distribucion keyboard layout azerty qwerty dvorak colemaq"
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
        words: "letra pequena texto pequeno font size small text legible demasiado grande interface scale"
    },
    {
        keys: ["language", "formatLocale"],
        words: "idioma language espanol spanish english cambiar formato fecha hora numeros currency"
    },
    {
        keys: ["terminalFontFamily", "fontFamily"],
        words: "fuente tipografia typeface terminal font monoespaciada consola"
    },
    {
        keys: ["desktop.appearance.activeOpacity", "desktop.appearance.inactiveOpacity",
               "desktop.appearance.fullscreenOpacity"],
        words: "ventana transparente translucida opacity see through no veo detras ventanas transparentes"
    },
    {
        keys: ["desktop.appearance.rounding", "desktop.appearance.roundingPower"],
        words: "esquinas redondas corner radius rounded softness puntiagudas ventana"
    },
    {
        keys: ["desktop.appearance.gapsIn", "desktop.appearance.gapsOut",
               "desktop.appearance.gapsWorkspaces"],
        words: "separacion espacio entre ventanas gaps spacing inner outer margin hueco"
    },
    {
        keys: ["desktop.appearance.blurEnabled", "desktop.appearance.blurSize"],
        words: "desenfoque borroso cristal blurred quitar blur frosting translucido ventanas glass"
    },
    {
        keys: ["desktop.appearance.shadowEnabled", "desktop.appearance.shadowRange"],
        words: "sombra shadow drop behind windows solapa plano sombras"
    },
    {
        keys: ["desktop.appearance.borderSize", "desktop.appearance.activeBorder",
               "desktop.appearance.inactiveBorder"],
        words: "borde grosor border thickness color colour foco activo linea ventana grueso ancho fino delgado"
    },
    {
        keys: ["desktop.appearance.layout"],
        words: "layout mosaico tiling dwindle master espacial organizar ventanas como colocar"
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
        words: "portapapeles clipboard tamano grande historial ancho alto"
    },
    {
        keys: ["gamebar.enabled", "gamebar.scale"],
        words: "barra juego gamebar overlay gaming mostrar tamano"
    },
    {
        keys: ["idle.enabled", "idle.ac.dimSec", "idle.battery.dimSec"],
        words: "apaga pantalla solo screen idle timeout bloqueo suspende ahorro energia minutos"
    }
];