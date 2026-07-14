"""
CLI wrapper for mimir-mcp binary.
Downloads and runs the Go binary.
"""

import os
import sys
import stat
import platform
import subprocess
import requests
from pathlib import Path
from platformdirs import user_data_dir

__version__ = "1.0.0"

GITHUB_REPO = "zeus-kim/mimir"
BINARY_NAME = "mimir-mcp"

def get_platform_info():
    """Get OS and architecture for binary download."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    # Normalize OS
    if system == "darwin":
        os_name = "darwin"
    elif system == "linux":
        os_name = "linux"
    elif system == "windows":
        os_name = "windows"
    else:
        raise RuntimeError(f"Unsupported OS: {system}")

    # Normalize architecture
    if machine in ("x86_64", "amd64"):
        arch = "amd64"
    elif machine in ("arm64", "aarch64"):
        arch = "arm64"
    elif machine in ("i386", "i686"):
        arch = "386"
    else:
        raise RuntimeError(f"Unsupported architecture: {machine}")

    return os_name, arch

def get_binary_dir():
    """Get the directory where the binary is stored."""
    return Path(user_data_dir("mimir-mcp", "zeus-kim"))

def get_binary_path():
    """Get the full path to the binary."""
    binary_dir = get_binary_dir()
    system = platform.system().lower()

    if system == "windows":
        return binary_dir / f"{BINARY_NAME}.exe"
    return binary_dir / BINARY_NAME

def get_latest_release():
    """Get the latest release info from GitHub."""
    url = f"https://api.github.com/repos/{GITHUB_REPO}/releases/latest"
    try:
        response = requests.get(url, timeout=30)
        response.raise_for_status()
        return response.json()
    except requests.RequestException:
        return None

def download_binary(version=None):
    """Download the binary for the current platform."""
    os_name, arch = get_platform_info()
    binary_dir = get_binary_dir()
    binary_path = get_binary_path()

    # Create directory
    binary_dir.mkdir(parents=True, exist_ok=True)

    # Get download URL
    if version:
        tag = version
    else:
        release = get_latest_release()
        if release:
            tag = release["tag_name"]
        else:
            tag = "latest"

    # Construct download URL
    ext = ".exe" if os_name == "windows" else ""
    filename = f"{BINARY_NAME}-{os_name}-{arch}{ext}"

    if tag == "latest":
        url = f"https://github.com/{GITHUB_REPO}/releases/latest/download/{filename}"
    else:
        url = f"https://github.com/{GITHUB_REPO}/releases/download/{tag}/{filename}"

    print(f"Downloading mimir-mcp ({os_name}/{arch})...")

    try:
        response = requests.get(url, stream=True, timeout=120)
        response.raise_for_status()

        with open(binary_path, "wb") as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)

        # Make executable on Unix
        if os_name != "windows":
            binary_path.chmod(binary_path.stat().st_mode | stat.S_IEXEC)

        print(f"Installed to: {binary_path}")
        return binary_path

    except requests.RequestException as e:
        print(f"Error downloading binary: {e}", file=sys.stderr)
        print("\nAlternative installation methods:", file=sys.stderr)
        print("  go install github.com/zeus-kim/mimir/cmd/mimir-mcp@latest", file=sys.stderr)
        print("  git clone https://github.com/zeus-kim/mimir && cd mimir && make build", file=sys.stderr)
        sys.exit(1)

def ensure_binary():
    """Ensure the binary is downloaded and return its path."""
    binary_path = get_binary_path()

    if not binary_path.exists():
        download_binary()

    return binary_path

def main():
    """Main entry point - run the mimir-mcp binary."""
    # Handle special commands
    if len(sys.argv) > 1:
        cmd = sys.argv[1]

        if cmd == "--upgrade":
            download_binary()
            return

        if cmd == "--path":
            binary_path = get_binary_path()
            if binary_path.exists():
                print(binary_path)
            else:
                print("Binary not installed. Run: mimir-mcp --upgrade")
            return

        if cmd == "--py-version":
            print(f"mimir-mcp Python wrapper v{__version__}")
            return

    # Ensure binary exists
    binary_path = ensure_binary()

    # Run binary with all arguments
    try:
        result = subprocess.run(
            [str(binary_path)] + sys.argv[1:],
            check=False
        )
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)
    except Exception as e:
        print(f"Error running mimir-mcp: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
