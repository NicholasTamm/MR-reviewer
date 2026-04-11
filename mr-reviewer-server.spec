# -*- mode: python ; coding: utf-8 -*-
"""
PyInstaller spec for the MR Reviewer backend server.

Build with:
    pyinstaller mr-reviewer-server.spec --noconfirm

The output binary (mr-reviewer-server / mr-reviewer-server.exe) is a
self-contained executable that embeds the Python interpreter and all
dependencies. No Python installation is required on the target machine.
"""
from PyInstaller.utils.hooks import collect_all, collect_submodules

# Collect all data/binaries/hiddenimports for key packages
uvicorn_datas, uvicorn_bins, uvicorn_hidden = collect_all('uvicorn')
fastapi_datas, fastapi_bins, fastapi_hidden = collect_all('fastapi')
anyio_datas, anyio_bins, anyio_hidden = collect_all('anyio')
starlette_datas, starlette_bins, starlette_hidden = collect_all('starlette')

a = Analysis(
    ['mr_reviewer/server_entry.py'],
    pathex=['.'],
    binaries=uvicorn_bins + fastapi_bins + anyio_bins + starlette_bins,
    datas=uvicorn_datas + fastapi_datas + anyio_datas + starlette_datas,
    hiddenimports=(
        uvicorn_hidden + fastapi_hidden + anyio_hidden + starlette_hidden + [
            'pydantic',
            'pydantic_core',
            'pydantic.v1',
            'anthropic',
            'httpx',
            'gitlab',
            'mr_reviewer',
            'mr_reviewer.api',
            'mr_reviewer.api.app',
            'mr_reviewer.api.routes',
            'mr_reviewer.api.schemas',
            'mr_reviewer.api.state',
            'mr_reviewer.config',
            'mr_reviewer.core',
            'mr_reviewer.diff_parser',
            'mr_reviewer.exceptions',
            'mr_reviewer.models',
            'mr_reviewer.parallel',
            'mr_reviewer.platforms',
            'mr_reviewer.platforms.github_platform',
            'mr_reviewer.platforms.gitlab_platform',
            'mr_reviewer.providers',
            'mr_reviewer.providers.anthropic_provider',
            'mr_reviewer.providers.gemini_provider',
            'mr_reviewer.providers.ollama_provider',
            'mr_reviewer.prompts',
            'mr_reviewer.url_parser',
        ]
    ),
    hookspath=[],
    runtime_hooks=[],
    excludes=[
        'tkinter', 'matplotlib', 'numpy', 'pandas', 'scipy',
        'PIL', 'IPython', 'jupyter', 'notebook',
    ],
    noarchive=False,
    optimize=0,
)

pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name='mr-reviewer-server',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=True,  # Keep console output for logging
    onefile=True,  # Single self-contained executable
)
