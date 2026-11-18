function clamp(value, low, high) {
    return Math.max(low, Math.min(Math.max(low, high), value));
}

function beside(rect, width, height, area, gap) {
    var right = area.x + area.width;
    var bottom = area.y + area.height;
    var x = rect.x + rect.width + gap;
    var y = rect.y;
    if (x + width > right) {
        if (rect.x - gap - width >= area.x) {
            x = rect.x - gap - width;
        } else if (rect.y + rect.height + gap + height <= bottom) {
            x = rect.x;
            y = rect.y + rect.height + gap;
        } else {
            x = rect.x;
            y = rect.y - height - gap;
        }
    }
    return { x: clamp(x, area.x, right - width), y: clamp(y, area.y, bottom - height) };
}

function lens(rect, width, height, area, gap, preferredHeight) {
    const right = area.x + area.width, bottom = area.y + area.height;
    var x, anchor, bottomAnchored = false, side;
    if (rect.x + rect.width + gap + width <= right) {
        x = rect.x + rect.width + gap;
        side = "right";
    } else if (rect.x - gap - width >= area.x) {
        x = rect.x - gap - width;
        side = "left";
    } else if (rect.y + rect.height + gap + height <= bottom) {
        x = rect.x;
        anchor = rect.y + rect.height + gap;
        side = "below";
    } else if (rect.y - gap - height >= area.y) {
        x = rect.x;
        anchor = rect.y - gap;
        side = "above";
        bottomAnchored = true;
    } else {
        const fallback = beside(rect, width, height, area, gap);
        x = fallback.x;
        anchor = fallback.y;
        side = "right";
    }
    if (anchor === undefined) {
        bottomAnchored = rect.y + preferredHeight > bottom
            && rect.y + rect.height - area.y > bottom - rect.y;
        anchor = bottomAnchored
            ? clamp(rect.y + rect.height, area.y + height, bottom)
            : clamp(rect.y, area.y, bottom - height);
    }
    x = clamp(x, area.x, right - width);
    anchor = bottomAnchored ? clamp(anchor, area.y + height, bottom) : clamp(anchor, area.y, bottom - height);
    return { x: x, y: bottomAnchored ? anchor - height : anchor, anchor: anchor,
        bottom: bottomAnchored, side: side, room: bottomAnchored ? anchor - area.y : bottom - anchor };
}


function mergeLines(text) {
    return String(text || "").split(/\n\s*\n/).map(function(paragraph) {
        return paragraph.replace(/\s*\n\s*/g, " ");
    }).join("\n\n");
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { clamp: clamp, beside: beside, lens: lens, mergeLines: mergeLines };
