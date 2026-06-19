"""
Test a static HTML file directly (no server needed).
Useful for testing the Flutter web build output or any static HTML artifact.
"""

import os
from playwright.sync_api import sync_playwright

HTML_PATH = "/path/to/your/file.html"
file_url = f"file://{os.path.abspath(HTML_PATH)}"

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1920, "height": 1080})

    page.goto(file_url)
    page.wait_for_load_state("networkidle")

    # Screenshot before interaction
    page.screenshot(path="/tmp/before.png", full_page=True)
    print("Screenshot before: /tmp/before.png")

    # Example: fill a form field and submit
    # page.get_by_label("Email").fill("test@example.com")
    # page.get_by_role("button", name="Submit").click()
    # page.wait_for_load_state("networkidle")

    # Screenshot after interaction
    page.screenshot(path="/tmp/after.png", full_page=True)
    print("Screenshot after: /tmp/after.png")

    browser.close()
