# Web Application Testing Skill

Use this skill to test the Notifyx Flutter web dashboard or any local web application.

## When to use this skill

- Verifying Flutter dashboard screens work correctly after building them
- Testing API integration from the dashboard (send notification, fetch history, etc.)
- Debugging UI issues by capturing screenshots
- Checking browser console logs for errors

## Decision Tree

```
Is the app a static HTML file?
  YES → Read the HTML file directly to find selectors, then use Playwright
  NO (Flutter Web / dynamic app) →
    Is the server already running?
      YES → Use element_discovery.py pattern to inspect the page
      NO  → Use scripts/with_server.py to start the server first
```

## Core Pattern: Reconnaissance then Action

Always do this before interacting with any page:
1. Navigate to the URL
2. Wait for `page.wait_for_load_state('networkidle')`
3. Take a screenshot to see current state
4. Inspect DOM for selectors
5. Then perform actions

## Starting Servers

```bash
# Start Flutter web dashboard + Go backend together
python skills/webapp-testing/scripts/with_server.py \
  --server "flutter run -d web-server --web-port 8081" --port 8081 \
  --server "go run ./cmd/server" --port 8080 \
  -- python your_test.py
```

## Key Rules

- Always launch Chromium in **headless mode**
- Always use `page.wait_for_load_state('networkidle')` before inspecting dynamic pages
- Treat `scripts/with_server.py` as a black box — run with `--help` first
- Save screenshots to `/tmp/` for quick inspection
- Close browser when done

## Flutter Web Notes

Flutter Web renders to a canvas — standard CSS selectors may not work. Prefer:
- `page.get_by_text("Send Notification")` over CSS class selectors
- `page.get_by_role("button", name="Send")` for buttons
- Screenshot-first approach to understand what's rendered
