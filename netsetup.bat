@echo off
chcp 936 >nul
setlocal enabledelayedexpansion
title Network Setup Tool

:: Check admin rights
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Requesting administrator privileges...
    powershell -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b
)

:MENU
call :GET_ADAPTERS_LIST
call :GET_CURRENT_IP
call :GET_DHCP_STATUS

cls
echo ============================================
echo         Network Setup Tool
echo ============================================
echo.
echo  Current Status: %dhcp_status%
echo  IP Address:     %current_ip%
echo  Adapter:        %current_adapter%
echo.
echo --------------------------------------------
echo.
echo  [1] DHCP - Auto Obtain IP
echo  [2] Static IP from config.ini
echo  [3] Show Current Network Config
echo  [4] Select Adapter
echo  [5] Exit
echo.
echo ============================================
echo.

set /p choice=Select option [1-5]: 

if "%choice%"=="1" goto DHCP
if "%choice%"=="2" goto STATIC
if "%choice%"=="3" goto SHOW
if "%choice%"=="4" goto SELECT_ADAPTER
if "%choice%"=="5" goto EXIT

echo Invalid selection, please try again...
timeout /t 2 >nul
goto MENU

:: ============================================
:: Get all connected adapters list
:: ============================================
:GET_ADAPTERS_LIST
set "adapter_count=0"
set "adapter_list="

for /f "skip=1 tokens=*" %%a in ('wmic nic where "NetConnectionStatus=2" get NetConnectionID /value 2^>nul') do (
    for /f "tokens=2 delims==" %%b in ("%%a") do (
        for /f "tokens=*" %%c in ("%%b") do (
            if not "%%c"=="" (
                set /a adapter_count+=1
                set "adapter_!adapter_count!=%%c"
                if !adapter_count! equ 1 (
                    set "current_adapter=%%c"
                )
            )
        )
    )
)

if %adapter_count% equ 0 (
    set "current_adapter=Unknown"
)
goto :eof

:: ============================================
:: Select adapter menu
:: ============================================
:SELECT_ADAPTER
cls
echo ============================================
echo         Select Network Adapter
echo ============================================
echo.

if %adapter_count% equ 0 (
    echo No active network adapters found!
    pause
    goto MENU
)

for /l %%i in (1,1,%adapter_count%) do (
    echo  [%%i] !adapter_%%i!
)

echo.
echo  [0] Back to Main Menu
echo.
echo ============================================
echo.

set /p adapter_choice=Select adapter [0-%adapter_count%]: 

if "%adapter_choice%"=="0" goto MENU

set "valid_choice=0"
for /l %%i in (1,1,%adapter_count%) do (
    if "%adapter_choice%"=="%%i" (
        set "valid_choice=1"
        set "current_adapter=!adapter_%%i!"
        echo.
        echo Selected adapter: !current_adapter!
        timeout /t 2 >nul
        goto MENU
    )
)

echo Invalid selection, please try again...
timeout /t 2 >nul
goto SELECT_ADAPTER

:: ============================================
:: Get current IP address
:: ============================================
:GET_CURRENT_IP
set "current_ip=Not connected"

if "%current_adapter%"=="Unknown" goto :eof

:: Get IP address for selected adapter from ipconfig
set "in_adapter=0"
for /f "tokens=*" %%a in ('ipconfig') do (
    set "line=%%a"
    
    :: Check if we found our adapter section
    echo !line! | findstr /i "%current_adapter%" >nul
    if !errorlevel! equ 0 (
        set "in_adapter=1"
    )
    
    :: If in our adapter section, look for IPv4
    if !in_adapter! equ 1 (
        echo !line! | findstr /i "IPv4" >nul
        if !errorlevel! equ 0 (
            for /f "tokens=2 delims=:" %%b in ("%%a") do (
                for /f "tokens=*" %%c in ("%%b") do (
                    set "temp_ip=%%c"
                    :: Check if it's not an APIPA address
                    echo !temp_ip! | findstr /b " 169.254." >nul
                    if errorlevel 1 (
                        set "current_ip=%%c"
                        set "in_adapter=0"
                        goto :eof
                    )
                )
            )
        )
        :: Check for blank line or next adapter
        if "!line!"=="" set "in_adapter=0"
    )
)
goto :eof

:: ============================================
:: Get DHCP status for the active adapter
:: ============================================
:GET_DHCP_STATUS
set "dhcp_status=Unknown"

