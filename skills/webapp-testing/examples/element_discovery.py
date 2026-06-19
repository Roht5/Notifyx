"""
Discover interactive elements on a page.
Use this first when you don't know the selectors — run it, read the output, then write targeted tests.
"""

from playwright.sync_api import sync_playwright

URL = "http://localhost:8081"  # Flutter dashboard default port

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1280, "height": 800})
    page.goto(URL)
    page.wait_for_load_state("networkidle")

    # Screenshot first — always
    page.screenshot(path="/tmp/page_discovery.png", full_page=True)
    print("Screenshot saved to /tmp/page_discovery.png")

    # Buttons
    print("\n--- Buttons ---")
    for btn in page.locator("button").all():
        text = btn.inner_text().strip() or "(hidden)"
        print(f"  button: {text!r}")

    # Links
    print("\n--- Links (first 5) ---")
    for link in page.locator("a").all()[:5]:
        text = link.inner_text().strip()
        href = link.get_attribute("href")
        print(f"  link: {text!r} → {href}")

    # Inputs
    print("\n--- Input fields ---")
    for field in page.locator("input, textarea, select").all():
        name = field.get_attribute("name") or field.get_attribute("id") or field.get_attribute("type") or "unknown"
        print(f"  input: {name!r}")

    browser.close()
