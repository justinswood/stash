#!/usr/bin/env python3
"""Drive the real Stash UI over the Chrome DevTools Protocol.

Why this exists
---------------
This app's rendered page is NOT its React source. `refract.js` builds and
inserts elements at runtime that exist nowhere in `ui/v2.5/src`, and theme
rules win or lose on specificity that grepping cannot show. A standalone
harness omits exactly the runtime mutation that causes the bug, so anything
about what is actually on screen has to be answered against the running page.

Two things make that awkward, and both are handled here:

  1. Chrome 111+ rejects DevTools websocket connections whose Origin is not
     allow-listed. Without `--remote-allow-origins` the handshake fails with
     "403 Forbidden - Rejected an incoming WebSocket connection", which looks
     exactly like the debug port being blocked outright. It is not.

  2. Stash multi-user auth. `?apikey=` (session.ApiKeyParameter) authenticates
     only the request it is on and sets NO session cookie, so the SPA's own
     XHRs 401 and createClient.ts bounces to /login - you get the login page,
     not the app. A header-injecting HTTP proxy fails too, because the SPA
     opens a websocket a simple proxy cannot carry. CDP's
     Network.setExtraHTTPHeaders applies at the browser's network layer and so
     covers the document, every XHR AND the websocket handshake.

Usage
-----
  scripts/cdp_drive.py <url> [js-expression]
  scripts/cdp_drive.py http://localhost:5173/performers 'document.title'
  scripts/cdp_drive.py http://localhost:5173/performers \
      'JSON.stringify([...document.querySelectorAll(".performer-card")].length)'

  --gql SUBSTR   also dump GraphQL request variables / response counts for
                 operations whose body contains SUBSTR (e.g. findPerformers)
  --screenshot F write a PNG to F
  --wait N       seconds to settle after navigate (default 7)

Return a string from the expression (JSON.stringify big objects) - CDP's
returnByValue drops undefined keys, which silently hides a null result.

Requires: google-chrome, and python3-websocket (websocket-client).
"""
import argparse
import base64
import json
import os
import re
import subprocess
import sys
import time
import urllib.request

try:
    import websocket  # websocket-client
except ImportError:
    sys.exit("need websocket-client: apt install python3-websocket")

DEFAULT_CONFIG = "/opt/stash/docker/production/config/config.yml"


def api_key(path=DEFAULT_CONFIG):
    """The instance API key from config.yml. Empty if auth is disabled."""
    try:
        with open(path) as f:
            for line in f:
                m = re.match(r"\s*api_key:\s*(.+)", line)
                if m:
                    return m.group(1).strip().strip("\"'")
    except OSError:
        pass
    return ""


