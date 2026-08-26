#!/bin/bash
# Download GeoLite2-Country.mmdb for GeoIP feature
# Requires a free MaxMind license key (register at https://www.maxmind.com/en/geolite2/signup)

set -e

LICENSE_KEY="${MAXMIND_LICENSE_KEY:-}"
DB_DIR="./rules"
DB_FILE="${DB_DIR}/GeoLite2-Country.mmdb"
DOWNLOAD_URL="https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=${LICENSE_KEY}&suffix=tar.gz"

if [ -z "$LICENSE_KEY" ]; then
    echo "Error: MAXMIND_LICENSE_KEY environment variable is not set."
    echo ""
    echo "To get a free license key:"
    echo "  1. Sign up at https://www.maxmind.com/en/geolite2/signup"
    echo "  2. Go to https://www.maxmind.com/en/accounts/current/license-key"
    echo "  3. Generate a license key"
    echo "  4. Export it: export MAXMIND_LICENSE_KEY=your_key_here"
    echo ""
    echo "Or pass it directly: MAXMIND_LICENSE_KEY=xxx ./scripts/download-geolite2.sh"
    exit 1
fi

mkdir -p "$DB_DIR"

# Check if already downloaded
if [ -f "$DB_FILE" ]; then
    echo "GeoLite2-Country.mmdb already exists at $DB_FILE"
    echo "Remove it to re-download."
    exit 0
fi

echo "Downloading GeoLite2-Country database..."
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

curl -sL "$DOWNLOAD_URL" -o "$TEMP_DIR/geolite2.tar.gz"

echo "Extracting..."
tar -xzf "$TEMP_DIR/geolite2.tar.gz" -C "$TEMP_DIR"

# Find the mmdb file
MMDB_FILE=$(find "$TEMP_DIR" -name "GeoLite2-Country.mmdb" -type f | head -1)

if [ -z "$MMDB_FILE" ]; then
    echo "Error: Could not find GeoLite2-Country.mmdb in downloaded archive"
    exit 1
fi

cp "$MMDB_FILE" "$DB_FILE"
echo "GeoLite2-Country.mmdb installed to $DB_FILE"
