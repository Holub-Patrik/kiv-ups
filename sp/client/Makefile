default:
	go build -C src -o ../poker-client -v

wayland-glfw:
	go build -C src -o ../poker-client -a -v -tags wayland,noaudio,glfw

wayland-rgfw:
	go build -C src -o ../poker-client -a -v -tags wayland,noaudio,rgfw

x11-glfw:
	go build -C src -o ../poker-client -a -v -tags x11,noaudio,glfw

x11-rgfw:
	go build -C src -o ../poker-client -a -v -tags x11,noaudio,rgfw

windows-crosscompile:
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 go build -C src -a -v -ldflags "-s -w" -tags noaudio -o ../poker-client.exe
