#!/usr/bin/env python3
import os
import re
import sys
import time

import requests

# ==== CONFIG ====
STASH_URL = os.environ.get("STASH_URL", "http://localhost:9999/graphql")
API_KEY = os.environ.get("STASH_APIKEY", "")
PER_PAGE = 500
DRY_RUN = True  # set to False after verifying
# ===============

if not API_KEY:
    print("ERROR: Set STASH_APIKEY environment variable.")
    sys.exit(1)

HEADERS = {
    "Content-Type": "application/json",
    "ApiKey": API_KEY,
}

# reuse a single HTTP session to avoid port exhaustion on Windows
session = requests.Session()


def gql(query, variables=None):
    resp = session.post(
        STASH_URL,
        json={"query": query, "variables": variables or {}},
        headers=HEADERS,
        timeout=60,
    )
    resp.raise_for_status()
    data = resp.json()
    if "errors" in data:
        raise RuntimeError(data["errors"])
    return data["data"]


FIND_SCENES_QUERY = """
query FindScenes($filter: FindFilterType) {
  findScenes(filter: $filter) {
    count
    scenes {
      id
      title
      files {
        path
      }
    }
  }
}
"""

SCENE_UPDATE_MUTATION = """
mutation SceneUpdate($input: SceneUpdateInput!) {
  sceneUpdate(input: $input) {
    id
    title
  }
}
"""


def parse_filename(filename):
    """
    Expect filenames like:
      "Performer Name - Scene Title.ext"
    Return (performer, "Scene Title") or (None, None) if no match.
    """
    if " - " not in filename:
        return None, None

    performer, rest = filename.split(" - ", 1)
    if "." not in rest:
        return None, None

    title_no_ext = rest.rsplit(".", 1)[0]

    # Optional: turn "Women Of Color #3" into "Women Of Color 3"
    title_no_ext = re.sub(r"#(\d+)$", r"\1", title_no_ext).strip()

    # Clean up whitespace
    performer = performer.strip()
    title_no_ext = re.sub(r"\s+", " ", title_no_ext).strip()

    if not performer or not title_no_ext:
        return None, None

    return performer, title_no_ext


def should_update_title(current_title, filename, filename_no_ext, performer, new_title):
    if current_title in ("", filename, filename_no_ext):
        return True

    if current_title.startswith(performer + " - ") and current_title.endswith(new_title):
        return True

    return False


def main():
    page = 1
    total_candidates = 0
    total_updated = 0

    while True:
        data = gql(FIND_SCENES_QUERY, {"filter": {"per_page": PER_PAGE, "page": page}})
        scenes = data["findScenes"]["scenes"]
        if not scenes:
            break

        print(f"\n=== Page {page} ({len(scenes)} scenes) ===")

        for scene in scenes:
            sid = scene["id"]
            current_title = scene["title"] or ""
            files = scene.get("files") or []
            if not files:
                continue

            filename = os.path.basename(files[0]["path"])
            filename_no_ext = filename.rsplit(".", 1)[0] if "." in filename else filename

            performer, new_title = parse_filename(filename)
            if not new_title:
                continue

            if not should_update_title(
                current_title, filename, filename_no_ext, performer, new_title
            ):
                continue

            if current_title == new_title:
                continue

            total_candidates += 1
            print(f"[SCENE {sid}] '{current_title}' -> '{new_title}'")

            if not DRY_RUN:
                try:
                    gql(SCENE_UPDATE_MUTATION, {"input": {"id": sid, "title": new_title}})
                    total_updated += 1
                    time.sleep(0.05)
                except Exception as exc:
                    print(f"  !! Failed to update scene {sid}: {exc}")

        page += 1
        time.sleep(0.2)

    mode = "DRY RUN" if DRY_RUN else "APPLIED"
    print(f"\n{mode} complete.")
    print(f"  Candidate scenes: {total_candidates}")
    print(f"  Scenes updated:  {total_updated}")


if __name__ == "__main__":
    main()