if "%current_adapter%"=="Unknown" (
    set "dhcp_status=No Adapter"
    goto :eof
)

:: Use PowerShell to check DHCP status for the selected adapter
:: First get the adapter index by NetConnectionID, then get DHCP status
for /f "tokens=*" %%a in ('powershell -NoProfile -Command "$adapter = Get-CimInstance Win32_NetworkAdapter | Where-Object {$_.NetConnectionID -eq '%current_adapter%'}; if ($adapter) { $config = Get-CimInstance Win32_NetworkAdapterConfiguration | Where-Object {$_.Index -eq $adapter.Index}; $config.DHCPEnabled }" 2^>nul') do (
    if "%%a"=="True" (
        set "dhcp_status=DHCP (Auto)"
    ) else (
        set "dhcp_status=Static (Manual)"
    )
    goto :eof
)

:: Fallback: try to get from ipconfig
set "in_adapter=0"
for /f "tokens=*" %%a in ('ipconfig /all') do (
    set "line=%%a"
    echo !line! | findstr /i "%current_adapter%" >nul
    if !errorlevel! equ 0 (
        set "in_adapter=1"
    )
    
    if !in_adapter! equ 1 (
        echo !line! | findstr /i "DHCP Enabled" >nul
        if !errorlevel! equ 0 (
            echo !line! | findstr /i "Yes" >nul
            if !errorlevel! equ 0 (
                set "dhcp_status=DHCP (Auto)"
            ) else (
                set "dhcp_status=Static (Manual)"
            )
            goto :eof
        )
        if "!line!"=="" set "in_adapter=0"
    )
)
goto :eof

:DHCP
cls
echo ============================================
echo         DHCP - Auto Obtain IP
echo ============================================
echo.

if "%current_adapter%"=="Unknown" (
    echo No network adapter selected!
    pause
    goto MENU
)

echo Target adapter: %current_adapter%
echo.
echo Configuring DHCP...

:: Set DHCP
netsh interface ip set address name="%current_adapter%" source=dhcp >nul 2>&1
if %errorlevel% neq 0 (
    echo Failed to set IP address!
    pause
    goto MENU
)

:: Set DNS to auto
netsh interface ip set dns name="%current_adapter%" source=dhcp >nul 2>&1
netsh interface ip add dns name="%current_adapter%" addr=none index=2 >nul 2>&1

echo IP address set to DHCP
echo DNS set to DHCP
echo.
echo Refreshing network config...

:: Release and renew IP
ipconfig /release "*%current_adapter%*" >nul 2>&1
ipconfig /renew "*%current_adapter%*" >nul 2>&1
ipconfig /flushdns >nul 2>&1

echo Network config refreshed
echo.

:: Show current config
echo Current Network Config:
echo --------------------------------------------
ipconfig | findstr /i "IPv4 子网 默认"
echo.

:: Check network connectivity
call :CHECK_NETWORK

pause
goto MENU

:STATIC
cls
echo ============================================
echo         Static IP Configuration
echo ============================================
echo.

:: Check if config file exists
if not exist "%~dp0config.ini" (
    echo Error: config.ini not found!
    echo Please ensure config.ini is in the same directory.
    pause
    goto MENU
)

if "%current_adapter%"=="Unknown" (
    echo No network adapter selected!
    pause
    goto MENU
)

echo Target adapter: %current_adapter%
echo.
echo Reading config file...
echo.

:: Read config file
set "IP_LIST="
set "Mask_LIST="
set "Gateway_LIST="
set "DNS1="
set "DNS2="
set "Target_LIST="

for /f "usebackq tokens=1,2 delims==" %%a in ("%~dp0config.ini") do (
    if /i "%%a"=="IP" set "IP_LIST=%%b"
    if /i "%%a"=="Mask" set "Mask_LIST=%%b"
    if /i "%%a"=="Gateway" set "Gateway_LIST=%%b"
    if /i "%%a"=="DNS1" set "DNS1=%%b"
    if /i "%%a"=="DNS2" set "DNS2=%%b"
    if /i "%%a"=="Target" set "Target_LIST=%%b"
)

:: Check required parameters
if not defined IP_LIST (
    echo Error: IP address missing in config!
    pause
    goto MENU
)
if not defined Mask_LIST (
    echo Error: Subnet mask missing in config!
    pause
    goto MENU
)
if not defined Gateway_LIST (
    echo Error: Gateway missing in config!
    pause
    goto MENU
)

