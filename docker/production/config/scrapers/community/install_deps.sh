#!/bin/sh
# Install Python dependencies required by community scrapers.
# Run inside the stash container after rebuild:
#   docker exec stash sh /root/.stash/scrapers/community/install_deps.sh
apk add --no-cache python3 py3-pip py3-lxml py3-requests
pip install --break-system-packages stashapp-tools cloudscraper
