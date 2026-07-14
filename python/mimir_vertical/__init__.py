"""
mimir-mcp: Vertical Search Engine MCP Server

Create domain-specific search engines through natural language.
"""

__version__ = "1.0.0"
__author__ = "zeus-kim"

from .cli import main, get_binary_path, ensure_binary

__all__ = ["main", "get_binary_path", "ensure_binary", "__version__"]