class Chrome:
    def __init__(self, port=9222, profile=None, window="1600,1200"):
        self.port = port
        self.profile = profile or f"/tmp/cdp-profile-{port}"
        subprocess.run(["rm", "-rf", self.profile], check=False)
        self.proc = subprocess.Popen(
            [
                "google-chrome",
                "--headless=new",
                # apparmor_restrict_unprivileged_userns=1 on this host, so the
                # sandbox cannot start; the browser is local and short-lived.
                "--no-sandbox",
                "--disable-gpu",
                "--disable-dev-shm-usage",
                f"--window-size={window}",
                f"--remote-debugging-port={port}",
                f"--user-data-dir={self.profile}",
                # see note 1 in the module docstring - without this the
                # websocket handshake 403s and CDP looks blocked
                "--remote-allow-origins=*",
                "about:blank",
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        for _ in range(80):
            try:
                with urllib.request.urlopen(
                    f"http://127.0.0.1:{port}/json/version", timeout=1
                ):
                    return
            except Exception:
                if self.proc.poll() is not None:
                    raise RuntimeError(
                        f"chrome exited ({self.proc.returncode}) before the "
                        f"debug port came up"
                    )
                time.sleep(0.25)
        self.kill()
        raise RuntimeError("CDP port never came up")

    def kill(self):
        self.proc.kill()
        # wait for exit before removing the profile - Chrome is still writing
        # its cache as it goes down, and rm -rf races it and prints errors
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            pass
        subprocess.run(
            ["rm", "-rf", self.profile], check=False,
            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )


class Page:
    def __init__(self, port=9222):
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/json/list") as r:
            targets = json.load(r)
        page = next(t for t in targets if t["type"] == "page")
        self.ws = websocket.create_connection(
            page["webSocketDebuggerUrl"], timeout=30, max_size=None
        )
        self.n = 0
        self.events = []

    def send(self, method, **params):
        self.n += 1
        self.ws.send(json.dumps({"id": self.n, "method": method, "params": params}))
        while True:
            msg = json.loads(self.ws.recv())
            if msg.get("id") == self.n:
                if "error" in msg:
                    raise RuntimeError(f"{method}: {msg['error']}")
                return msg.get("result", {})
            if "method" in msg:
                self.events.append(msg)

    def pump(self, seconds):
        """Collect protocol events for a while (needed to see network traffic)."""
        end = time.time() + seconds
        self.ws.settimeout(1)
        while time.time() < end:
            try:
                msg = json.loads(self.ws.recv())
            except Exception:
                continue
            if "method" in msg:
                self.events.append(msg)
        self.ws.settimeout(30)

    def evaluate(self, expr):
        r = self.send(
            "Runtime.evaluate", expression=expr, returnByValue=True, awaitPromise=True
        )
        return r.get("result", {}).get("value")

    def close(self):
        try:
            self.ws.close()
        except Exception:
            pass


def report_gql(page, needle):
    """Print request variables and response counts for matching operations."""
    wanted = {}
    for ev in page.events:
        if ev.get("method") == "Network.requestWillBeSent":
            post = ev["params"]["request"].get("postData") or ""
            if needle in post:
                try:
                    body = json.loads(post)
                except ValueError:
                    continue
                wanted[ev["params"]["requestId"]] = body
                print(f"--- {needle}: request variables ---")
                print(json.dumps(body.get("variables"), indent=2))
    for ev in page.events:
        if (
            ev.get("method") == "Network.loadingFinished"
            and ev["params"]["requestId"] in wanted
        ):
            rid = ev["params"]["requestId"]
            try:
                body = json.loads(
                    page.send("Network.getResponseBody", requestId=rid)["body"]
                )
            except Exception as e:
                print(f"--- {needle}: response unavailable ({e}) ---")
                continue
            # Print the shape, not the payload. A full findPerformers
            # response is tens of KB of performer records and buries the one
            # number you actually came for.
            data = body.get("data") or {}
            for op, val in data.items():
                if isinstance(val, dict):
                    summary = {
                        k: (v if not isinstance(v, list) else f"[{len(v)} items]")
                        for k, v in val.items()
                    }
                else:
                    summary = val
                print(f"--- {needle}: response {op} -> {json.dumps(summary)} ---")
            if body.get("errors"):
                print(f"--- {needle}: ERRORS {json.dumps(body['errors'])[:400]} ---")


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("url")
    ap.add_argument("expr", nargs="?", default="document.title")
    ap.add_argument("--gql", metavar="SUBSTR")
    ap.add_argument("--screenshot", metavar="FILE")
    ap.add_argument("--wait", type=float, default=7.0)
    ap.add_argument("--port", type=int, default=int(os.environ.get("CDP_PORT", 9222)))
    ap.add_argument("--config", default=DEFAULT_CONFIG)
    args = ap.parse_args()

    chrome = Chrome(port=args.port)
    page = None
    try:
        page = Page(port=args.port)
        for domain in ("Network", "Page", "Runtime"):
            page.send(f"{domain}.enable")

        key = api_key(args.config)
        if key:
            page.send("Network.setExtraHTTPHeaders", headers={"ApiKey": key})

        page.send("Page.navigate", url=args.url)
        page.pump(args.wait)

        print(page.evaluate(args.expr))

        if args.gql:
            report_gql(page, args.gql)

        if args.screenshot:
            shot = page.send("Page.captureScreenshot", format="png")
            with open(args.screenshot, "wb") as f:
                f.write(base64.b64decode(shot["data"]))
            print(f"screenshot -> {args.screenshot}", file=sys.stderr)
    finally:
        if page:
            page.close()
        chrome.kill()


if __name__ == "__main__":
    main()