echo Config loaded:
echo   IP List:      %IP_LIST%
echo   Mask List:    %Mask_LIST%
echo   Gateway List: %Gateway_LIST%
echo   DNS1:         %DNS1%
echo   DNS2:         %DNS2%
echo   Targets:      %Target_LIST%
echo.

:: Parse first IP, mask and gateway
for /f "tokens=1 delims=," %%i in ("%IP_LIST%") do (
    set "PRIMARY_IP=%%i"
)
for /f "tokens=1 delims=," %%m in ("%Mask_LIST%") do (
    set "PRIMARY_MASK=%%m"
)
for /f "tokens=1 delims=," %%g in ("%Gateway_LIST%") do (
    set "PRIMARY_GATEWAY=%%g"
)

echo Applying static IP configuration...
echo.

:: Set primary IP with gateway
netsh interface ip set address name="%current_adapter%" source=static addr=%PRIMARY_IP% mask=%PRIMARY_MASK% gateway=%PRIMARY_GATEWAY% gwmetric=1 >nul 2>&1
if %errorlevel% neq 0 (
    echo Failed to set primary IP address!
    pause
    goto MENU
)
echo Primary IP set: %PRIMARY_IP% / %PRIMARY_MASK% gw %PRIMARY_GATEWAY%

:: Add additional IPs with corresponding masks and gateways
set "ip_index=0"
for %%i in (%IP_LIST%) do (
    set /a ip_index+=1
    if !ip_index! gtr 1 (
        :: Get the corresponding mask
        set "current_mask="
        set "temp_mask_index=0"
        for %%m in (%Mask_LIST%) do (
            set /a temp_mask_index+=1
            if !temp_mask_index! equ !ip_index! (
                set "current_mask=%%m"
            )
        )
        if not defined current_mask (
            for %%m in (%Mask_LIST%) do set "current_mask=%%m"
        )
        :: Get the corresponding gateway (if available)
        set "current_gateway="
        set "gw_count=0"
        set "temp_gw_index=0"
        for %%g in (%Gateway_LIST%) do (
            set /a gw_count+=1
            set /a temp_gw_index+=1
            if !temp_gw_index! equ !ip_index! (
                set "current_gateway=%%g"
            )
        )
        :: Add address with or without gateway
        if defined current_gateway (
            set /a gw_metric=ip_index
            netsh interface ip add address name="%current_adapter%" addr=%%i mask=!current_mask! gateway=!current_gateway! gwmetric=!gw_metric! >nul 2>&1
            if !errorlevel! equ 0 (
                echo Additional IP set: %%i / !current_mask! gw !current_gateway!
            ) else (
                echo Failed to add IP: %%i
            )
        ) else (
            netsh interface ip add address name="%current_adapter%" addr=%%i mask=!current_mask! >nul 2>&1
            if !errorlevel! equ 0 (
                echo Additional IP set: %%i / !current_mask!
            ) else (
                echo Failed to add IP: %%i
            )
        )
    )
)

:: Set DNS
if defined DNS1 (
    netsh interface ip set dns name="%current_adapter%" source=static addr=%DNS1% register=primary >nul 2>&1
    echo Primary DNS set: %DNS1%
)

if defined DNS2 (
    netsh interface ip add dns name="%current_adapter%" addr=%DNS2% index=2 >nul 2>&1
    echo Secondary DNS set: %DNS2%
)

echo.
echo Static IP configuration applied
echo.
echo Flushing DNS cache...
ipconfig /flushdns >nul 2>&1

echo.
echo Current Network Config:
echo --------------------------------------------
ipconfig | findstr /i "IPv4 子网 默认"
echo.

:: Check network connectivity with multiple targets
call :CHECK_NETWORK

pause
goto MENU

:CHECK_NETWORK
echo ============================================
echo         Network Connectivity Check
echo ============================================
echo.

if not defined Target_LIST (
    set "Target_LIST=223.5.5.5"
)

:: Check each target
for %%t in (%Target_LIST%) do (
    echo Pinging %%t...
    ping -n 2 -w 1000 %%t >nul 2>&1
    if %errorlevel% equ 0 (
        echo   [OK] %%t - reachable
        ping -n 1 -w 500 %%t | findstr /i "时间" 2>nul
    ) else (
        echo   [X] %%t - unreachable
    )
    echo.
)

goto :eof

:SHOW
cls
echo ============================================
echo         Current Network Configuration
echo ============================================
echo.
ipconfig /all
echo.
pause
goto MENU

:EXIT
exit /b
