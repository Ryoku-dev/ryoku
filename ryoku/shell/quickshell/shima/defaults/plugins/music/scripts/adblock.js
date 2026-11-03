// YouTube Music AdBlock userscript for iNiR WebEngine
// Skips video ads, closes overlay ads, hides banner ads.
// Runs at document idle (after page load).

(function() {
    'use strict';

    const LOG_PREFIX = '[iNiR-AdBlock]';

    // --- CSS: hide ad containers ---
    const style = document.createElement('style');
    style.textContent = `
        /* Video ad overlays */
        .video-ads, .ytp-ad-module,
        .ytp-ad-overlay-container, .ytp-ad-text-overlay,
        .ytp-ad-skip-button-container,
        /* Banner ads */
        ytmusic-mealbar-promo-renderer,
        ytmusic-statement-banner-renderer,
        tp-yt-paper-dialog.ytmusic-popup-container,
        /* Premium upsell */
        ytmusic-enforced-premium-upsell-dialog-renderer,
        yt-mealbar-promo-renderer,
        /* Masthead ads */
        #masthead-ad, #player-ads,
        /* General ad containers */
        .ad-showing .ytp-ad-overlay-close-button,
        .ytd-merch-shelf-renderer,
        .ytd-action-companion-ad-renderer,
        .ytd-in-feed-ad-layout-renderer,
        .ytd-banner-promo-renderer,
        .ytd-statement-banner-renderer,
        .ytd-ad-slot-renderer,
        ytmusic-promoted-sparkles-web-renderer,
        /* Survey popups */
        .ytd-popup-container paper-dialog,
        /* Sidebar ads */
        .ytd-rich-item-renderer[is-ad] {
            display: none !important;
        }
    `;
    document.documentElement.appendChild(style);

    // --- Video ad skipper ---
    function skipAd() {
        // Check if an ad is playing
        const player = document.querySelector('.html5-video-player');
        if (!player) return;

        const isAdShowing = player.classList.contains('ad-showing');
        if (!isAdShowing) return;

        // Try to click skip button
        const skipBtn = document.querySelector(
            '.ytp-ad-skip-button, .ytp-ad-skip-button-modern, .ytp-skip-ad-button, ' +
            'button.ytp-ad-skip-button-text'
        );
        if (skipBtn) {
            skipBtn.click();
            console.log(LOG_PREFIX, 'Clicked skip button');
            return;
        }

        // No skip button — fast-forward the ad video
        const video = document.querySelector('video');
        if (video && video.duration && isFinite(video.duration)) {
            video.currentTime = video.duration;
            console.log(LOG_PREFIX, 'Fast-forwarded ad');
        }
    }

    // --- Close overlay/popup ads ---
    function closeOverlays() {
        // Close ad overlay dismiss buttons
        const closeButtons = document.querySelectorAll(
            '.ytp-ad-overlay-close-button, ' +
            '.ytp-ad-skip-button-icon, ' +
            'tp-yt-paper-dialog .yt-spec-button-shape-next--outline'
        );
        closeButtons.forEach(btn => {
            try { btn.click(); } catch(e) {}
        });

        // Dismiss premium upsell dialogs
        const premiumDismiss = document.querySelector(
            'ytmusic-enforced-premium-upsell-dialog-renderer tp-yt-paper-button'
        );
        if (premiumDismiss) {
            premiumDismiss.click();
            console.log(LOG_PREFIX, 'Dismissed premium upsell');
        }

        // Dismiss mealbar promos
        const mealbarDismiss = document.querySelector(
            'ytmusic-mealbar-promo-renderer #dismiss-button'
        );
        if (mealbarDismiss) {
            mealbarDismiss.click();
        }
    }

    // --- Mute ads if they can't be skipped ---
    function muteAdAudio() {
        const player = document.querySelector('.html5-video-player');
        if (!player) return;

        const video = document.querySelector('video');
        if (!video) return;

        if (player.classList.contains('ad-showing')) {
            if (!video.muted) {
                video.muted = true;
                video.dataset.inirMuted = 'true';
            }
        } else if (video.dataset.inirMuted === 'true') {
            video.muted = false;
            delete video.dataset.inirMuted;
        }
    }

    // --- Main loop (runs every 500ms) ---
    setInterval(() => {
        skipAd();
        closeOverlays();
        muteAdAudio();
    }, 500);

    // Also observe DOM mutations for faster ad detection
    const observer = new MutationObserver((mutations) => {
        for (const m of mutations) {
            if (m.type === 'attributes' && m.attributeName === 'class') {
                const el = m.target;
                if (el.classList && el.classList.contains('ad-showing')) {
                    skipAd();
                    muteAdAudio();
                }
            }
        }
    });

    // Start observing once video player exists
    function attachObserver() {
        const player = document.querySelector('.html5-video-player');
        if (player) {
            observer.observe(player, { attributes: true, attributeFilter: ['class'] });
            console.log(LOG_PREFIX, 'Observer attached to video player');
        } else {
            setTimeout(attachObserver, 1000);
        }
    }

    if (document.readyState === 'complete') {
        attachObserver();
    } else {
        window.addEventListener('load', attachObserver);
    }

    console.log(LOG_PREFIX, 'YouTube Music adblock loaded');
})();
