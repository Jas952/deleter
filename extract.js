// Twitter Session Extractor - Output for DELETER Bot
// Run this in DevTools Console on x.com
// Copy the output and paste into the bot
//
// HOW TO USE:
// 1. Open x.com and login
// 2. Press F12 → Console
// 3. Copy this entire script
// 4. Paste, press Enter
// 5. Copy the output line (auto-copied to clipboard)
// 6. Paste into the bot's prompt

(function() {
    const data = {
        user_id: '',
        ct0: '',
        twid: '',
        guest_id: '',
        query_id_user_tweets: '36rb3Xj3iJ64Q-9wKDjCcQ',
        query_id_delete_retweet: 'ZyZigVsNiFO6v1dEks1eWg'
    };

    // Get cookies
    document.cookie.split(';').forEach(c => {
        const [key, val] = c.trim().split('=');
        if (key === 'ct0') data.ct0 = decodeURIComponent(val);
        if (key === 'twid') data.twid = decodeURIComponent(val);
        if (key.startsWith('guest_id')) data.guest_id = decodeURIComponent(val);
    });

    // Extract user ID from twid
    if (data.twid) {
        const match = data.twid.match(/(\d+)/);
        if (match) data.user_id = match[1];
    }

    // Try to find query IDs in page
    const html = document.documentElement.innerHTML;
    const utMatch = html.match(/"(\w{22})","operationName":"UserTweets"/);
    const drMatch = html.match(/"(\w{22})","operationName":"DeleteRetweet"/);
    if (utMatch) data.query_id_user_tweets = utMatch[1];
    if (drMatch) data.query_id_delete_retweet = drMatch[1];

    // Output format for bot: user_id|ct0|guest_id|query_id_user_tweets|query_id_delete_retweet
    const output = `${data.user_id}|${data.ct0}|${data.guest_id}|${data.query_id_user_tweets}|${data.query_id_delete_retweet}`;
    
    console.log('%c========================================', 'color: #4ECDC4; font-size: 18px; font-weight: bold;');
    console.log('%cCOPY THIS LINE AND PASTE INTO BOT:', 'color: #4ECDC4; font-size: 14px;');
    console.log('%c' + output, 'color: #2ECC71; font-size: 16px; background: #1a1a2e; padding: 15px; border-radius: 8px; display: block; margin: 10px 0;');
    console.log('%c========================================', 'color: #4ECDC4; font-size: 18px; font-weight: bold;');
    
    // Also copy to clipboard
    navigator.clipboard.writeText(output).then(() => {
        console.log('%c✅ Automatically copied to clipboard!', 'color: #2ECC71; font-size: 14px;');
        console.log('%cJust paste into the bot (Ctrl+V / Cmd+V)', 'color: #888; font-size: 12px;');
    }).catch(() => {
        console.log('%c⚠️ Could not auto-copy. Please manually copy the green line above.', 'color: #FFE66D;');
    });
})();
