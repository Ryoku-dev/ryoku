pragma Singleton

import Quickshell

Singleton {
    id: root

    function pixelAlign(containerSize: real, contentSize: real): real {
        const center = (containerSize - contentSize) / 2;
        return Math.round(center) - center;
    }
}
