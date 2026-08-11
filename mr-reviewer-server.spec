from PyInstaller.utils.hooks import collect_all

datas, binaries, hiddenimports = [], [], []
for package in ("fastapi", "uvicorn", "starlette", "anthropic", "gitlab", "google.genai", "ollama"):
    package_datas, package_binaries, package_hiddenimports = collect_all(package)
    datas += package_datas
    binaries += package_binaries
    hiddenimports += package_hiddenimports

a = Analysis(["mr_reviewer/server_entry.py"], pathex=["."], binaries=binaries, datas=datas, hiddenimports=hiddenimports)
pyz = PYZ(a.pure)
exe = EXE(pyz, a.scripts, a.binaries, a.zipfiles, a.datas, [], name="mr-reviewer-server", console=True, onefile=True)
