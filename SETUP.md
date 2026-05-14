# 🔧 Setup Guide for Twitter Feed Cleaner

This guide walks you through getting the required authentication tokens from Twitter/X.

## Overview

The bot needs **5 pieces of information** to work:

```
┌─────────────────────────────────────────────────────────────┐
│  1. User ID         ───── from twid cookie                 │
│  2. ct0 (CSRF)      ───── from browser cookie               │
│  3. guest_id        ───── from browser cookie               │
│  4. Query IDs       ───── from page source  ← auto-extracted │
│  5. auth_token      ───── from DevTools     ← manual step  │
└─────────────────────────────────────────────────────────────┘
           ↑                         ↑
    extract.js gets these    You copy this manually
```

> **Why manual for auth_token?** It's an **HttpOnly cookie** - browsers intentionally hide it from JavaScript for security. You must copy it manually from DevTools.

---

## Step 1: Run extract.js in Browser

### 1.1 Open Twitter
- Go to **x.com** and **login** to your account
- Make sure you're on the main feed (twitter.com/home)

### 1.2 Open DevTools Console
```
┌──────────────────────────────────────────────────┐
│  Windows/Linux:  Press F12  or  Ctrl+Shift+J     │
│  Mac:             Press F12  or  Cmd+Option+J    │
└──────────────────────────────────────────────────┘
```

### 1.3 Run the Script
1. Open the file `extract.js` from this project folder
2. Copy **all** the code (Ctrl+A, Ctrl+C)
3. Paste into the DevTools Console
4. Press **Enter**

### 1.4 Copy the Output
You'll see something like:
```
========================================
COPY THIS LINE AND PASTE INTO BOT:
123456789|a1b2c3d4...|v1%3A123...|36rb3Xj3...|ZyZigVsN...
========================================
✅ Automatically copied to clipboard!
```

**Copy this line** (it's also auto-copied to your clipboard).

---

## Step 2: Get auth_token (Manual Step)

This is the **most important** step. Follow carefully:

### 2.1 Switch to Application Tab
```
┌────────────────────────────────────────────┐
│  Console | Elements | Network | Sources |   │
│                              ↓             │
│  >>> Application <<<  |  Performance        │
└────────────────────────────────────────────┘
                 ↑
         Click here
```

### 2.2 Navigate to Cookies
```
┌────────────────────────────────────────────────────┐
│  Storage                                            │
│  ├── ▶ Local Storage                                │
│  ├── ▶ Session Storage                            │
│  ├── ▷ Cookies                                     │
│  │      └── ▶ https://x.com  ←─── CLICK THIS      │
└────────────────────────────────────────────────────┘
```

### 2.3 Find and Copy auth_token
You'll see a table. Look for the row:

```
┌──────────────────────────────────────────────────────────────┐
│  Name           │  Value                                      │
├──────────────────────────────────────────────────────────────┤
│  ct0            │  a1b2c3d4e5f6...                            │
│  guest_id       │  v1%3A123456789                            │
│  twid           │  u%3D1234567890123456789                   │
│  ...            │  ...                                        │
│  🔍 auth_token  │  🔑 c7bb02a0513ffae...d1b2c3 ←─ COPY THIS   │
└──────────────────────────────────────────────────────────────┘
       ↑                              ↑
   Find this row              Double-click, Ctrl+C
```

**Steps:**
1. Find the row with `auth_token` in the Name column
2. **Double-click** the Value cell
3. Press **Ctrl+C** (or Cmd+C on Mac) to copy
4. **Important:** The value is long (like `c7bb02a0513ffae...`) - copy it fully

### 2.4 Get kdt (Optional but Recommended)

Same process as auth_token:
1. In the same cookies table, find `kdt`
2. Double-click Value, copy it
3. This adds extra session stability

---

## Step 3: Run the Bot

```bash
cd ~/Docs/project/deleter
go build -o deleter .
./deleter
```

The bot will guide you through:
1. **Paste** the line from extract.js
2. **Paste** auth_token (when prompted)
3. **Paste** kdt (optional)
4. **Review** and save

### Navigation Keys
| Key | Action |
|-----|--------|
| **ENTER** | Continue to next step |
| **ESC** | Go back to fix mistakes |

---

## 📁 What Gets Saved

After setup, `.session.json` is created in your project folder:

```json
{
  "user_id": "1234567890123456789",
  "cookies": {
    "auth_token": "c7bb02a0513ffaedf11979e0bdce0ec6ca63c9ba",
    "ct0": "f9f84fd66f441e7b4b48b47e415c7e4b...",
    "twid": "u%3D1538858660801269763",
    "kdt": "uuvxTYKxBRz95qW4AAaBRadJC7BDDE0X...",
    "guest_id": "v1%3A176565822044635355"
  },
  "query_id_user_tweets": "36rb3Xj3iJ64Q-9wKDjCcQ",
  "query_id_delete_retweet": "ZyZigVsNiFO6v1dEks1eWg",
  "created_at": "2024-01-15T10:30:00Z"
}
```

⚠️ **Security:**
- Valid for **14 days** (then you re-run setup)
- File is **gitignored** — won't be committed
- Keep it safe — contains your login tokens!

---

---

## 🚨 Troubleshooting

### "Session expired" or 401 error
```bash
rm .session.json
./deleter  # Re-run setup wizard
```

### Can't find Application tab
Use **Network tab** alternative:
1. DevTools → Network tab
2. Refresh page (F5)
3. Click any request to `/graphql/`
4. Scroll to Request Headers → find `cookie:`
5. Copy `auth_token=...` value

### auth_token is empty when I paste
You must be **logged in** to x.com. Check that you see your feed, not login page.

### "Invalid session data"
Press **ESC** in the bot to go back and re-enter the value. Make sure you copied the full auth_token (it's long!).

---

## 📝 Alternative: Manual .env Setup

If you prefer not using the wizard:

```bash
cp .env.example .env
# Edit .env with your tokens
```

The bot will load from `.env` on first run and auto-save to `.session.json`.

---

## 💡 Quick Commands

```bash
# Build
go build -o deleter .

# Run (with wizard if no session)
./deleter

# If you need to reset session, just delete it:
rm .session.json
./deleter  # Will start setup wizard again
```
