pragma Singleton

import Quickshell

Singleton {
    // Opens the Hub on a page (a Hub section key such as "shellstyle"), or on its
    // home page without one. The Caelestia settings that the Nexus app held live on
    // the Hub's shellbar, shellpanels and shellstyle pages.
    function open(page): void {
        Quickshell.execDetached(["ryoku-shell", "hub", "open"].concat(page ? [String(page)] : []));
    }
    // The bar's context cards carry the number of the Nexus page they opened in the
    // Caelestia shell (3 is the launcher, the rest are parts of the bar). The Hub has a
    // page for the launcher and one for the Caelestia bar.
    function openPage(subPageIdx: int): void {
        Quickshell.execDetached(["ryoku-shell", "hub", "open", subPageIdx === 3 ? "launcher" : "shellbar"]);
    }
}
