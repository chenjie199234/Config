@echo off
REM      Warning!!!!!!!!!!!This file is readonly!Don't modify this file!

where /q git.exe
if %errorlevel% == 1 (
	echo "missing dependence: git"
	goto :end
)

where /q go.exe
if %errorlevel% == 1 (
	echo "missing dependence: golang"
	goto :end
)

where /q protoc.exe
if %errorlevel% == 1 (
	echo "missing dependence: protoc"
	goto :end
)

where /q protoc-gen-go.exe
if %errorlevel% == 1 (
	echo "missing dependence: protoc-gen-go"
	goto :end
)

where /q protoc-gen-go-web.exe
if %errorlevel% == 1 (
	echo "missing dependence: protoc-gen-go-web"
	goto :end
)

where /q protoc-gen-go-rpc.exe
if %errorlevel% == 1 (
	echo "missing dependence: protoc-gen-go-rpc"
	goto :end
)

where /q codegen.exe
if %errorlevel% == 1 (
	echo "missing dependence: codegen"
	goto :end
)

if "%1" == "" (
	goto :help
)
if %1 == "" (
	goto :help
)
if %1 == "h" (
	goto :help
)
if "%1" == "h" (
	goto :help
)
if %1 == "-h" (
	goto :help
)
if "%1" == "-h" (
	goto :help
)
if %1 == "help" (
	goto :help
)
if "%1" == "help" (
	goto :help
)
if %1 == "-help" (
	goto :help
)
if "%1" == "-help" (
	goto :help
)
if %1 == "run" (
	goto :run
)
if "%1" == "run" (
	goto :run
)
if %1 == "build" (
	goto :build
)
if "%1" == "build" (
	goto :build
)
if %1 == "pb" (
	goto :pb
)
if "%1" == "pb" (
	goto :pb
)
if %1 == "kube" (
	goto :kube
)
if "%1" ==  "kube" (
	goto :kube
)
if %1 == "new" (
	if "%2" == "" (
		goto :help
	)
	if %2 == "" (
		goto :help
	)
	goto :new
)
if "%1" == "new" (
	if "%2" == "" (
		goto :help
	)
	if %2 == "" (
		goto :help
	)
	goto :new
)

:pb
	go mod tidy
	for /F %%i in ('go list -m -f "{{.Dir}}" github.com/chenjie199234/Corelib') do ( set corelib=%%i)
	protoc -I ./ -I %corelib% --go_out=paths=source_relative:. ./api/*.proto
	protoc -I ./ -I %corelib% --go-rpc_out=paths=source_relative:. ./api/*.proto
	protoc -I ./ -I %corelib% --go-web_out=paths=source_relative:. ./api/*.proto
	go mod tidy
goto :end

:run
	go mod tidy
	go run main.go
goto :end

:build
	go mod tidy
	go build -ldflags "-s -w" -o main.exe
	where /q upx.exe
	if %errorlevel% == 1 (
		echo "recommand to use upx.exe to compress exec file"
		goto :end
	)
	uxp.exe -9 main.exe
goto :end

:kube
	codegen -n config -g default -k
goto :end

:new
	codegen -n config -g default -s %2
goto :end

:help
	echo cmd.bat — every thing you need
	echo           please install golang
	echo           please install protoc
	echo           please install protoc-gen-go
	echo           please install protoc-gen-go-rpc
	echo           please install protoc-gen-go-web
	echo
	echo Usage:
	echo    ./cmd.bat <option^>
	echo
	echo Options:
	echo    run                       Run this program.
	echo    build                     Complie this program to binary.
	echo    pb                        Generate the proto in this program.
	echo    new <sub service name^>    Create a new sub service.
	echo    kube                      Update or add kubernetes config.
	echo    h/-h/help/-help/--help    Show this message.

:end
pause
exit /b 0