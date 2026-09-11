@echo off
rem Compila o GamePatch.dll (32 bits, runtime estatico: nao depende de DLL nenhum).
rem Precisa do Visual Studio 2022 Build Tools com C++.
setlocal
cd /d "%~dp0"
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars32.bat" >nul || exit /b 1
if not exist out mkdir out
cl /nologo /LD /MT /O2 /W4 /EHsc /Fo:out\ gamepatch.cpp timerfields.cpp /link /OUT:out\GamePatch.dll /NOLOGO || exit /b 1
echo.
echo GamePatch.dll gerado em %~dp0out
