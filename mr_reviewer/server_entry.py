"""Standalone entry point for the PyInstaller-compiled backend server binary.

This module is the PyInstaller target. It delegates directly to the main()
function in __main__.py, which handles --serve / --port argument parsing
and starts the uvicorn server.
"""
import sys
from mr_reviewer.__main__ import main

if __name__ == '__main__':
    sys.exit(main())
