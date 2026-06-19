"""
Capture browser console logs while interacting with the dashboard.
Useful for debugging Flutter web errors or API call failures.
"""

from playwright.sync_api import sync_playwright

URL = "http://localhost:8081"
LOG_FILE = "/tmp/console.log"

messages = []

def handle_console(msg):
    entry = f"[{msg.type}] {msg.text}"
    messages.append(entry)
    print(entry)

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1280, "height": 800})
    page.on("console", handle_console)

    page.goto(URL)
    page.wait_for_load_state("networkidle")

    # Perform any interaction here, e.g.:
    # page.get_by_text("Send Notification").click()
    # page.wait_for_load_state("networkidle")

    with open(LOG_FILE, "w") as f:
        f.write("\n".join(messages))

    print(f"\nTotal console messages: {len(messages)}")
    print(f"Log saved to {LOG_FILE}")

    browser.close()
