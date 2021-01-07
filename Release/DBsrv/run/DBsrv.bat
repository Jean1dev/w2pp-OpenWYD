@echo off
TITLE DBsrv
:Loop
echo.
echo %date% %time%
DBsrv.exe
goto Loop
