pragma Singleton

import Quickshell

Singleton {
    // Formats
    readonly property list<string> validImageTypes: ["jpeg", "png", "webp", "tiff", "svg", "gif"]
    readonly property list<string> validImageExtensions: ["jpg", "jpeg", "png", "webp", "tif", "tiff", "svg", "gif"]
    readonly property list<string> validVideoExtensions: ["mp4", "webm", "mkv", "avi", "mov"]

    function isValidImageByName(name: string): bool {
        const lowerName = name.toLowerCase();
        return validImageExtensions.some(t => lowerName.endsWith(`.${t}`));
    }

    function isValidVideoByName(name: string): bool {
        const lowerName = name.toLowerCase();
        return validVideoExtensions.some(t => lowerName.endsWith(`.${t}`));
    }

    function isValidMediaByName(name: string): bool {
        return isValidImageByName(name) || isValidVideoByName(name);
    }

    // Thumbnails
    // https://specifications.freedesktop.org/thumbnail-spec/latest/directory.html
    readonly property var thumbnailSizes: ({
        "normal": 128,
        "large": 256,
        "x-large": 512,
        "xx-large": 1024
    })
    function thumbnailSizeNameForDimensions(width: int, height: int): string {
        const sizeNames = Object.keys(thumbnailSizes);
        for(let i = 0; i < sizeNames.length; i++) {
            const sizeName = sizeNames[i];
            const maxSize = thumbnailSizes[sizeName];
            if (width <= maxSize && height <= maxSize) return sizeName;
        }
        return "xx-large";
    }
}
