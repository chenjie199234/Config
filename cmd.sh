#      Warning!!!!!!!!!!!This file is readonly!Don't modify this file!

help() {
	echo "cmd.sh — every thing you need"
	echo "         please install golang"
	echo "         please install protoc"
	echo "         please install protoc-gen-go"
	echo "         please install protoc-gen-go-rpc"
	echo "         please install protoc-gen-go-web"
	echo ""
	echo "Usage:"
	echo "   ./cmd.sh <option>"
	echo ""
	echo "Options:"
	echo "   run                       Run this program"
	echo "   build                     Complie this program to binary"
	echo "   pb                        Generate the proto in this program"
	echo "   new <sub service name>    Create a new sub service"
	echo "   kubernetes                Update or add kubernetes config"
	echo "   h/-h/help/-help/--help    Show this message"
}

pb() {
	protoc --go_out=paths=source_relative:. ./api/*.proto
	protoc --go-rpc_out=paths=source_relative:. ./api/*.proto
	protoc --go-web_out=paths=source_relative:. ./api/*.proto
	go mod tidy
}

run() {
	go mod tidy
	go run main.go
}

build() {
	go mod tidy
	go build -ldflags "-s -w" -o main
	if (type upx >/dev/null 2>&1);then
		upx -9  main
	else
		echo "recommand to use upx to compress exec file"
	fi
}

new() {
	codegen -n config -g default -s $1
}

kubernetes() {
	codegen -n config -g default -k
}

if !(type git >/dev/null 2>&1);then
	echo "missing dependence: git"
	exit 0
fi

if !(type go >/dev/null 2>&1);then
	echo "missing dependence: golang"
	exit 0
fi

if !(type protoc >/dev/null 2>&1);then
	echo "missing dependence: protoc"
	exit 0
fi

if !(type protoc-gen-go >/dev/null 2>&1);then
	echo "missing dependence: protoc-gen-go"
	exit 0
fi

if !(type protoc-gen-go-web >/dev/null 2>&1);then
	echo "missing dependence: protoc-gen-go-web"
	exit 0
fi

if !(type protoc-gen-go-rpc >/dev/null 2>&1);then
	echo "missing dependence: protoc-gen-go-rpc"
	exit 0
fi

if !(type codegen >/dev/null 2>&1);then
	echo "missing dependence: codegen"
	exit 0
fi

if [[ $# == 0 ]] || [[ "$1" == "h" ]] || [[ "$1" == "help" ]] || [[ "$1" == "-h" ]] || [[ "$1" == "-help" ]] || [[ "$1" == "--help" ]]; then
	help
	exit 0
fi

if [[ "$1" == "run" ]]; then
	run
	exit 0
fi

if [[ "$1" == "build" ]];then
	build
	exit 0
fi

if [[ "$1" == "pb" ]];then
	pb
	exit 0
fi

if [[ "$1" == "kubernetes" ]];then
	kubernetes
	exit 0
fi

if [[ $# == 2 ]] && [[ "$1" == "new" ]];then
	new $2
	exit 0
fi

echo "option unsupport"
help
