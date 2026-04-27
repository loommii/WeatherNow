#!/bin/bash

DATA_DIR="$(cd "$(dirname "$0")" && pwd)/data"
V4_XDB="$DATA_DIR/ip2region_v4.xdb"
V6_XDB="$DATA_DIR/ip2region_v6.xdb"
V4_URL="https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v4.xdb"
V6_URL="https://raw.githubusercontent.com/lionsoul2014/ip2region/master/data/ip2region_v6.xdb"

mkdir -p "$DATA_DIR"

download_file() {
    local url=$1
    local target=$2
    local desc=$3

    if [ -f "$target" ]; then
        echo "$desc already exists at $target, skipping."
        echo "  To update, delete the file and re-run this script."
        return 0
    fi

    echo "Downloading $desc from $url ..."
    if curl -L --progress-bar -o "$target" "$url"; then
        local size
        size=$(stat -f%z "$target" 2>/dev/null || stat -c%s "$target" 2>/dev/null || echo "0")
        if [ "$size" -gt 0 ]; then
            echo "$desc downloaded successfully ($size bytes)."
        else
            echo "Error: $desc downloaded but file is empty."
            rm -f "$target"
            return 1
        fi
    else
        echo "Error: Failed to download $desc."
        rm -f "$target"
        return 1
    fi
}

echo "=== ip2region data setup ==="
echo "Data directory: $DATA_DIR"
echo ""

download_file "$V4_URL" "$V4_XDB" "IPv4 xdb"
download_file "$V6_URL" "$V6_XDB" "IPv6 xdb"

echo ""
echo "=== Setup complete ==="
echo "Configure your .env:"
echo "  IP2REGION_V4_DB_PATH=$V4_XDB"
echo "  IP2REGION_V6_DB_PATH=$V6_XDB"
