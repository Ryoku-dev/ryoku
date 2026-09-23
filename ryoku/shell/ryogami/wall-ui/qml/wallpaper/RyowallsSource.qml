import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// A pluggable remote wallpaper source driven by the ryowalls engine's JSON
// verbs (moewalls, motionbgs, ryostore extras): the same search/download shape
// behind one object, so the picker's Browse surface gains sources without a
// bespoke browser each. Results are NDJSON rows: {id, thumb, video, dl, name,
// resolution, ...}. download() saves the master into the video library and
// emits its path; the picker applies it through the ryogami daemon.
QtObject {
    id: src

    property string searchVerb: ""       // e.g. "moewalls-search"
    property string downloadVerb: ""     // e.g. "moewalls-download"
    property bool needsPost: false       // moewalls download wants the post url
    property var extraArgs: []            // e.g. ["--repo", "owner/repo"] for library-list

    // native (no-binary) source paths, phased in per provider; the ryowalls
    // binary stays the fallback until every provider is ported (then it sunsets).
    readonly property string _ua: "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
    readonly property string _ryostoreBase: "https://raw.githubusercontent.com/ryoku-dev/ryostore/main"
    readonly property string _mbBase: "https://motionbgs.com"
    readonly property string _mwBase: "https://moewalls.com"
    property string _nativeProvider: ""
    property string _pendingQuery: ""
    property string _nativeDlBuf: ""
    property string _repoRepo: ""
    property string _repoSub: ""
    property string _repoType: "all"

    property var results: []
    property bool loading: false
    property string error: ""

    property int currentPage: 1
    property int lastPage: 1
    readonly property bool hasMore: currentPage < lastPage

    // client-side pagination cache (extras/repos fetch the whole list once,
    // then slice per page): key marks which query the cache belongs to.
    property var _fullList: []
    property string _fullKey: ""

    // id of the row currently downloading, "" when idle
    property string downloadingId: ""
    signal applied(string path)
    signal failed(string reason)

    property string _buf: ""

    function _clientKey(query) {
        return searchVerb + "|" + ("" + (query || "")) + "|" + JSON.stringify(extraArgs)
    }

    // slice the cached full list for extras/repos into the current page's view
    function _applyClientPage() {
        var per = 24
        var total = _fullList.length
        lastPage = Math.max(1, Math.ceil(total / per))
        if (currentPage > lastPage) currentPage = lastPage
        var start = (currentPage - 1) * per
        results = _fullList.slice(start, start + per)
        error = total === 0 ? I18n.tr("no results") : ""
        loading = false
    }

    function search(query, page) {
        if (!searchVerb || loading) return
        var pg = page || 1
        // client-side sources reslice the cached list on page turns, no refetch
        if ((searchVerb === "extras-search" || searchVerb === "library-list")
                && pg > 1 && _fullKey === _clientKey(query)) {
            currentPage = pg
            _applyClientPage()
            return
        }
        _buf = ""
        error = ""
        loading = true
        currentPage = pg
        results = []
        if (searchVerb === "extras-search") {
            _nativeProvider = "ryostore"
            _pendingQuery = query || ""
            _fullKey = _clientKey(query)
            _nativeSearchProc.command = ["curl", "-fsSL", "-A", _ua, _ryostoreBase + "/livewalls/registry.json"]
            _nativeSearchProc.running = true
            return
        }
        if (searchVerb === "motionbgs-search") {
            _nativeProvider = "motionbgs"
            _pendingQuery = query || ""
            var q2 = ("" + (query || "")).toLowerCase().replace(/ /g, "-")
            var mbPath = q2.length > 0
                ? "/tag:" + q2 + "/" + (pg > 1 ? pg + "/" : "")
                : (pg > 1 ? "/" + pg + "/" : "/")
            _nativeSearchProc.command = ["curl", "-fsSL", "-A", _ua, "-e", _mbBase + "/", _mbBase + mbPath]
            _nativeSearchProc.running = true
            return
        }
        if (searchVerb === "moewalls-search") {
            _nativeProvider = "moewalls"
            _pendingQuery = query || ""
            var mwUrl = (query && query.length > 0)
                ? _mwBase + (pg > 1 ? "/page/" + pg + "/" : "/") + "?s=" + encodeURIComponent(query)
                : _mwBase + "/anime/" + (pg > 1 ? "page/" + pg + "/" : "")
            _nativeSearchProc.command = ["curl", "-fsSL", "-A", _ua, "-e", _mwBase + "/", mwUrl]
            _nativeSearchProc.running = true
            return
        }
        if (searchVerb === "library-list") {
            _nativeProvider = "repos"
            _pendingQuery = query || ""
            _fullKey = _clientKey(query)
            var repo = "", rbranch = "", rsub = "", rtype = "all"
            for (var k = 0; k < extraArgs.length; k++) {
                if (extraArgs[k] === "--repo") repo = "" + (extraArgs[k + 1] || "")
                else if (extraArgs[k] === "--branch") rbranch = "" + (extraArgs[k + 1] || "")
                else if (extraArgs[k] === "--path") rsub = "" + (extraArgs[k + 1] || "")
                else if (extraArgs[k] === "--type") rtype = "" + (extraArgs[k + 1] || "all")
            }
            if (!repo.length) { loading = false; error = I18n.tr("no repo"); return }
            _repoRepo = repo; _repoSub = rsub; _repoType = rtype
            var fetch =
                "ua=" + JSON.stringify(_ua) + "; repo=" + JSON.stringify(repo) + "; br=" + JSON.stringify(rbranch) + "; "
                + "tok=\"${GITHUB_TOKEN:-$(gh auth token 2>/dev/null || true)}\"; "
                + "auth=(); [ -n \"$tok\" ] && auth=(-H \"Authorization: Bearer $tok\"); "
                + "[ -z \"$br\" ] && br=$(curl -fsSL -A \"$ua\" \"${auth[@]}\" \"https://api.github.com/repos/$repo\" 2>/dev/null | jq -r '.default_branch // \"main\"'); "
                + "tree=$(curl -fsSL -A \"$ua\" \"${auth[@]}\" \"https://api.github.com/repos/$repo/git/trees/$br?recursive=1\" 2>/dev/null); "
                + "regp=$(printf '%s' \"$tree\" | jq -r '[.tree[].path | select(endswith(\"livewalls/registry.json\") or . == \"registry.json\")][0] // \"\"' 2>/dev/null); "
                + "printf 'BRANCH %s\\n' \"$br\"; "
                + "if [ -n \"$regp\" ]; then printf 'REGISTRY\\n'; curl -fsSL -A \"$ua\" \"https://raw.githubusercontent.com/$repo/$br/$regp\" 2>/dev/null; else printf 'TREE\\n'; printf '%s' \"$tree\"; fi"
            _nativeSearchProc.command = ["bash", "-lc", fetch]
            _nativeSearchProc.running = true
            return
        }
        loading = false
        error = I18n.tr("unknown source")
    }

    function nextPage() {
        if (loading || !hasMore) return
        search(_pendingQuery, currentPage + 1)
    }
    function prevPage() {
        if (loading || currentPage <= 1) return
        search(_pendingQuery, currentPage - 1)
    }

    // native ryostore: fetch the curated registry.json and reshape it into the
    // same rows the binary emitted, so the Browse surface is unchanged.
    property var _nativeSearchProc: Process {
        stdout: SplitParser { splitMarker: ""; onRead: function(data) { src._buf += data } }
        onExited: function(code) {
            src.loading = false
            if (code !== 0) { src.error = I18n.tr("search failed"); src.results = []; return }
            var out = []
            try {
                if (src._nativeProvider === "ryostore") {
                    var reg = JSON.parse(src._buf)
                    var q = ("" + src._pendingQuery).toLowerCase()
                    var ws = reg.wallpapers || []
                    for (var i = 0; i < ws.length; i++) {
                        var w = ws[i]
                        var hit = q === "" || ("" + (w.name || "")).toLowerCase().indexOf(q) >= 0
                        var tags = w.tags || []
                        for (var t = 0; !hit && t < tags.length; t++)
                            if (("" + tags[t]).toLowerCase().indexOf(q) >= 0) hit = true
                        if (!hit) continue
                        out.push({ id: w.id, thumb: src._ryostoreBase + "/" + w.poster,
                                   video: w.video, dl: w.video, resolution: "",
                                   name: w.name, author: (w.author || "") })
                    }
                }
                else if (src._nativeProvider === "motionbgs") {
                    var re = new RegExp("/i/c/[0-9]+x[0-9]+/media/[0-9]+/[^\"' ]+\\.jpe?g", "g")
                    var seen = {}, m
                    while ((m = re.exec(src._buf)) !== null) {
                        var p = m[0]
                        var parts = p.split("/")           // ['','i','c',WxH,'media',id,fname]
                        var wxh = parts[3], mid = parts[5], fname = parts[6]
                        if (parseInt(wxh.split("x")[0]) < 300) continue
                        if (seen[mid]) continue
                        seen[mid] = true
                        var mbase = ("" + fname).replace(/\.[0-9]+x[0-9]+/, "").replace(/\.jpe?g$/, "")
                        var rmt = ("" + fname).match(/\.([0-9]+x[0-9]+)\.jpe?g$/)
                        out.push({ id: mid, thumb: src._mbBase + p,
                                   video: src._mbBase + "/dl/hd/" + mid + "/",
                                   dl: src._mbBase + "/dl/4k/" + mid + "/",
                                   resolution: rmt ? rmt[1] : "", name: mbase.replace(/-/g, " ") })
                        if (out.length >= 24) break
                    }
                }
                else if (src._nativeProvider === "moewalls") {
                    var cards = src._buf.split("<article ")
                    for (var ci = 0; ci < cards.length; ci++) {
                        var card = cards[ci]
                        if (card.indexOf("g1-frame") < 0) continue
                        var msrc = card.match(new RegExp('src="(https://moewalls\\.com/wp-content/uploads/[0-9]{4}/[0-9]{2}/[^"]*-thumb-[0-9]+x[0-9]+\\.(?:jpe?g|png))"'))
                        if (!msrc) continue
                        var srcUrl = msrc[1]
                        var mhref = card.match(new RegExp('class="g1-frame" href="(https://moewalls\\.com/[^"]*)"'))
                        var mtitle = card.match(new RegExp('<a title="([^"]*)" class="g1-frame"'))
                        var mwres = card.match(/resolutions-([0-9]+x[0-9]+)/)
                        var rel = srcUrl.split("/uploads/")[1]
                        var yy = rel.split("/")[0]
                        var rp = rel.split("/")
                        var mb2 = rp[rp.length - 1].split("-thumb-")[0]
                        var webm = src._mwBase + "/wp-content/uploads/preview/" + yy + "/" + mb2 + "-preview.webm"
                        var nm = mtitle ? mtitle[1].replace(/ Live Wallpaper$/, "") : mb2.replace(/-/g, " ")
                        out.push({ id: mb2,
                                   thumb: "https://wsrv.nl/?url=" + encodeURIComponent(srcUrl),
                                   video: webm, dl: webm,
                                   resolution: mwres ? mwres[1] : "",
                                   name: nm, moewalls_url: (mhref ? mhref[1] : "") })
                        if (out.length >= 24) break
                    }
                }
                else if (src._nativeProvider === "repos") {
                    var nl = src._buf.indexOf("\n")
                    var br = src._buf.slice(0, nl).replace(/^BRANCH /, "")
                    var rest = src._buf.slice(nl + 1)
                    var nl2 = rest.indexOf("\n")
                    var marker = rest.slice(0, nl2)
                    var body = rest.slice(nl2 + 1)
                    var rawb = "https://raw.githubusercontent.com/" + src._repoRepo + "/" + br
                    var mediab = "https://media.githubusercontent.com/media/" + src._repoRepo + "/" + br
                    var rq = ("" + src._pendingQuery).toLowerCase()
                    var tproxy = function(u) { return u ? ("https://wsrv.nl/?url=" + u.replace(/^https?:\/\//, "") + "&w=480&output=webp&q=80") : "" }
                    var enc = function(pp) { return pp.split("/").map(encodeURIComponent).join("/") }
                    if (marker === "REGISTRY") {
                        var abs = function(pp) { return /^https?:\/\//.test(pp) ? pp : (rawb + "/" + pp) }
                        var rws = (JSON.parse(body).wallpapers) || []
                        for (var ri = 0; ri < rws.length; ri++) {
                            var rw = rws[ri]
                            if (src._repoType === "images") continue
                            var rhit = rq === "" || ("" + (rw.name || "")).toLowerCase().indexOf(rq) >= 0
                            var rtags = rw.tags || []
                            for (var rt = 0; !rhit && rt < rtags.length; rt++) if (("" + rtags[rt]).toLowerCase().indexOf(rq) >= 0) rhit = true
                            if (!rhit) continue
                            out.push({ id: rw.id, kind: "video", thumb: tproxy(abs(rw.poster)),
                                       video: abs(rw.video), dl: abs(rw.video), resolution: "", name: rw.name })
                        }
                    } else {
                        var tree = (JSON.parse(body).tree) || []
                        var lfs = {}
                        for (var li = 0; li < tree.length; li++) { var te = tree[li]; if (te.type === "blob" && (te.size || 1e9) < 1024) lfs[te.path] = true }
                        var host = function(pp) { return lfs[pp] ? mediab : rawb }
                        var dirOf = function(pp) { return pp.replace(/\/[^/]+$/, "") }
                        var humanize = function(s) { return s.replace(/-[0-9]+$/, "").replace(/[-_]/g, " ") }
                        var sub = src._repoSub || ""
                        var scoped = []
                        for (var si = 0; si < tree.length; si++) { var sp = tree[si].path; if (sp && (sub === "" || sp.indexOf(sub) === 0)) scoped.push(sp) }
                        var vids = scoped.filter(function(pp) { return /\.(mp4|webm|mkv|mov)$/i.test(pp) })
                        var imgs = scoped.filter(function(pp) { return /\.(jpe?g|png|webp)$/i.test(pp) })
                        var viddirs = {}; vids.forEach(function(pp) { viddirs[dirOf(pp)] = true })
                        var vpd = {}; vids.forEach(function(pp) { var d = dirOf(pp); vpd[d] = (vpd[d] || 0) + 1 })
                        var rows = []
                        imgs.forEach(function(pp) {
                            if (viddirs[dirOf(pp)]) return
                            if (/thumb|poster|cover/i.test(pp)) return
                            rows.push({ kind: "image", path: pp, thumb: pp, media: pp,
                                        name: humanize(pp.replace(/^.*\//, "").replace(/\.[^.]+$/, "")) })
                        })
                        vids.forEach(function(pp) {
                            var d = dirOf(pp)
                            var di = imgs.filter(function(x) { return x.indexOf(d + "/") === 0 })
                            var poster = di.filter(function(x) { return /thumb|poster|cover/i.test(x) })[0] || di[0] || ""
                            var nm2 = (vpd[d] === 1) ? d.replace(/^.*\//, "") : pp.replace(/^.*\//, "").replace(/\.[^.]+$/, "")
                            rows.push({ kind: "video", path: pp, thumb: poster, media: pp, name: humanize(nm2) })
                        })
                        rows = rows.filter(function(r) { return src._repoType === "all" || (src._repoType === "live" && r.kind === "video") || (src._repoType === "images" && r.kind === "image") })
                        rows = rows.filter(function(r) { return rq === "" || r.path.toLowerCase().indexOf(rq) >= 0 })
                        for (var fi = 0; fi < rows.length; fi++) {
                            var rr = rows[fi]
                            var turl = rr.thumb === "" ? "" : (host(rr.thumb) + "/" + enc(rr.thumb))
                            var murl = host(rr.media) + "/" + enc(rr.media)
                            out.push({ id: rr.path, kind: rr.kind, thumb: tproxy(turl), large: turl,
                                       video: (rr.kind === "video" ? murl : ""), dl: murl, resolution: "", name: rr.name })
                        }
                    }
                }
            } catch (e) { src.error = I18n.tr("search failed"); src.results = []; return }
            if (src._nativeProvider === "ryostore" || src._nativeProvider === "repos") {
                src._fullList = out
                src._applyClientPage()
            } else {
                src.lastPage = out.length > 0 ? src.currentPage + 1 : src.currentPage
                src.error = out.length === 0 ? I18n.tr("no results") : ""
                src.results = out
            }
        }
    }

    property var _nativeDlProc: Process {
        stdout: SplitParser { splitMarker: ""; onRead: function(data) { src._nativeDlBuf += data } }
        onExited: function(code) {
            var p = src._nativeDlBuf.trim()
            src.downloadingId = ""
            if (code === 0 && p.length > 0) src.applied(p)
            else src.failed(I18n.tr("download failed"))
        }
    }

    function download(item) {
        if (!downloadVerb || !item || downloadingId.length > 0) return
        _nativeDlBuf = ""
        downloadingId = "" + item.id
        if (downloadVerb === "extras-download") {
            var url = "" + (item.dl || item.video || "")
            var ext = url.split(".").pop()
            if (!ext || ext.length > 5) ext = "mp4"
            var dir = Quickshell.env("HOME") + "/Pictures/livewalls"
            var xout = dir + "/ryoku-" + item.id + "." + ext
            _nativeDlProc.command = ["bash", "-lc",
                "mkdir -p " + JSON.stringify(dir) + " && curl -fsSL -A " + JSON.stringify(_ua)
                + " " + JSON.stringify(url) + " -o " + JSON.stringify(xout)
                + " && printf '%s' " + JSON.stringify(xout)]
            _nativeDlProc.running = true
            return
        }
        if (downloadVerb === "motionbgs-download") {
            var mUrl = "" + (item.dl || item.video || "")
            var mDir = Quickshell.env("HOME") + "/Pictures/livewalls"
            var mOut = mDir + "/motionbgs-" + item.id + ".mp4"
            _nativeDlProc.command = ["bash", "-lc",
                "mkdir -p " + JSON.stringify(mDir) + " && curl -fsSL -A " + JSON.stringify(_ua)
                + " -e " + JSON.stringify(_mbBase + "/") + " " + JSON.stringify(mUrl)
                + " -o " + JSON.stringify(mOut) + " && printf '%s' " + JSON.stringify(mOut)]
            _nativeDlProc.running = true
            return
        }
        if (downloadVerb === "moewalls-download") {
            var moeUrl = "" + (item.dl || item.video || "")
            var post = "" + (item.moewalls_url || "")
            var moeDir = Quickshell.env("HOME") + "/Pictures/livewalls"
            var mp4 = moeDir + "/moewalls-" + item.id + ".mp4"
            var wext = moeUrl.split(".").pop(); if (!wext || wext.length > 5) wext = "webm"
            var wf = moeDir + "/moewalls-" + item.id + "." + wext
            var sh = "mkdir -p " + JSON.stringify(moeDir) + "; "
                + "ua=" + JSON.stringify(_ua) + "; post=" + JSON.stringify(post) + "; "
                + "url=" + JSON.stringify(moeUrl) + "; mp4=" + JSON.stringify(mp4) + "; wf=" + JSON.stringify(wf) + "; "
                + "tok=$(curl -fsSL -A \"$ua\" \"$post\" 2>/dev/null | grep -oE '<a[^>]*id=\"moe-download\"[^>]*>' | grep -oE 'data-url=\"[^\"]*\"' | sed -E 's/data-url=\"([^\"]*)\"/\\1/' | head -1); "
                + "if [ -n \"$tok\" ] && curl -fsSL -A \"$ua\" -e \"$post\" \"https://go.moewalls.com/download.php?video=$tok\" -o \"$mp4\" 2>/dev/null; then printf '%s' \"$mp4\"; exit 0; fi; "
                + "if curl -fsSL -A \"$ua\" " + JSON.stringify(moeUrl) + " -o \"$wf\" 2>/dev/null; then printf '%s' \"$wf\"; exit 0; fi; exit 1"
            _nativeDlProc.command = ["bash", "-lc", sh]
            _nativeDlProc.running = true
            return
        }
        if (downloadVerb === "library-download") {
            var lUrl = "" + (item.dl || item.video || item.large || "")
            var lHome = Quickshell.env("HOME")
            var lsh = "url=" + JSON.stringify(lUrl) + "; ua=" + JSON.stringify(_ua) + "; home=" + JSON.stringify(lHome) + "; "
                + "ext=\"${url##*.}\"; ext=\"${ext%%\\?*}\"; "
                + "pics=\"${XDG_PICTURES_DIR:-$home/Pictures}\"; "
                + "case \"$ext\" in mp4|webm|mkv|mov) dest=\"$pics/livewalls\";; jpg|jpeg|png|webp) dest=\"$pics/Wallpapers\";; *) ext=mp4; dest=\"$pics/livewalls\";; esac; "
                + "mkdir -p \"$dest\"; out=\"$dest/lib-$(printf '%s' \"$url\" | md5sum | cut -c1-12).$ext\"; "
                + "curl -fsSL -A \"$ua\" \"$url\" -o \"$out\" && printf '%s' \"$out\""
            _nativeDlProc.command = ["bash", "-lc", lsh]
            _nativeDlProc.running = true
            return
        }
        downloadingId = ""
        failed(I18n.tr("unknown source"))
    }
}
