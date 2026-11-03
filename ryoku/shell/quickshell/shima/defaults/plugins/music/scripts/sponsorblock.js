// SponsorBlock userscript for iNiR WebEngine
// Skips sponsored segments in YouTube Music videos using the SponsorBlock API.
// Categories: sponsor, selfpromo, interaction, intro, outro, music_offtopic
// Runs at document idle, polls video time and skips matching segments.

(function() {
    'use strict';

    const LOG_PREFIX = '[iNiR-SponsorBlock]';
    const API_BASE = 'https://sponsor.ajay.app/api';

    // Categories to skip (user could customize later via config)
    const SKIP_CATEGORIES = [
        'sponsor',
        'selfpromo',
        'interaction',
        'intro',
        'outro',
        'music_offtopic'
    ];

    // Cache: videoId → segments array
    const segmentCache = {};

    // Track current video to detect changes
    let currentVideoId = null;
    let currentSegments = [];
    let skipNotificationTimeout = null;

    // --- Extract video ID from URL or player ---
    function getVideoId() {
        // YouTube Music uses /watch?v=VIDEO_ID
        const params = new URLSearchParams(window.location.search);
        const v = params.get('v');
        if (v) return v;

        // Fallback: check for video element src
        const video = document.querySelector('video');
        if (video && video.src) {
            const match = video.src.match(/[?&]v=([^&]+)/);
            if (match) return match[1];
        }

        return null;
    }

    // --- Fetch segments from SponsorBlock API ---
    async function fetchSegments(videoId) {
        if (segmentCache[videoId] !== undefined) {
            return segmentCache[videoId];
        }

        try {
            const cats = encodeURIComponent(JSON.stringify(SKIP_CATEGORIES));
            const url = `${API_BASE}/skipSegments?videoID=${videoId}&categories=${cats}`;

            const response = await fetch(url);

            if (response.status === 404) {
                // No segments for this video — cache empty
                segmentCache[videoId] = [];
                return [];
            }

            if (!response.ok) {
                console.warn(LOG_PREFIX, 'API error:', response.status);
                return [];
            }

            const data = await response.json();
            const segments = data.map(seg => ({
                start: seg.segment[0],
                end: seg.segment[1],
                category: seg.category,
                uuid: seg.UUID
            }));

            segmentCache[videoId] = segments;
            console.log(LOG_PREFIX, `Found ${segments.length} segments for ${videoId}`);
            return segments;

        } catch (e) {
            console.warn(LOG_PREFIX, 'Failed to fetch segments:', e);
            return [];
        }
    }

    // --- Show skip notification ---
    function showSkipNotification(category) {
        // Remove existing notification
        const existing = document.getElementById('inir-sb-notification');
        if (existing) existing.remove();

        const labels = {
            'sponsor': 'Sponsor',
            'selfpromo': 'Self-Promotion',
            'interaction': 'Interaction Reminder',
            'intro': 'Intro',
            'outro': 'Outro',
            'music_offtopic': 'Non-Music'
        };

        const div = document.createElement('div');
        div.id = 'inir-sb-notification';
        div.style.cssText = `
            position: fixed;
            bottom: 100px;
            right: 20px;
            background: rgba(0, 0, 0, 0.8);
            color: white;
            padding: 8px 16px;
            border-radius: 8px;
            font-size: 13px;
            font-family: sans-serif;
            z-index: 999999;
            pointer-events: none;
            transition: opacity 0.3s;
            opacity: 1;
        `;
        div.textContent = `⏭ Skipped: ${labels[category] || category}`;
        document.body.appendChild(div);

        if (skipNotificationTimeout) clearTimeout(skipNotificationTimeout);
        skipNotificationTimeout = setTimeout(() => {
            div.style.opacity = '0';
            setTimeout(() => div.remove(), 300);
        }, 2000);
    }

    // --- Main check loop ---
    async function checkAndSkip() {
        const videoId = getVideoId();
        if (!videoId) return;

        // Video changed — fetch new segments
        if (videoId !== currentVideoId) {
            currentVideoId = videoId;
            currentSegments = await fetchSegments(videoId);
        }

        if (currentSegments.length === 0) return;

        const video = document.querySelector('video');
        if (!video || video.paused) return;

        const currentTime = video.currentTime;

        for (const seg of currentSegments) {
            // Check if we're inside a segment (with small tolerance)
            if (currentTime >= seg.start && currentTime < seg.end - 0.5) {
                console.log(LOG_PREFIX, `Skipping ${seg.category} segment: ${seg.start.toFixed(1)}s → ${seg.end.toFixed(1)}s`);
                video.currentTime = seg.end;
                showSkipNotification(seg.category);
                break;  // Only skip one segment per check
            }
        }
    }

    // --- Poll every 500ms ---
    setInterval(checkAndSkip, 500);

    // --- Also detect navigation changes (YouTube Music is SPA) ---
    let lastUrl = location.href;
    const urlObserver = new MutationObserver(() => {
        if (location.href !== lastUrl) {
            lastUrl = location.href;
            currentVideoId = null;  // Force re-fetch on URL change
            currentSegments = [];
        }
    });

    function attachUrlObserver() {
        urlObserver.observe(document.body, { childList: true, subtree: true });
        console.log(LOG_PREFIX, 'URL change observer attached');
    }

    if (document.readyState === 'complete') {
        attachUrlObserver();
    } else {
        window.addEventListener('load', attachUrlObserver);
    }

    console.log(LOG_PREFIX, 'SponsorBlock loaded — categories:', SKIP_CATEGORIES.join(', '));
})();
