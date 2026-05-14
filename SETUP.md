# Quick Setup Guide

The bot now has an **interactive setup wizard** that guides you through the entire process with built-in instructions!

## First Run - Interactive Setup

When you run `./deleter` without a saved session, the bot will automatically start the setup wizard:

```bash
cd ~/Docs/project/deleter
./deleter
```

You'll see a guided 5-step process:

### Step 1: Extract Data from Browser

The bot shows instructions:
1. Open **x.com** in your browser and login
2. Press **F12** → **Console** tab
3. Copy contents of `extract.js` file (located in project folder)
4. Paste into console and press Enter
5. Copy the output line (format: `user_id|ct0|guest_id|query_id_user|query_id_delete`)
6. Return to bot and press **ENTER**

### Step 2: Paste Extracted Data

Paste the line copied from browser console. The bot automatically parses:
- ✅ User ID
- ✅ ct0 (CSRF token)
- ✅ guest_id
- ✅ Query IDs

### Step 3: Enter auth_token (Manual)

The bot shows detailed instructions with mini-guide:

**How to get auth_token:**
1. In DevTools, click **Application** tab
2. In left sidebar: **Storage** → **Cookies** → `https://x.com`
3. Find **"auth_token"** in the table
4. Double-click the **Value** cell and copy it
5. Return to bot and paste it
6. Press **ENTER**

> **Why manual?** `auth_token` is an HttpOnly cookie - browsers hide it from JavaScript for security.

### Step 4: Enter kdt (Optional)

Same process as auth_token, but optional. The bot shows instructions again.

### Step 5: Verify and Save

Review all data displayed by the bot:
- User ID
- Tokens (truncated for security)
- Query IDs

Press **ENTER** to save, or **ESC** to go back and correct.

---

## The extract.js Script

Located at `extract.js` in your project folder. It extracts from browser:
- User ID (from twid cookie)
- ct0 (CSRF token)
- guest_id
- Query IDs (from page source)

**Output format:** `user_id|ct0|guest_id|query_id_user_tweets|query_id_delete_retweet`

---

## What Gets Saved

After setup, `.session.json` is created with:
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
  "headers": { ... },
  "query_id_user_tweets": "36rb3Xj3iJ64Q-9wKDjCcQ",
  "query_id_delete_retweet": "ZyZigVsNiFO6v1dEks1eWg",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Security:** 
- File is saved locally in your project folder
- Valid for **14 days**
- Added to `.gitignore` — never committed
- Contains sensitive data, keep it secure!

---

## Navigation During Setup

| Key | Action |
|-----|--------|
| **ENTER** | Continue to next step |
| **ESC** | Go back to previous step |
| **Q** | Quit (on first screen) |

At any step you can press **ESC** to return and correct data.

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Session expired" | Re-run `./deleter` — it will guide you through setup again |
| 401 error | Session expired, redo setup |
| Can't find Application tab | Use Network tab → Request Headers → cookie |
| auth_token empty | You must be logged into x.com |

---

## Alternative: Using .env File

If you prefer manual setup, create `.env` file:

```bash
cp .env.example .env
```

Fill in manually. Program will load from `.env` and auto-save to `.session.json` for future runs.

---

## Quick Commands

```bash
# Build
go build -o deleter .

# Run (with wizard if no session)
./deleter

# If you need to reset session, just delete it:
rm .session.json
./deleter  # Will start setup wizard again
```
